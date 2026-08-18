package tvcalendar

import (
	"bytes"
	"context"
	"io"
	"net/http"
	"os"
	"testing"
	"time"
)

type roundTripFunc func(req *http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return f(req)
}

func newMockHTTPClient(handler roundTripFunc) *http.Client {
	return &http.Client{
		Transport: handler,
	}
}

func TestClientFetchRangeWithMock(t *testing.T) {
	mockJSON := `{
		"status": "ok",
		"result": [
			{
				"id": "1001",
				"title": "US Non-Farm Payrolls",
				"country": "US",
				"currency": "USD",
				"actual": 250.0,
				"forecast": 180.0,
				"previous": 160.0,
				"unit": "K",
				"importance": 1,
				"date": "2025-01-10T13:30:00Z"
			},
			{
				"id": "1002",
				"title": "ECB Interest Rate Decision",
				"country": "EU",
				"currency": "EUR",
				"actual": 3.25,
				"forecast": 3.25,
				"previous": 3.50,
				"unit": "%",
				"importance": 1,
				"date": "2025-01-16T12:45:00Z"
			}
		]
	}`

	mockClient := newMockHTTPClient(func(req *http.Request) (*http.Response, error) {
		if req.Header.Get("Origin") != "https://www.tradingview.com" {
			t.Errorf("Missing required Origin header")
		}
		return &http.Response{
			StatusCode: http.StatusOK,
			Body:       io.NopCloser(bytes.NewBufferString(mockJSON)),
			Header:     make(http.Header),
		}, nil
	})

	var progressCalled bool
	client := NewClient(
		WithHTTPClient(mockClient),
		WithConcurrency(2),
		WithTimeLocation(time.UTC),
		WithProgressCallback(func(current, total int) {
			progressCalled = true
		}),
	)
	defer client.Close()

	ctx := context.Background()
	start := time.Date(2025, time.January, 1, 0, 0, 0, 0, time.UTC)
	end := time.Date(2025, time.January, 31, 0, 0, 0, 0, time.UTC)

	events, err := client.FetchRange(ctx, start, end)
	if err != nil {
		t.Fatalf("FetchRange failed: %v", err)
	}

	if len(events) != 2 {
		t.Fatalf("Expected 2 events, got %d", len(events))
	}

	if !progressCalled {
		t.Errorf("Expected progress callback to be called")
	}

	if events[0].Title != "US Non-Farm Payrolls" || events[0].Actual != "250K" {
		t.Errorf("Unexpected event 0 data: %+v", events[0])
	}
	if events[1].Title != "ECB Interest Rate Decision" || events[1].Actual != "3.25%" {
		t.Errorf("Unexpected event 1 data: %+v", events[1])
	}
}

func TestClientFetchHelpers(t *testing.T) {
	mockJSON := `{
		"status": "ok",
		"result": [
			{"id": "1", "title": "CPI", "country": "US", "currency": "USD", "importance": 1, "date": "2025-01-15T10:00:00Z"}
		]
	}`

	mockClient := newMockHTTPClient(func(req *http.Request) (*http.Response, error) {
		return &http.Response{
			StatusCode: http.StatusOK,
			Body:       io.NopCloser(bytes.NewBufferString(mockJSON)),
			Header:     make(http.Header),
		}, nil
	})

	client := NewClient(
		WithHTTPClient(mockClient),
		WithUserAgent("CustomAgent/1.0"),
		WithMaxRetries(2),
		WithProxy("http://127.0.0.1:8080"),
		WithCountryFilter("US"),
	)
	defer client.Close()

	ctx := context.Background()

	// 1. FetchMonth
	monthEvents, err := client.FetchMonth(ctx, 2025, time.January)
	if err != nil || len(monthEvents) != 1 {
		t.Errorf("FetchMonth failed: %v, len=%d", err, len(monthEvents))
	}

	// 2. FetchWeek
	weekEvents, err := client.FetchWeek(ctx, time.Date(2025, 1, 15, 0, 0, 0, 0, time.UTC))
	if err != nil || len(weekEvents) != 1 {
		t.Errorf("FetchWeek failed: %v, len=%d", err, len(weekEvents))
	}

	// 3. FetchLiveFeed
	liveEvents, err := client.FetchLiveFeed(ctx)
	if err != nil || len(liveEvents) != 1 {
		t.Errorf("FetchLiveFeed failed: %v, len=%d", err, len(liveEvents))
	}

	// 4. StreamLive
	streamCtx, streamCancel := context.WithTimeout(ctx, 100*time.Millisecond)
	defer streamCancel()

	eventsChan, errChan := client.StreamLive(streamCtx, 50*time.Millisecond)
	select {
	case evs := <-eventsChan:
		if len(evs) != 1 {
			t.Errorf("Expected 1 event from StreamLive, got %d", len(evs))
		}
	case errStream := <-errChan:
		t.Errorf("StreamLive error: %v", errStream)
	case <-time.After(1 * time.Second):
		t.Errorf("StreamLive timed out")
	}
}

