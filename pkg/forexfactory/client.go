package forexfactory

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"runtime"
	"strings"
	"sync"
	"time"

	"github.com/chromedp/chromedp"
)

// Client is the main crawler and client for accessing Forex Factory calendar data.
type Client struct {
	httpClient *http.Client
	userAgent  string
	proxyURL   string
	rateLimit  int // Maximum requests per second
	timeLoc    *time.Location
	headers    map[string]string // Custom HTTP headers

	mu          sync.Mutex
	lastRequest time.Time

	bypassMu sync.Mutex // Mutex specifically to prevent multiple headless instances from solving Cloudflare concurrently
}

// NewClient creates and initializes a new Client with the provided options.
func NewClient(opts ...Option) *Client {
	c := &Client{
		httpClient: &http.Client{
			Timeout: 20 * time.Second,
		},
		userAgent: "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/125.0.0.0 Safari/537.36",
		rateLimit: 1, // Default rate limit: 1 request/second
		headers:   make(map[string]string),
	}

	for _, opt := range opts {
		opt(c)
	}

	// Build custom Transport if Proxy is configured
	if c.proxyURL != "" {
		if u, err := url.Parse(c.proxyURL); err == nil {
			transport := &http.Transport{
				Proxy: http.ProxyURL(u),
			}
			c.httpClient.Transport = transport
		}
	}

	return c
}

// findChromiumPath attempts to discover a valid Google Chrome, Microsoft Edge, or Chromium-based
// executable on Windows, macOS, or Linux, ensuring maximum headless compatibility across all major operating systems.
func findChromiumPath() string {
	var paths []string

	switch runtime.GOOS {
	case "windows":
		paths = []string{
			`C:\Program Files\Google\Chrome\Application\chrome.exe`,
			`C:\Program Files (x86)\Google\Chrome\Application\chrome.exe`,
			`C:\Program Files (x86)\Microsoft\Edge\Application\msedge.exe`,
			`C:\Program Files\Microsoft\Edge\Application\msedge.exe`,
		}
		if localAppData := os.Getenv("LOCALAPPDATA"); localAppData != "" {
			paths = append(paths,
				localAppData+`\Google\Chrome\Application\chrome.exe`,
				localAppData+`\Microsoft\Edge\Application\msedge.exe`,
			)
		}
	case "darwin": // macOS
		paths = []string{
			"/Applications/Google Chrome.app/Contents/MacOS/Google Chrome",
			"/Applications/Microsoft Edge.app/Contents/MacOS/Microsoft Edge",
			"/Applications/Chromium.app/Contents/MacOS/Chromium",
		}
	case "linux":
		paths = []string{
			"/usr/bin/google-chrome",
			"/usr/bin/chrome",
			"/usr/bin/chromium",
			"/usr/bin/chromium-browser",
			"/usr/bin/microsoft-edge",
			"/snap/bin/chromium",
			"/snap/bin/google-chrome",
		}
	}

	for _, path := range paths {
		if _, err := os.Stat(path); err == nil {
			return path
		}
	}
	return "" // Return empty to fallback to chromedp default path searches
}

// waitRateLimit blocks until the rate-limiter allows a request to proceed.
func (c *Client) waitRateLimit() {
	if c.rateLimit <= 0 {
		return
	}

	c.mu.Lock()
	defer c.mu.Unlock()

	minInterval := time.Second / time.Duration(c.rateLimit)
	elapsed := time.Since(c.lastRequest)
	if elapsed < minInterval {
		time.Sleep(minInterval - elapsed)
	}
	c.lastRequest = time.Now()
}

// applyHeaders applies premium browser and custom headers to the request.
func (c *Client) applyHeaders(req *http.Request) {
	c.mu.Lock()
	defer c.mu.Unlock()

	req.Header.Set("User-Agent", c.userAgent)
	req.Header.Set("Accept", "text/html,application/xhtml+xml,application/xml;q=0.9,image/avif,image/webp,image/apng,*/*;q=0.8")
	req.Header.Set("Accept-Language", "en-US,en;q=0.9")
	req.Header.Set("Cache-Control", "no-cache")
	req.Header.Set("Pragma", "no-cache")

	for key, val := range c.headers {
		req.Header.Set(key, val)
	}
}

