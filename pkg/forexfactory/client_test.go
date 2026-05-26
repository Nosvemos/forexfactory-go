package forexfactory

import (
	"context"
	"net/http"
	"net/http/httptest"
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
		if r.Header.Get("Cookie") != "test-cookie-val" {
			t.Errorf("Expected Cookie 'test-cookie-val', got '%s'", r.Header.Get("Cookie"))
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
