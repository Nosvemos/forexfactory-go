package forexfactory

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"
)

func TestClientOptions(t *testing.T) {
	// Test WithUserAgent, WithRateLimit, WithConcurrency and WithProgressCallback
	var calledProgress bool
	client := NewClient(
		WithUserAgent("TestAgent/1.0"),
		WithRateLimit(5),
		WithProxy("http://127.0.0.1:8080"),
		WithHeader("X-Custom-Cookie", "mycookievalue"),
		WithConcurrency(10),
		WithProgressCallback(func(c, tot int) { calledProgress = true }),
	)

	if client.userAgent != "TestAgent/1.0" {
		t.Errorf("Expected userAgent 'TestAgent/1.0', got '%s'", client.userAgent)
	}

	if client.rateLimit != 5 {
		t.Errorf("Expected rateLimit 5, got %d", client.rateLimit)
	}

	if client.proxyURL != "http://127.0.0.1:8080" {
		t.Errorf("Expected proxyURL 'http://127.0.0.1:8080', got '%s'", client.proxyURL)
	}

	if client.headers["X-Custom-Cookie"] != "mycookievalue" {
		t.Errorf("Expected header X-Custom-Cookie to be 'mycookievalue', got '%s'", client.headers["X-Custom-Cookie"])
	}

	if client.concurrency != 10 {
		t.Errorf("Expected concurrency 10, got %d", client.concurrency)
	}

	if client.progressCallback == nil {
		t.Errorf("Expected progressCallback to be configured")
	} else {
		client.progressCallback(1, 2)
		if !calledProgress {
			t.Errorf("Expected progress callback to trigger")
		}
	}
}

func TestClientRateLimiter(t *testing.T) {
	// Setup a fast client, rate limit of 10 requests per second (100ms interval)
	client := NewClient(WithRateLimit(10))

	start := time.Now()
	// Trigger 3 fast requests
	client.waitRateLimit()
	client.waitRateLimit()
	client.waitRateLimit()
	elapsed := time.Since(start)

	// 3 requests with rate limit of 10 QPS should take at least ~200ms
	// First request: immediate (elapsed = 0)
	// Second request: waits 100ms
	// Third request: waits 100ms
	// Total expected wait: >= 200ms
	if elapsed < 180*time.Millisecond {
		t.Errorf("Rate limiter failed: elapsed time %v is less than expected ~200ms", elapsed)
	}
}

func TestExecuteRequestHeaders(t *testing.T) {
	// Setup mock test server to verify client sends our custom headers
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("User-Agent") != "CustomUA/2.0" {
			t.Errorf("Expected User-Agent 'CustomUA/2.0', got '%s'", r.Header.Get("User-Agent"))
		}
		if !strings.Contains(r.Header.Get("Cookie"), "test-cookie-val") {
			t.Errorf("Expected Cookie to contain 'test-cookie-val', got '%s'", r.Header.Get("Cookie"))
		}
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("mock response"))
	}))
	defer server.Close()

	client := NewClient(
		WithUserAgent("CustomUA/2.0"),
		WithHeader("Cookie", "test-cookie-val"),
	)

	body, err := client.executeRequest(context.Background(), server.URL)
	if err != nil {
		t.Fatalf("executeRequest failed: %v", err)
	}

	if string(body) != "mock response" {
		t.Errorf("Expected body 'mock response', got '%s'", string(body))
	}
}

func TestClientFetchRangeValidation(t *testing.T) {
	client := NewClient()

	// 1. Verify that a start date after end date returns an error
	start := time.Date(2026, time.May, 26, 0, 0, 0, 0, time.UTC)
	end := time.Date(2026, time.May, 25, 0, 0, 0, 0, time.UTC)
	_, err := client.FetchRange(context.Background(), start, end)
	if err == nil {
		t.Errorf("Expected error when start is after end date, got nil")
	}

	// 2. Verify that progress callback is called if configured (we can test with an empty range or similar if allowed)
}

