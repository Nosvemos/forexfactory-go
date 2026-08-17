package forexfactory

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/chromedp/cdproto/network"
	"github.com/chromedp/cdproto/page"
	"github.com/chromedp/chromedp"
	"github.com/imroc/req/v3"
)

var defaultUserAgentPool = []string{
	"Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/125.0.0.0 Safari/537.36",
	"Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/125.0.0.0 Safari/537.36",
	"Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/124.0.0.0 Safari/537.36",
	"Mozilla/5.0 (X11; Linux x86_64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/125.0.0.0 Safari/537.36",
	"Mozilla/5.0 (Windows NT 10.0; Win64; x64; rv:126.0) Gecko/20100101 Firefox/126.0",
}

// Client is the main crawler and client for accessing Forex Factory calendar data.
type Client struct {
	httpClient       *http.Client
	userAgent        string // UA for HTTP requests (must match TLS fingerprint)
	browserUserAgent string // UA for headless browser (must match real OS)
	proxyURL         string
	proxyPool        []string // Rotating proxy list
	proxyIndex       int
	userAgentPool    []string // Rotating UA pool
	uaIndex          int
	maxRetries       int // Max attempts on network or 5xx server errors
	rateLimit        int // Maximum requests per second
	timeLoc          *time.Location
	headers          map[string]string // Custom HTTP headers
	concurrency      int
	progressCallback func(current, total int)
	impactFilter     map[Impact]bool // Filter only specific impact levels
	headless         bool            // Use headless mode for chromedp

	mu          sync.Mutex
	lastRequest time.Time

	bypassMu  sync.Mutex // Mutex specifically to prevent multiple headless instances from solving Cloudflare concurrently
	browserMu sync.Mutex // Mutex to serialize all browser fallbacks (prevent concurrency storm)

	// Long-lived browser context fields
	allocCtx     context.Context
	allocCancel  context.CancelFunc
	chromeCtx    context.Context
	chromeCancel context.CancelFunc
}

// NewClient creates and initializes a new Client with the provided options.
func NewClient(opts ...Option) *Client {
	reqClient := req.C().SetTimeout(20 * time.Second)
	reqClient.ImpersonateChrome()

	c := &Client{
		httpClient:       reqClient.GetClient(),
		userAgent:        "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/125.0.0.0 Safari/537.36",
		browserUserAgent: "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/125.0.0.0 Safari/537.36",
		userAgentPool:    defaultUserAgentPool,
		maxRetries:       3,
		rateLimit:        1, // Default rate limit: 1 request/second
		concurrency:      3, // Default concurrency: 3 workers
		headers:          make(map[string]string),
		headless:         true, // Default to true
	}

	// Load cached cookies on client initialization so option functions can override them if needed
	c.loadSession()

	for _, opt := range opts {
		opt(c)
	}

	// Build custom Transport if Proxy is configured
	if c.proxyURL != "" {
		reqClient.SetProxyURL(c.proxyURL)
	}

	return c
}

type sessionData struct {
	Cookie      string    `json:"cookie"`
	UserAgent   string    `json:"user_agent"`
	LastUpdated time.Time `json:"last_updated"`
}

func getSessionFilePath() string {
	cacheDir, err := os.UserCacheDir()
	if err != nil {
		cacheDir = os.TempDir()
	}
	dir := filepath.Join(cacheDir, "forexfactory-go")
	_ = os.MkdirAll(dir, 0700)
	return filepath.Join(dir, "session.json")
}

func (c *Client) loadSession() {
	path := getSessionFilePath()
	data, err := os.ReadFile(path)
	if err != nil {
		return
	}
	var sess sessionData
	if err := json.Unmarshal(data, &sess); err != nil {
		return
	}

	// Session is valid for 24 hours
	if time.Since(sess.LastUpdated) < 24*time.Hour && sess.Cookie != "" {
		c.mu.Lock()
		if c.headers == nil {
			c.headers = make(map[string]string)
		}
		c.headers["Cookie"] = sess.Cookie
		if sess.UserAgent != "" {
			c.userAgent = sess.UserAgent
		}
		c.mu.Unlock()
	}
}

