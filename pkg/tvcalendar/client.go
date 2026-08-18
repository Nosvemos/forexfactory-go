package tvcalendar

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"sort"
	"sync"
	"time"
)

const (
	defaultEndpoint  = "https://economic-calendar.tradingview.com/events"
	defaultUserAgent = "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/130.0.0.0 Safari/537.36"
)

// Client is the lightning-fast, pure HTTP client for global economic calendar data.
type Client struct {
	httpClient       *http.Client
	userAgent        string
	proxyURL         string
	maxRetries       int
	rateLimit        int // Requests per second
	concurrency      int
	timeLoc          *time.Location
	impactFilter     map[Impact]bool
	currencyFilter   map[string]bool
	countryFilter    map[string]bool
	progressCallback func(current, total int)

	mu          sync.Mutex
	lastRequest time.Time
}

// NewClient creates and initializes a new Client with the provided options.
func NewClient(opts ...Option) *Client {
	c := &Client{
		httpClient:  &http.Client{Timeout: 15 * time.Second},
		userAgent:   defaultUserAgent,
		maxRetries:  3,
		rateLimit:   10, // Default 10 req/s (TradingView allows fast bursts)
		concurrency: 5,  // Default 5 concurrent workers
	}

	for _, opt := range opts {
		opt(c)
	}

	if c.proxyURL != "" {
		if parsedURL, err := url.Parse(c.proxyURL); err == nil {
			c.httpClient.Transport = &http.Transport{
				Proxy: http.ProxyURL(parsedURL),
			}
		}
	}

	return c
}

// Close gracefully releases any client resources.
func (c *Client) Close() error {
	if c.httpClient != nil {
		c.httpClient.CloseIdleConnections()
	}
	return nil
}

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

// executeRequest performs an HTTP GET request with retries and rate limiting.
func (c *Client) executeRequest(ctx context.Context, targetURL string) ([]byte, error) {
	maxAttempts := c.maxRetries
	if maxAttempts <= 0 {
		maxAttempts = 3
	}

	var lastErr error
	for attempt := 0; attempt < maxAttempts; attempt++ {
		if attempt > 0 {
			backoff := time.Duration(1<<attempt) * 150 * time.Millisecond
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

		req.Header.Set("User-Agent", c.userAgent)
		req.Header.Set("Origin", "https://www.tradingview.com")
		req.Header.Set("Referer", "https://www.tradingview.com/")
		req.Header.Set("Accept", "application/json, text/plain, */*")
		req.Header.Set("Accept-Language", "en-US,en;q=0.9")

		resp, err := c.httpClient.Do(req)
		if err != nil {
			lastErr = fmt.Errorf("request failed (attempt %d/%d): %w", attempt+1, maxAttempts, err)
			continue
		}

		if resp.StatusCode != http.StatusOK {
			_ = resp.Body.Close()
			lastErr = fmt.Errorf("server responded with status %d: %s", resp.StatusCode, resp.Status)
			continue
		}

		body, errRead := io.ReadAll(resp.Body)
		_ = resp.Body.Close()
		if errRead != nil {
			lastErr = fmt.Errorf("failed to read response body: %w", errRead)
			continue
		}

		return body, nil
	}

	return nil, fmt.Errorf("all %d attempts failed: %w", maxAttempts, lastErr)
}

// FetchTimeRange retrieves events within the specified time window in a single HTTP request.
func (c *Client) FetchTimeRange(ctx context.Context, from, to time.Time) ([]Event, error) {
	fromStr := from.UTC().Format("2006-01-02T15:04:05.000Z")
	toStr := to.UTC().Format("2006-01-02T15:04:05.000Z")

	targetURL := fmt.Sprintf("%s?from=%s&to=%s", defaultEndpoint, url.QueryEscape(fromStr), url.QueryEscape(toStr))

	data, err := c.executeRequest(ctx, targetURL)
	if err != nil {
		return nil, err
	}

	events, err := ParseJSON(data, c.timeLoc)
	if err != nil {
		return nil, err
	}

	return c.filterEvents(events), nil
}

// FetchRange downloads calendar events concurrently across any historical or future date range (e.g. 10+ years).
// It slices the range into monthly chunks, downloads them in parallel, and merges them chronologically.
func (c *Client) FetchRange(ctx context.Context, start, end time.Time) ([]Event, error) {
	if start.After(end) {
		return nil, fmt.Errorf("start date cannot be after end date")
	}

	type chunk struct {
		from time.Time
		to   time.Time
	}

	var chunks []chunk
	curr := time.Date(start.Year(), start.Month(), 1, 0, 0, 0, 0, time.UTC)
	for curr.Before(end) || curr.Equal(end) {
		chunkFrom := curr
		if chunkFrom.Before(start) {
			chunkFrom = start
		}

		nextMonth := curr.AddDate(0, 1, 0)
		chunkTo := nextMonth.Add(-time.Millisecond)
		if chunkTo.After(end) {
			chunkTo = end
		}

		chunks = append(chunks, chunk{from: chunkFrom, to: chunkTo})
		curr = nextMonth
	}

	totalChunks := len(chunks)
	if totalChunks == 0 {
		return nil, nil
	}

	type jobResult struct {
		events []Event
		err    error
	}

	jobsChan := make(chan chunk, totalChunks)
	resultsChan := make(chan jobResult, totalChunks)

	workers := c.concurrency
	if workers > totalChunks {
		workers = totalChunks
	}
	if workers < 1 {
		workers = 1
	}

	var wg sync.WaitGroup
	for w := 0; w < workers; w++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for ch := range jobsChan {
				evs, err := c.FetchTimeRange(ctx, ch.from, ch.to)
				resultsChan <- jobResult{events: evs, err: err}
			}
		}()
	}

	for _, ch := range chunks {
		jobsChan <- ch
	}
	close(jobsChan)

	wg.Wait()
	close(resultsChan)

	var allEvents []Event
	seenIDs := make(map[string]bool)
	completed := 0

	for res := range resultsChan {
		completed++
		if c.progressCallback != nil {
			c.progressCallback(completed, totalChunks)
		}
		if res.err != nil {
			return nil, fmt.Errorf("failed fetching chunk (%d/%d): %w", completed, totalChunks, res.err)
		}
		for _, e := range res.events {
			if e.ID != "" && seenIDs[e.ID] {
				continue
			}
			if e.ID != "" {
				seenIDs[e.ID] = true
			}
			allEvents = append(allEvents, e)
		}
	}

	sort.Slice(allEvents, func(i, j int) bool {
		if allEvents[i].Date.Equal(allEvents[j].Date) {
			return allEvents[i].Title < allEvents[j].Title
		}
		return allEvents[i].Date.Before(allEvents[j].Date)
	})

	return allEvents, nil
}