// SolveCloudflareChallenge programmatically drives a headless Chrome browser via chromedp
// to solve the Cloudflare validation check. It extracts the document.cookie string
// and binds it to the Client's headers for all subsequent calls.
func (c *Client) SolveCloudflareChallenge() error {
	opts := append(chromedp.DefaultExecAllocatorOptions[:],
		chromedp.Flag("headless", true),
		chromedp.Flag("disable-gpu", true),
		chromedp.Flag("no-sandbox", true),
	)

	// Inject auto-discovered Chromium path if Chrome/Edge was found on standard system locations
	if extPath := findChromiumPath(); extPath != "" {
		opts = append(opts, chromedp.ExecPath(extPath))
	}

	allocCtx, allocCancel := chromedp.NewExecAllocator(context.Background(), opts...)
	defer allocCancel()

	ctx, cancel := chromedp.NewContext(allocCtx)
	defer cancel()

	// Assign context timeout to protect against getting stuck
	ctx, cancel = context.WithTimeout(ctx, 40*time.Second)
	defer cancel()

	var cookiesStr string

	err := chromedp.Run(ctx,
		chromedp.Navigate("https://www.forexfactory.com/calendar"),
		// Wait for the calendar table to become visible (which indicates challenge has been resolved)
		chromedp.WaitVisible("table.calendar__table", chromedp.ByQuery),
		chromedp.ActionFunc(func(ctx context.Context) error {
			var err error
			err = chromedp.Evaluate("document.cookie", &cookiesStr).Do(ctx)
			return err
		}),
	)

	if err != nil {
		return fmt.Errorf("failed to navigate and resolve Cloudflare challenge: %w", err)
	}

	if cookiesStr != "" {
		c.mu.Lock()
		if c.headers == nil {
			c.headers = make(map[string]string)
		}
		c.headers["Cookie"] = cookiesStr
		c.mu.Unlock()
	}

	return nil
}

// FetchWeekViaBrowser drives a headless Chromium instance to navigate to the weekly calendar,
// waits for it to render, and extracts the raw outer HTML content to bypass advanced TLS fingerprinting.
func (c *Client) FetchWeekViaBrowser(ctx context.Context, targetURL string) ([]byte, error) {
	opts := append(chromedp.DefaultExecAllocatorOptions[:],
		chromedp.Flag("headless", true),
		chromedp.Flag("disable-gpu", true),
		chromedp.Flag("no-sandbox", true),
	)

	if extPath := findChromiumPath(); extPath != "" {
		opts = append(opts, chromedp.ExecPath(extPath))
	}

	allocCtx, allocCancel := chromedp.NewExecAllocator(context.Background(), opts...)
	defer allocCancel()

	ctx, cancel := chromedp.NewContext(allocCtx)
	defer cancel()

	ctx, cancel = context.WithTimeout(ctx, 45*time.Second)
	defer cancel()

	var htmlContent string

	err := chromedp.Run(ctx,
		chromedp.Navigate(targetURL),
		chromedp.WaitVisible("table.calendar__table", chromedp.ByQuery),
		chromedp.OuterHTML("html", &htmlContent),
	)

	if err != nil {
		return nil, fmt.Errorf("failed to scrape calendar page via browser: %w", err)
	}

	return []byte(htmlContent), nil
}