func TestClientRetryAndErrorHandling(t *testing.T) {
	attempts := 0
	mockClient := newMockHTTPClient(func(req *http.Request) (*http.Response, error) {
		attempts++
		if attempts < 2 {
			return &http.Response{
				StatusCode: http.StatusInternalServerError,
				Status:     "500 Internal Server Error",
				Body:       io.NopCloser(bytes.NewBufferString("server error")),
				Header:     make(http.Header),
			}, nil
		}
		return &http.Response{
			StatusCode: http.StatusOK,
			Body:       io.NopCloser(bytes.NewBufferString(`{"status":"ok","result":[]}`)),
			Header:     make(http.Header),
		}, nil
	})

	client := NewClient(
		WithHTTPClient(mockClient),
		WithMaxRetries(3),
	)
	defer client.Close()

	events, err := client.FetchDay(context.Background(), time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC))
	if err != nil {
		t.Fatalf("Expected retry to succeed, got error: %v", err)
	}
	if len(events) != 0 {
		t.Errorf("Expected 0 events, got %d", len(events))
	}
	if attempts != 2 {
		t.Errorf("Expected 2 attempts, got %d", attempts)
	}
}

func TestClientInvalidRange(t *testing.T) {
	client := NewClient()
	defer client.Close()

	start := time.Date(2025, 2, 1, 0, 0, 0, 0, time.UTC)
	end := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)

	_, err := client.FetchRange(context.Background(), start, end)
	if err == nil {
		t.Errorf("Expected error when start is after end, got nil")
	}
}

func TestClientFilters(t *testing.T) {
	mockJSON := `{
		"status": "ok",
		"result": [
			{"id": "1", "title": "High USD", "country": "US", "currency": "USD", "importance": 1, "date": "2025-01-02T10:00:00Z"},
			{"id": "2", "title": "Low USD", "country": "US", "currency": "USD", "importance": -1, "date": "2025-01-02T11:00:00Z"},
			{"id": "3", "title": "High EUR", "country": "DE", "currency": "EUR", "importance": 1, "date": "2025-01-02T12:00:00Z"}
		]
	}`

	mockClient := newMockHTTPClient(func(req *http.Request) (*http.Response, error) {
		return &http.Response{
			StatusCode: http.StatusOK,
			Body:       io.NopCloser(bytes.NewBufferString(mockJSON)),
			Header:     make(http.Header),
		}, nil
	})

	client := NewClient(
		WithHTTPClient(mockClient),
		WithImpactFilter(ImpactHigh),
		WithCurrencyFilter("USD"),
	)
	defer client.Close()

	events, err := client.FetchDay(context.Background(), time.Date(2025, 1, 2, 0, 0, 0, 0, time.UTC))
	if err != nil {
		t.Fatalf("FetchDay failed: %v", err)
	}

	if len(events) != 1 {
		t.Fatalf("Expected 1 filtered event, got %d", len(events))
	}
	if events[0].ID != "1" || events[0].Title != "High USD" {
		t.Errorf("Mismatch in filtered event: %+v", events[0])
	}
}

func TestExcelAndParquetExport(t *testing.T) {
	events := []Event{
		{
			ID:       "1",
			Title:    "US CPI MoM",
			Country:  "US",
			Currency: "USD",
			Date:     time.Date(2025, 1, 15, 13, 30, 0, 0, time.UTC),
			Impact:   ImpactHigh,
			Actual:   "0.4%",
			Forecast: "0.3%",
			Previous: "0.2%",
		},
		{
			ID:       "2",
			Title:    "Retail Sales",
			Country:  "US",
			Currency: "USD",
			Date:     time.Date(2025, 1, 15, 14, 30, 0, 0, time.UTC),
			Impact:   ImpactMedium,
			Actual:   "0.1%",
			Forecast: "0.2%",
			Previous: "0.3%",
		},
		{
			ID:       "3",
			Title:    "Mortgage Approvals",
			Country:  "GB",
			Currency: "GBP",
			Date:     time.Date(2025, 1, 15, 9, 30, 0, 0, time.UTC),
			Impact:   ImpactLow,
			Actual:   "50K",
			Forecast: "52K",
			Previous: "51K",
		},
		{
			ID:       "4",
			Title:    "Bank Holiday",
			Country:  "JP",
			Currency: "JPY",
			Date:     time.Date(2025, 1, 15, 0, 0, 0, 0, time.UTC),
			Impact:   ImpactNone,
			IsAllDay: true,
		},
	}

	tempExcel := "temp_dir/test_export.xlsx"
	defer os.RemoveAll("temp_dir")

	if err := WriteExcel(events, tempExcel); err != nil {
		t.Fatalf("WriteExcel failed: %v", err)
	}

	tempParquet := "temp_parquet_dir/test_export.parquet"
	defer os.RemoveAll("temp_parquet_dir")

	if err := WriteParquet(tempParquet, events); err != nil {
		t.Fatalf("WriteParquet failed: %v", err)
	}
}
