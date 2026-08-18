package forexcalendar

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
			ID:       "99",
			Title:    "US CPI MoM",
			Country:  "US",
			Currency: "USD",
			Date:     time.Date(2025, 1, 15, 13, 30, 0, 0, time.UTC),
			Impact:   ImpactHigh,
			Actual:   "0.4%",
			Forecast: "0.3%",
			Previous: "0.2%",
		},
	}

	tempExcel := "test_export.xlsx"
	defer os.Remove(tempExcel)

	if err := WriteExcel(events, tempExcel); err != nil {
		t.Fatalf("WriteExcel failed: %v", err)
	}

	tempParquet := "test_export.parquet"
	defer os.Remove(tempParquet)

	if err := WriteParquet(tempParquet, events); err != nil {
		t.Fatalf("WriteParquet failed: %v", err)
	}
}