func (c *Client) saveSession(cookieStr string, userAgent string) error {
	path := getSessionFilePath()
	sess := sessionData{
		Cookie:      cookieStr,
		UserAgent:   userAgent,
		LastUpdated: time.Now(),
	}
	data, err := json.Marshal(sess)
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0600)
}

func (c *Client) initBrowser() error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.chromeCtx != nil {
		return nil // Already initialized
	}

	opts := append(chromedp.DefaultExecAllocatorOptions[:],
		chromedp.Flag("headless", c.headless),
		chromedp.Flag("disable-gpu", c.headless),
		chromedp.Flag("no-sandbox", true),
		chromedp.Flag("disable-blink-features", "AutomationControlled"),
		chromedp.Flag("excludeSwitches", "enable-automation"),
		chromedp.Flag("blink-settings", "imagesEnabled=false"),
	)

	if extPath := findChromiumPath(); extPath != "" {
		opts = append(opts, chromedp.ExecPath(extPath))
	}

	c.allocCtx, c.allocCancel = chromedp.NewExecAllocator(context.Background(), opts...)
	c.chromeCtx, c.chromeCancel = chromedp.NewContext(c.allocCtx)

	return nil
}

// Close safely cleans up any long-lived browser sessions.
func (c *Client) Close() error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.chromeCancel != nil {
		c.chromeCancel()
		c.chromeCtx = nil
		c.chromeCancel = nil
	}
	if c.allocCancel != nil {
		c.allocCancel()
		c.allocCtx = nil
		c.allocCancel = nil
	}
	return nil
}

// resetBrowser kills the current browser process and resets contexts so that
// the next call to initBrowser() will start a completely fresh browser instance.
// This must be called whenever a browser operation fails (timeout, crash, etc.)
// to prevent the corrupted chromeCtx from poisoning all subsequent operations.
func (c *Client) resetBrowser() {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.chromeCancel != nil {
		c.chromeCancel()
	}
	if c.allocCancel != nil {
		c.allocCancel()
	}
	c.chromeCtx = nil
	c.chromeCancel = nil
	c.allocCtx = nil
	c.allocCancel = nil
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
// Calculates sleep duration under lock, then sleeps outside the lock to prevent global client lockup.
func (c *Client) waitRateLimit() {
	if c.rateLimit <= 0 {
		return
	}

	minInterval := time.Second / time.Duration(c.rateLimit)

	c.mu.Lock()
	now := time.Now()
	var waitDuration time.Duration
	if c.lastRequest.IsZero() {
		c.lastRequest = now
	} else {
		nextAllowed := c.lastRequest.Add(minInterval)
		if now.Before(nextAllowed) {
			waitDuration = nextAllowed.Sub(now)
			c.lastRequest = nextAllowed
		} else {
			c.lastRequest = now
		}
	}
	c.mu.Unlock()

	if waitDuration > 0 {
		time.Sleep(waitDuration)
	}
}

// getEffectiveUserAgent returns the current User-Agent or rotates from the pool.
func (c *Client) getEffectiveUserAgent() string {
	c.mu.Lock()
	defer c.mu.Unlock()
	if len(c.userAgentPool) > 0 {
		ua := c.userAgentPool[c.uaIndex%len(c.userAgentPool)]
		c.uaIndex++
		return ua
	}
	return c.userAgent
}

// stealthScriptAction injects scripts into Chromium before document creation to bypass Turnstile checks.
func stealthScriptAction() chromedp.Action {
	return chromedp.ActionFunc(func(ctx context.Context) error {
		script := `
			Object.defineProperty(navigator, 'webdriver', {get: () => undefined});
			window.chrome = { runtime: {}, app: {}, csi: () => {}, loadTimes: () => {} };
			Object.defineProperty(navigator, 'plugins', {get: () => [1, 2, 3, 4, 5]});
			Object.defineProperty(navigator, 'languages', {get: () => ['en-US', 'en']});
		`
		_, err := page.AddScriptToEvaluateOnNewDocument(script).Do(ctx)
		return err
	})
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
	cookieVal := req.Header.Get("Cookie")
	if cookieVal == "" {
		cookieVal = "fftimezoneoffset=0; fftimeformat=1;"
		req.Header.Set("Cookie", cookieVal)
	} else if !strings.Contains(cookieVal, "fftimezoneoffset") {
		cookieVal = strings.TrimSuffix(cookieVal, ";") + "; fftimezoneoffset=0; fftimeformat=1;"
		req.Header.Set("Cookie", cookieVal)
	}
}

// SolveCloudflareChallenge programmatically drives a Chrome/Edge browser via chromedp
// to solve the Cloudflare validation check. It extracts all session cookies (including HttpOnly ones)
// and binds them to the Client's headers for all subsequent calls, saving them in a local cache.
// If headless mode fails or times out, it automatically falls back to headed mode to ensure a bulletproof bypass.
func (c *Client) SolveCloudflareChallenge() error {
	c.browserMu.Lock()
	defer c.browserMu.Unlock()
	return c.solveCloudflareChallengeLocked()
}

func (c *Client) solveCloudflareChallengeLocked() error {
	if err := c.initBrowser(); err != nil {
		return err
	}

	// Create a short-lived timeout context from our long-lived chromeCtx
	ctx, cancel := context.WithTimeout(c.chromeCtx, 40*time.Second)
	defer cancel()

	var cookies []*network.Cookie
	var actualUA string

	err := chromedp.Run(ctx,
		stealthScriptAction(),
		chromedp.Navigate("https://www.forexfactory.com/calendar"),
		// Wait for the calendar table to become visible (which indicates challenge has been resolved)
		chromedp.WaitVisible("table.calendar__table", chromedp.ByQuery),
		chromedp.Evaluate("navigator.userAgent", &actualUA),
		chromedp.ActionFunc(func(ctx context.Context) error {
			var err error
			err = network.Enable().Do(ctx)
			if err != nil {
				return err
			}
			cookies, err = network.GetCookies().Do(ctx)
			return err
		}),
	)

	if err != nil {
		if c.headless {
			fmt.Fprintln(os.Stderr, "\n[Session Renewal] Headless challenge blocked or timed out. Retrying in headed mode (a browser window will briefly appear)...")
			c.headless = false
			c.resetBrowser()
			errRetry := c.solveCloudflareChallengeLocked()
			c.headless = true // Restore headless preference for future calls
			return errRetry
		}
		c.resetBrowser() // Reset browser so next attempt starts fresh
		return fmt.Errorf("failed to navigate and resolve Cloudflare challenge: %w", err)
	}

	var cookieStrings []string
	for _, cookie := range cookies {
		cookieStrings = append(cookieStrings, fmt.Sprintf("%s=%s", cookie.Name, cookie.Value))
	}
	cookiesStr := strings.Join(cookieStrings, "; ")

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
		if actualUA != "" {
			c.userAgent = actualUA
		}
		c.mu.Unlock()

		// Save the newly resolved cookie session to local persistent cache
		_ = c.saveSession(cookiesStr, actualUA)
	}

	return nil
}