// executeRequest performs an HTTP request, applying the configured headers and rate limit.
// If it receives a 403 Forbidden or 429 Too Many Requests, it automatically triggers
// the headless Cloudflare bypass, updates session cookies, and retries the request seamlessly.
func (c *Client) executeRequest(ctx context.Context, targetURL string) ([]byte, error) {
	c.waitRateLimit()

	req, err := http.NewRequestWithContext(ctx, "GET", targetURL, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	c.applyHeaders(req)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("HTTP request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusForbidden || resp.StatusCode == http.StatusTooManyRequests {
		// Handle Cloudflare bypass as a single-instance concurrent lock
		c.bypassMu.Lock()

		// Re-apply headers in case another worker already solved the challenge and updated c.headers
		c.applyHeaders(req)

		// Check if we already got the cookies updated while waiting for bypassMu
		currentCookie := req.Header.Get("Cookie")
		
		c.mu.Lock()
		clientCookie := c.headers["Cookie"]
		c.mu.Unlock()

		if currentCookie == clientCookie {
			// Cookies have not changed yet, meaning we are the first worker to handle the bypass
			if errBypass := c.SolveCloudflareChallenge(); errBypass != nil {
				c.bypassMu.Unlock()
				return nil, fmt.Errorf("automated Cloudflare bypass failed: %w", errBypass)
			}
		}
		c.bypassMu.Unlock()

		// Re-apply cookies and retry the request
		c.applyHeaders(req)

		c.waitRateLimit()
		respRetry, errRetry := c.httpClient.Do(req)
		if errRetry != nil {
			return nil, fmt.Errorf("HTTP request retry failed: %w", errRetry)
		}
		defer respRetry.Body.Close()

		if respRetry.StatusCode != http.StatusOK {
			return nil, fmt.Errorf("unexpected status code on retry %d: %s", respRetry.StatusCode, respRetry.Status)
		}

		body, errRead := io.ReadAll(respRetry.Body)
		if errRead != nil {
			return nil, fmt.Errorf("failed to read response body on retry: %w", errRead)
		}

		return body, nil
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected status code %d: %s", resp.StatusCode, resp.Status)
	}

	body, errRead := io.ReadAll(resp.Body)
	if errRead != nil {
		return nil, fmt.Errorf("failed to read response body: %w", errRead)
	}

	return body, nil
}

// FetchWeek fetches and parses the calendar events for the week containing the specified date.
// If the HTTP scraper fails (due to Cloudflare session constraints), it automatically falls back
// to executing the request natively in Microsoft Edge/Chrome, delivering bulletproof bypass stability.
func (c *Client) FetchWeek(ctx context.Context, date time.Time) ([]Event, error) {
	daysToSunday := int(date.Weekday())
	sunday := date.AddDate(0, 0, -daysToSunday)

	mon := strings.ToLower(sunday.Format("Jan"))
	day := sunday.Format("2")
	year := sunday.Format("2006")
	weekParam := fmt.Sprintf("%s%s.%s", mon, day, year)

	targetURL := fmt.Sprintf("https://www.forexfactory.com/calendar?week=%s", weekParam)

	// Attempt standard fast HTTP request first
	body, err := c.executeRequest(ctx, targetURL)
	if err != nil {
		// Fallback to loading via headless Chromium-based browser to completely bypass Cloudflare restrictions
		fmt.Fprintf(os.Stderr, "\n[Cloudflare Bypass] Fast HTTP blocked for week %s. Running headless browser session...\n", weekParam)
		body, err = c.FetchWeekViaBrowser(ctx, targetURL)
		if err != nil {
			return nil, fmt.Errorf("both HTTP crawler and headless browser extraction failed: %w", err)
		}
	}

	parsedYear := sunday.Year()
	events, err := ParseHTML(strings.NewReader(string(body)), parsedYear, c.timeLoc)
	if err != nil {
		return nil, fmt.Errorf("failed to parse HTML for week %s: %w", weekParam, err)
	}

	return events, nil
}

// FetchLiveFeed fetches the current week's events from the fast and lightweight XML feed.
func (c *Client) FetchLiveFeed(ctx context.Context) ([]Event, error) {
	targetURL := "https://nfs.faireconomy.media/ff_calendar_thisweek.xml"

	body, err := c.executeRequest(ctx, targetURL)
	if err != nil {
		return nil, err
	}

	events, err := ParseXML(body, c.timeLoc)
	if err != nil {
		return nil, fmt.Errorf("failed to parse XML live feed: %w", err)
	}

	return events, nil
}
