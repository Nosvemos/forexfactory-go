package forexfactory

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"runtime"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/chromedp/chromedp"
)

// Client is the main crawler and client for accessing Forex Factory calendar data.
type Client struct {
	httpClient        *http.Client
	userAgent         string
	proxyURL          string
	rateLimit         int // Maximum requests per second
	timeLoc           *time.Location
	headers           map[string]string // Custom HTTP headers
	concurrency       int
	progressCallback  func(current, total int)
	impactFilter      map[Impact]bool   // Filter only specific impact levels

	mu          sync.Mutex
	lastRequest time.Time

	bypassMu  sync.Mutex // Mutex specifically to prevent multiple headless instances from solving Cloudflare concurrently
	browserMu sync.Mutex // Mutex to serialize all browser fallbacks (prevent concurrency storm)
}

// NewClient creates and initializes a new Client with the provided options.
func NewClient(opts ...Option) *Client {
	c := &Client{
		httpClient: &http.Client{
			Timeout: 20 * time.Second,
		},
		userAgent:   "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/125.0.0.0 Safari/537.36",
		rateLimit:   1, // Default rate limit: 1 request/second
		concurrency: 3, // Default concurrency: 3 workers
		headers:     make(map[string]string),
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

	// Force UTC timezone and 12-hour format on Forex Factory visual calendar
	if cookieVal := req.Header.Get("Cookie"); cookieVal == "" {
		req.Header.Set("Cookie", "fftimezoneoffset=0; fftimeformat=1;")
	} else if !strings.Contains(cookieVal, "fftimezoneoffset") {
		req.Header.Set("Cookie", cookieVal+"; fftimezoneoffset=0; fftimeformat=1;")
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
		chromedp.Flag("disable-blink-features", "AutomationControlled"),
		chromedp.Flag("excludeSwitches", "enable-automation"),
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
		// Force UTC timezone and 12-hour format on Forex Factory visual calendar
		if !strings.Contains(cookiesStr, "fftimezoneoffset") {
			cookiesStr = cookiesStr + "; fftimezoneoffset=0; fftimeformat=1;"
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
		chromedp.Flag("disable-blink-features", "AutomationControlled"),
		chromedp.Flag("excludeSwitches", "enable-automation"),
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
		// Serialize browser fallbacks to prevent concurrent Chromium storms
		c.browserMu.Lock()

		// Double check: retry standard HTTP in case another worker resolved the challenge
		body, err = c.executeRequest(ctx, targetURL)
		if err == nil {
			c.browserMu.Unlock()
		} else {
			// Fallback to loading via headless Chromium-based browser to completely bypass Cloudflare restrictions
			fmt.Fprintf(os.Stderr, "\n[Cloudflare Bypass] Fast HTTP blocked for week %s. Running headless browser session...\n", weekParam)
			body, err = c.FetchWeekViaBrowser(ctx, targetURL)
			c.browserMu.Unlock()
			if err != nil {
				return nil, fmt.Errorf("both HTTP crawler and headless browser extraction failed: %w", err)
			}
		}
	}

	events, err := ParseHTMLWithSunday(strings.NewReader(string(body)), sunday, c.timeLoc)
	if err != nil {
		return nil, fmt.Errorf("failed to parse HTML for week %s: %w", weekParam, err)
	}

	// Filter events by impact level if filter is configured
	if len(c.impactFilter) > 0 {
		filtered := make([]Event, 0, len(events))
		for _, e := range events {
			if c.impactFilter[e.Impact] {
				filtered = append(filtered, e)
			}
		}
		events = filtered
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

	// Filter events by impact level if filter is configured
	if len(c.impactFilter) > 0 {
		filtered := make([]Event, 0, len(events))
		for _, e := range events {
			if c.impactFilter[e.Impact] {
				filtered = append(filtered, e)
			}
		}
		events = filtered
	}

	return events, nil
}

// FetchRange downloads calendar events concurrently across the specified date range.
// It splits the range into weeks (starting from the Sunday of each week), distributes the downloads
// across concurrent workers (using the configured rate limiter), invokes the progress callback if set,
// and returns a chronologically sorted slice of Events matching the range.
func (c *Client) FetchRange(ctx context.Context, start, end time.Time) ([]Event, error) {
	if start.After(end) {
		return nil, fmt.Errorf("start date cannot be after end date")
	}

	// Calculate all Sundays spanning the range week-by-week
	var sundays []time.Time
	currentDate := start.AddDate(0, 0, -int(start.Weekday())) // Sunday of start week
	for currentDate.Before(end) || currentDate.Equal(end) {
		sundays = append(sundays, currentDate)
		currentDate = currentDate.AddDate(0, 0, 7)
	}

	numJobs := len(sundays)
	if numJobs == 0 {
		return nil, nil
	}

	type job struct {
		sunday time.Time
	}

	type result struct {
		events []Event
		err    error
		sunday time.Time
	}

	jobsChan := make(chan job, numJobs)
	resultsChan := make(chan result, numJobs)

	workers := c.concurrency
	if workers > numJobs {
		workers = numJobs
	}
	if workers < 1 {
		workers = 1
	}

	// Start concurrent workers
	var wg sync.WaitGroup
	for w := 1; w <= workers; w++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := range jobsChan {
				select {
				case <-ctx.Done():
					resultsChan <- result{err: ctx.Err(), sunday: j.sunday}
					return
				default:
				}

				events, err := c.FetchWeek(ctx, j.sunday)
				resultsChan <- result{events: events, err: err, sunday: j.sunday}
			}
		}()
	}

	// Dispatch jobs
	for _, s := range sundays {
		jobsChan <- job{sunday: s}
	}
	close(jobsChan)

	// Wait for workers to finish in a separate goroutine and close results channel
	go func() {
		wg.Wait()
		close(resultsChan)
	}()

	var allEvents []Event
	var firstErr error

	// Collect results
	completed := 0
	for res := range resultsChan {
		completed++
		if c.progressCallback != nil {
			c.progressCallback(completed, numJobs)
		}

		if res.err != nil {
			if firstErr == nil {
				firstErr = fmt.Errorf("error downloading week of %s: %w", res.sunday.Format("2006-01-02"), res.err)
			}
			continue
		}

		allEvents = append(allEvents, res.events...)
	}

	if firstErr != nil {
		return nil, firstErr
	}

	// Filter out events falling strictly outside of start/end range
	var filteredEvents []Event
	// Normalize filter boundaries to date-only UTC for stable comparisons
	filterStart := time.Date(start.Year(), start.Month(), start.Day(), 0, 0, 0, 0, time.UTC)
	filterEnd := time.Date(end.Year(), end.Month(), end.Day(), 0, 0, 0, 0, time.UTC)

	for _, e := range allEvents {
		eventDate := time.Date(e.Date.Year(), e.Date.Month(), e.Date.Day(), 0, 0, 0, 0, time.UTC)
		if (eventDate.After(filterStart) || eventDate.Equal(filterStart)) && (eventDate.Before(filterEnd) || eventDate.Equal(filterEnd)) {
			filteredEvents = append(filteredEvents, e)
		}
	}

	// Sort events chronologically to restore order from parallel downloads
	sort.Slice(filteredEvents, func(i, j int) bool {
		return filteredEvents[i].Date.Before(filteredEvents[j].Date)
	})

	return filteredEvents, nil
}

