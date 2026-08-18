package main

import (
	"bytes"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/Nosvemos/tradingview-calendar-go/pkg/tvcalendar"
)

type roundTripFunc func(req *http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return f(req)
}

func TestClientRegistry(t *testing.T) {
	client := tvcalendar.NewClient()
	handle := registerClient(client)

	if handle <= 0 {
		t.Fatalf("Expected valid positive handle, got %d", handle)
	}

	retrieved := getClient(handle)
	if retrieved != client {
		t.Errorf("Expected retrieved client to match registered instance")
	}

	unregisterClient(handle)

	retrievedAfter := getClient(handle)
	if retrievedAfter != nil {
		t.Errorf("Expected client to be nil after unregister, got %+v", retrievedAfter)
	}
}

func TestInitClientOptionsPermutationsAndSuccessfulFetch(t *testing.T) {
	mockJSON := `{"status":"ok","result":[{"id":"1","title":"NFP","country":"US","currency":"USD","importance":1,"date":"2025-01-15T13:30:00Z"}]}`
	mockClient := &http.Client{
		Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
			return &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(bytes.NewBufferString(mockJSON)),
				Header:     make(http.Header),
			}, nil
		}),
	}

	// 1. Full options with valid timezone and mock client
	jsonFull := `{"user_agent":"Agent/2.0","proxy_url":"","rate_limit":20,"concurrency":4,"timezone":"America/New_York","impacts":["High","Medium","Low"],"currencies":["USD","EUR"],"countries":["US","DE"]}`
	handleFull := initClientWithCustomOptions(jsonFull, tvcalendar.WithHTTPClient(mockClient))
	if handleFull <= 0 {
		t.Fatalf("Expected valid handle, got %d", handleFull)
	}

	// 2. Options with invalid timezone to trigger error fallback
	jsonInvalidTZ := `{"timezone":"Invalid/Unknown_Zone","rate_limit":5,"concurrency":1}`
	handleInvalidTZ := initClientFromJSON(jsonInvalidTZ)
	if handleInvalidTZ <= 0 {
		t.Fatalf("Expected valid handle for invalid timezone fallback, got %d", handleInvalidTZ)
	}

	// 3. Valid successful calls on handle
	weekRes := fetchWeekJSON(handleFull, time.Date(2025, 1, 15, 0, 0, 0, 0, time.UTC).Unix())
	if !strings.Contains(weekRes, "NFP") {
		t.Errorf("Expected NFP in week response, got %s", weekRes)
	}

	rangeRes := fetchRangeJSON(handleFull, time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC).Unix(), time.Date(2025, 1, 15, 0, 0, 0, 0, time.UTC).Unix())
	if !strings.Contains(rangeRes, "NFP") {
		t.Errorf("Expected NFP in range response, got %s", rangeRes)
	}

	liveRes := fetchLiveFeedJSON(handleFull)
	if !strings.Contains(liveRes, "NFP") {
		t.Errorf("Expected NFP in live response, got %s", liveRes)
	}

	// 4. Invalid range error
	rangeErrRes := fetchRangeJSON(handleFull, 5000, 1000)
	if !strings.Contains(rangeErrRes, "error") {
		t.Errorf("Expected error on range start after end, got %s", rangeErrRes)
	}

	// 5. Invalid handles
	if res := fetchWeekJSON(999999, 100); !strings.Contains(res, "handle not found") {
		t.Errorf("Expected handle not found, got %s", res)
	}
	if res := fetchRangeJSON(999999, 100, 200); !strings.Contains(res, "handle not found") {
		t.Errorf("Expected handle not found, got %s", res)
	}
	if res := fetchLiveFeedJSON(999999); !strings.Contains(res, "handle not found") {
		t.Errorf("Expected handle not found, got %s", res)
	}

	unregisterClient(handleFull)
	unregisterClient(handleInvalidTZ)
}