// FetchWeekViaBrowser drives a Chromium instance to navigate to the weekly calendar,
// waits for it to render, and extracts the raw outer HTML content to bypass advanced TLS fingerprinting.
// If headless mode fails or times out, it automatically falls back to headed mode.
func (c *Client) FetchWeekViaBrowser(ctx context.Context, targetURL string) ([]byte, error) {
	var weekParam string
	if idx := strings.Index(targetURL, "week="); idx != -1 {
		weekParam = targetURL[idx+5:]
	} else {
		weekParam = targetURL
	}
	fmt.Fprintf(os.Stderr, "\n[Session Renewal] Resolving Cloudflare challenge for week %s via browser...\n", weekParam)

	// Note: FetchWeekViaBrowser is called by FetchWeek while holding c.browserMu,
	// so we assume c.browserMu is ALREADY locked when this is called.
	return c.fetchWeekViaBrowserLocked(ctx, targetURL)
}

func (c *Client) fetchWeekViaBrowserLocked(ctx context.Context, targetURL string) ([]byte, error) {
	if err := c.initBrowser(); err != nil {
		return nil, err
	}

	// Create a short-lived context derived from our long-lived chromeCtx
	browserCtx, cancel := context.WithTimeout(c.chromeCtx, 45*time.Second)
	defer cancel()

	var htmlContent string
	var cookies []*network.Cookie
	var actualUA string

	err := chromedp.Run(browserCtx,
		stealthScriptAction(),
		chromedp.Navigate(targetURL),
		chromedp.WaitVisible("table.calendar__table", chromedp.ByQuery),
		chromedp.OuterHTML("html", &htmlContent),
		chromedp.Evaluate("navigator.userAgent", &actualUA),
		chromedp.ActionFunc(func(ctx context.Context) error {
			var err error
			err = network.Enable().Do(ctx)
			if err != nil {
				fmt.Fprintf(os.Stderr, "[Debug] network.Enable failed: %v\n", err)
				return err
			}
			cookies, err = network.GetCookies().Do(ctx)
			if err != nil {
				fmt.Fprintf(os.Stderr, "[Debug] network.GetCookies failed: %v\n", err)
			}
			return err
		}),
	)

	if err != nil {
		if c.headless {
			fmt.Fprintln(os.Stderr, "\n[Session Renewal] Headless browser blocked or timed out. Retrying in headed mode (a browser window will briefly appear)...")
			c.headless = false
			c.resetBrowser()
			body, errRetry := c.fetchWeekViaBrowserLocked(ctx, targetURL)
			c.headless = true // Restore headless preference for future calls
			return body, errRetry
		}
		c.resetBrowser() // Reset browser so next attempt starts fresh
		fmt.Fprintf(os.Stderr, "[Debug] chromedp.Run failed: %v\n", err)
		return nil, fmt.Errorf("failed to scrape calendar page via browser: %w", err)
	}

	var cookieStrings []string
	for _, cookie := range cookies {
		cookieStrings = append(cookieStrings, fmt.Sprintf("%s=%s", cookie.Name, cookie.Value))
	}
	cookiesStr := strings.Join(cookieStrings, "; ")

	fmt.Fprintf(os.Stderr, "[Debug] Extracted %d cookies from browser. Combined string length: %d\n", len(cookies), len(cookiesStr))

	if cookiesStr != "" {
		c.mu.Lock()
		if c.headers == nil {
			c.headers = make(map[string]string)
		}
		if !strings.Contains(cookiesStr, "fftimezoneoffset") {
			cookiesStr = cookiesStr + "; fftimezoneoffset=0; fftimeformat=1;"
		}
		c.headers["Cookie"] = cookiesStr
		if actualUA != "" {
			c.userAgent = actualUA
		}
		c.mu.Unlock()

		// Save the newly resolved cookie session to local persistent cache
		saveErr := c.saveSession(cookiesStr, actualUA)
		if saveErr != nil {
			fmt.Fprintf(os.Stderr, "[Debug] saveSession failed: %v\n", saveErr)
		} else {
			fmt.Fprintf(os.Stderr, "[Debug] Successfully saved session cache to disk!\n")
		}
	}

	return []byte(htmlContent), nil
}