// FetchMonth retrieves all events for a given calendar month.
func (c *Client) FetchMonth(ctx context.Context, year int, month time.Month) ([]Event, error) {
	start := time.Date(year, month, 1, 0, 0, 0, 0, time.UTC)
	end := start.AddDate(0, 1, 0).Add(-time.Millisecond)
	return c.FetchTimeRange(ctx, start, end)
}

// FetchWeek retrieves all events for the week containing the specified date (Sunday to Saturday).
func (c *Client) FetchWeek(ctx context.Context, date time.Time) ([]Event, error) {
	daysToSunday := int(date.Weekday())
	sunday := time.Date(date.Year(), date.Month(), date.Day()-daysToSunday, 0, 0, 0, 0, time.UTC)
	saturday := sunday.AddDate(0, 0, 7).Add(-time.Millisecond)
	return c.FetchTimeRange(ctx, sunday, saturday)
}

// FetchDay retrieves all events for a single 24-hour day.
func (c *Client) FetchDay(ctx context.Context, date time.Time) ([]Event, error) {
	start := time.Date(date.Year(), date.Month(), date.Day(), 0, 0, 0, 0, time.UTC)
	end := start.AddDate(0, 0, 1).Add(-time.Millisecond)
	return c.FetchTimeRange(ctx, start, end)
}

// FetchLiveFeed retrieves the current week's upcoming and recently released events.
func (c *Client) FetchLiveFeed(ctx context.Context) ([]Event, error) {
	now := time.Now().UTC()
	return c.FetchWeek(ctx, now)
}

// StreamLive polls for live calendar events at the specified interval.
func (c *Client) StreamLive(ctx context.Context, interval time.Duration) (<-chan []Event, <-chan error) {
	eventsChan := make(chan []Event)
	errChan := make(chan error, 1)

	if interval <= 0 {
		interval = 30 * time.Second
	}

	go func() {
		defer close(eventsChan)
		defer close(errChan)

		ticker := time.NewTicker(interval)
		defer ticker.Stop()

		// Initial fetch
		if events, err := c.FetchLiveFeed(ctx); err != nil {
			errChan <- err
		} else {
			eventsChan <- events
		}

		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				events, err := c.FetchLiveFeed(ctx)
				if err != nil {
					errChan <- err
				} else {
					eventsChan <- events
				}
			}
		}
	}()

	return eventsChan, errChan
}

func (c *Client) filterEvents(events []Event) []Event {
	if len(c.impactFilter) == 0 && len(c.currencyFilter) == 0 && len(c.countryFilter) == 0 {
		return events
	}

	filtered := make([]Event, 0, len(events))
	for _, e := range events {
		if len(c.impactFilter) > 0 && !c.impactFilter[e.Impact] {
			continue
		}
		if len(c.currencyFilter) > 0 && !c.currencyFilter[e.Currency] {
			continue
		}
		if len(c.countryFilter) > 0 && !c.countryFilter[e.Country] {
			continue
		}
		filtered = append(filtered, e)
	}

	return filtered
}