func TestClientImpactsOptionAndFiltering(t *testing.T) {
	client := NewClient(
		WithImpacts(ImpactHigh, ImpactMedium),
	)

	if !client.impactFilter[ImpactHigh] {
		t.Errorf("Expected impact filter to have ImpactHigh")
	}
	if !client.impactFilter[ImpactMedium] {
		t.Errorf("Expected impact filter to have ImpactMedium")
	}
	if client.impactFilter[ImpactLow] {
		t.Errorf("Expected impact filter not to have ImpactLow")
	}

	// Verify events are filtered correctly
	testEvents := []Event{
		{Title: "High Event", Impact: ImpactHigh},
		{Title: "Low Event", Impact: ImpactLow},
	}

	events := []Event{}
	for _, e := range testEvents {
		if client.impactFilter[e.Impact] {
			events = append(events, e)
		}
	}

	if len(events) != 1 || events[0].Title != "High Event" {
		t.Errorf("Expected only High Event to be kept, got %d items", len(events))
	}
}

func TestClientConcurrentRateLimiter(t *testing.T) {
	// Verifies that waitRateLimit doesn't hold locks during sleep and allows concurrent calls
	client := NewClient(WithRateLimit(20)) // 50ms interval

	start := time.Now()
	var wg sync.WaitGroup
	for i := 0; i < 5; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			client.waitRateLimit()
		}()
	}
	wg.Wait()
	elapsed := time.Since(start)

	// 5 requests with 20 QPS (50ms interval) should span at least ~200ms
	if elapsed < 150*time.Millisecond {
		t.Errorf("Concurrent rate limiter finished too fast: %v", elapsed)
	}
}

func TestApplyHeadersSingleCookie(t *testing.T) {
	client := NewClient(
		WithHeader("Cookie", "session=abc123xyz"),
	)

	req, _ := http.NewRequest("GET", "https://example.com", nil)
	client.applyHeaders(req)

	// Should only have 1 Cookie header entry
	cookieHeaders := req.Header.Values("Cookie")
	if len(cookieHeaders) != 1 {
		t.Errorf("Expected exactly 1 Cookie header entry, got %d: %v", len(cookieHeaders), cookieHeaders)
	}

	cookieVal := req.Header.Get("Cookie")
	if !strings.Contains(cookieVal, "session=abc123xyz") || !strings.Contains(cookieVal, "fftimezoneoffset=0") {
		t.Errorf("Expected cookie to contain both session and fftimezoneoffset, got %q", cookieVal)
	}
}

func TestClientProxyPoolAndUserAgentPool(t *testing.T) {
	proxies := []string{"http://127.0.0.1:8001", "http://127.0.0.1:8002"}
	userAgents := []string{"UA-1", "UA-2", "UA-3"}

	client := NewClient(
		WithProxyPool(proxies),
		WithUserAgentPool(userAgents),
		WithMaxRetries(2),
	)

	if len(client.proxyPool) != 2 {
		t.Errorf("Expected 2 proxies in pool, got %d", len(client.proxyPool))
	}
	if len(client.userAgentPool) != 3 {
		t.Errorf("Expected 3 user agents in pool, got %d", len(client.userAgentPool))
	}

	// Verify UA rotation
	ua1 := client.getEffectiveUserAgent()
	ua2 := client.getEffectiveUserAgent()
	ua3 := client.getEffectiveUserAgent()
	ua4 := client.getEffectiveUserAgent()

	if ua1 != "UA-1" || ua2 != "UA-2" || ua3 != "UA-3" || ua4 != "UA-1" {
		t.Errorf("UA rotation sequence failed: %s, %s, %s, %s", ua1, ua2, ua3, ua4)
	}
}

func TestExecuteRequestExponentialBackoffOn500(t *testing.T) {
	attempts := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		attempts++
		if attempts < 3 {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("recovered on attempt 3"))
	}))
	defer server.Close()

	client := NewClient(
		WithMaxRetries(3),
		WithRateLimit(50), // fast
	)

	body, err := client.executeRequest(context.Background(), server.URL)
	if err != nil {
		t.Fatalf("executeRequest with backoff failed: %v", err)
	}

	if string(body) != "recovered on attempt 3" {
		t.Errorf("Expected body 'recovered on attempt 3', got %q", string(body))
	}

	if attempts != 3 {
		t.Errorf("Expected exactly 3 attempts, got %d", attempts)
	}
}