// executeRequest performs an HTTP request with exponential backoff retries.
// If it receives a 403 Forbidden or 429 Too Many Requests, it automatically triggers
// the headless Cloudflare bypass, updates session cookies, and retries the request seamlessly.
func (c *Client) executeRequest(ctx context.Context, targetURL string) ([]byte, error) {
	maxAttempts := c.maxRetries
	if maxAttempts <= 0 {
		maxAttempts = 3
	}

	var lastErr error
	for attempt := 0; attempt < maxAttempts; attempt++ {
		if attempt > 0 {
			backoff := time.Duration(1<<attempt) * 200 * time.Millisecond
			select {
			case <-ctx.Done():
				return nil, ctx.Err()
			case <-time.After(backoff):
			}
		}

		c.waitRateLimit()

		req, err := http.NewRequestWithContext(ctx, "GET", targetURL, nil)
		if err != nil {
			return nil, fmt.Errorf("failed to create request: %w", err)
		}

		c.applyHeaders(req)

		resp, err := c.httpClient.Do(req)
		if err != nil {
			lastErr = fmt.Errorf("HTTP request failed (attempt %d/%d): %w", attempt+1, maxAttempts, err)
			continue
		}

		if resp.StatusCode == http.StatusForbidden || resp.StatusCode == http.StatusTooManyRequests {
			failedCookie := req.Header.Get("Cookie")
			_ = resp.Body.Close()

			// Handle Cloudflare bypass as a single-instance concurrent lock
			c.bypassMu.Lock()

			c.mu.Lock()
			clientCookie := c.headers["Cookie"]
			c.mu.Unlock()

			// Check if cookies have NOT changed since our request failed
			if clientCookie == "" || !strings.Contains(failedCookie, clientCookie) {
				if errBypass := c.SolveCloudflareChallenge(); errBypass != nil {
					c.bypassMu.Unlock()
					lastErr = fmt.Errorf("automated Cloudflare bypass failed: %w", errBypass)
					continue
				}
			}
			c.bypassMu.Unlock()

			// Re-apply updated cookies and retry the request
			c.applyHeaders(req)
			c.waitRateLimit()

			respRetry, errRetry := c.httpClient.Do(req)
			if errRetry != nil {
				lastErr = fmt.Errorf("HTTP request retry failed: %w", errRetry)
				continue
			}

			if respRetry.StatusCode == http.StatusOK {
				body, errRead := io.ReadAll(respRetry.Body)
				_ = respRetry.Body.Close()
				if errRead != nil {
					return nil, fmt.Errorf("failed to read response body on retry: %w", errRead)
				}
				return body, nil
			}

			_ = respRetry.Body.Close()
			lastErr = fmt.Errorf("unexpected status code on retry %d: %s", respRetry.StatusCode, respRetry.Status)
			continue
		}

		if resp.StatusCode >= 500 {
			_ = resp.Body.Close()
			lastErr = fmt.Errorf("server error %d: %s", resp.StatusCode, resp.Status)
			continue
		}

		if resp.StatusCode != http.StatusOK {
			_ = resp.Body.Close()
			return nil, fmt.Errorf("unexpected status code %d: %s", resp.StatusCode, resp.Status)
		}

		body, errRead := io.ReadAll(resp.Body)
		_ = resp.Body.Close()
		if errRead != nil {
			return nil, fmt.Errorf("failed to read response body: %w", errRead)
		}

		return body, nil
	}

	return nil, fmt.Errorf("all %d request attempts failed: %w", maxAttempts, lastErr)
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

		// Double check: retry standard HTTP in case another worker resolved the challenge.
		// We use a direct httpClient.Do call here to avoid calling executeRequest under browserMu lock,
		// preventing any AB-BA deadlock between browserMu and bypassMu.
		var retrySucceeded bool
		reqRetry, errReq := http.NewRequestWithContext(ctx, "GET", targetURL, nil)
		if errReq == nil {
			c.applyHeaders(reqRetry)
			c.waitRateLimit()
			respRetry, errResp := c.httpClient.Do(reqRetry)
			if errResp == nil {
				defer respRetry.Body.Close()
				if respRetry.StatusCode == http.StatusOK {
					if bodyBytes, errRead := io.ReadAll(respRetry.Body); errRead == nil {
						body = bodyBytes
						retrySucceeded = true
					}
				}
			}
		}

		if retrySucceeded {
			c.browserMu.Unlock()
		} else {
			// Fallback to loading via headed/headless Chromium-based browser to completely bypass Cloudflare restrictions
			body, err = c.FetchWeekViaBrowser(ctx, targetURL)
			c.browserMu.Unlock()
			if err != nil {
				return nil, fmt.Errorf("both HTTP crawler and browser extraction failed: %w", err)
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

// FetchEventDetail fetches deep-dive metadata, economic metrics, and historical releases for a specific event ID.
func (c *Client) FetchEventDetail(ctx context.Context, eventID string) (*EventDetail, error) {
	if eventID == "" {
		return nil, fmt.Errorf("eventID cannot be empty")
	}

	targetURL := fmt.Sprintf("https://www.forexfactory.com/calendar?show=%s", eventID)
	body, err := c.executeRequest(ctx, targetURL)
	if err != nil {
		// Fallback to browser execution if blocked by Cloudflare
		c.browserMu.Lock()
		body, err = c.fetchWeekViaBrowserLocked(ctx, targetURL)
		c.browserMu.Unlock()
		if err != nil {
			return nil, fmt.Errorf("failed to fetch event detail for ID %s: %w", eventID, err)
		}
	}

	detail, err := ParseEventDetail(strings.NewReader(string(body)), eventID)
	if err != nil {
		return nil, err
	}

	return detail, nil
}
