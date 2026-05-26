package forexfactory

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"
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
}

// NewClient creates and initializes a new Client with the provided options.
func NewClient(opts ...Option) *Client {
	c := &Client{
		httpClient: &http.Client{
			Timeout: 15 * time.Second,
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

// executeRequest performs an HTTP request, applying the configured headers and rate limit.
func (c *Client) executeRequest(ctx context.Context, targetURL string) ([]byte, error) {
	c.waitRateLimit()

	req, err := http.NewRequestWithContext(ctx, "GET", targetURL, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	// Apply premium browser headers to prevent standard bot detection blocks
	req.Header.Set("User-Agent", c.userAgent)
	req.Header.Set("Accept", "text/html,application/xhtml+xml,application/xml;q=0.9,image/avif,image/webp,image/apng,*/*;q=0.8")
	req.Header.Set("Accept-Language", "en-US,en;q=0.9")
	req.Header.Set("Cache-Control", "no-cache")
	req.Header.Set("Pragma", "no-cache")

	// Apply user-defined custom headers (e.g. Cookies, custom Referer, etc.)
	for key, val := range c.headers {
		req.Header.Set(key, val)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("HTTP request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected status code %d: %s", resp.StatusCode, resp.Status)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %w", err)
	}

	return body, nil
}

// FetchWeek fetches and parses the calendar events for the week containing the specified date.
// Forex Factory weekly calendar runs Sunday to Saturday. The URL format relies on Sunday's date.
func (c *Client) FetchWeek(ctx context.Context, date time.Time) ([]Event, error) {
	// Find the Sunday of the target week
	daysToSunday := int(date.Weekday())
	sunday := date.AddDate(0, 0, -daysToSunday)

	// Format Sunday date for the Forex Factory query parameter (e.g. may24.2026)
	mon := strings.ToLower(sunday.Format("Jan"))
	day := sunday.Format("2")
	year := sunday.Format("2006")
	weekParam := fmt.Sprintf("%s%s.%s", mon, day, year)

	targetURL := fmt.Sprintf("https://www.forexfactory.com/calendar?week=%s", weekParam)

	body, err := c.executeRequest(ctx, targetURL)
	if err != nil {
		return nil, err
	}

	// Parse HTML and return events
	parsedYear := sunday.Year()
	events, err := ParseHTML(strings.NewReader(string(body)), parsedYear, c.timeLoc)
	if err != nil {
		return nil, fmt.Errorf("failed to parse HTML for week %s: %w", weekParam, err)
	}

	return events, nil
}

// FetchLiveFeed fetches the current week's events from the fast and lightweight XML feed.
// This is perfect for real-time streaming and monitoring events without loading the full site.
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
