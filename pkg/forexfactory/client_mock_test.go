package forexfactory

import (
	"bytes"
	"context"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"
)

// mockRoundTripper intercepts HTTP requests and returns pre-configured mock responses.
type mockRoundTripper struct {
	roundTripFunc func(req *http.Request) (*http.Response, error)
}

func (m *mockRoundTripper) RoundTrip(req *http.Request) (*http.Response, error) {
	return m.roundTripFunc(req)
}

func TestClientOfflineMockScrape(t *testing.T) {
	// Mock XML Live Feed payload
	mockXML := `<?xml version="1.0" encoding="UTF-8"?>
	<weeklyevents>
		<event>
			<title>Mock Interest Rate Decision</title>
			<country>USD</country>
			<date>05-26-2026</date>
			<time>2:00pm</time>
			<impact>High</impact>
			<forecast>5.25%</forecast>
			<previous>5.00%</previous>
			<actual>5.25%</actual>
		</event>
		<event>
			<title>Mock German CPI m/m</title>
			<country>EUR</country>
			<date>05-26-2026</date>
			<time>All Day</time>
			<impact>Medium</impact>
			<forecast>0.2%</forecast>
			<previous>0.1%</previous>
			<actual>0.3%</actual>
		</event>
	</weeklyevents>`

	// Mock HTML Weekly Calendar payload
	mockHTML := `<!DOCTYPE html>
	<html>
	<body>
		<table class="calendar__table">
			<tr class="calendar__row">
				<td class="calendar__currency">USD</td>
				<td class="calendar__event"><a href="/calendar?show=99887">Mock CPI m/m</a></td>
				<td class="calendar__date">Tue May 26</td>
				<td class="calendar__time">8:30am</td>
				<td class="calendar__impact"><span class="icon--impact-red red"></span></td>
				<td class="calendar__actual">0.3%</td>
				<td class="calendar__forecast">0.2%</td>
				<td class="calendar__previous">0.1%</td>
			</tr>
		</table>
	</body>
	</html>`

	mockTransport := &mockRoundTripper{
		roundTripFunc: func(req *http.Request) (*http.Response, error) {
			urlStr := req.URL.String()

			if strings.Contains(urlStr, "ff_calendar_thisweek.xml") {
				return &http.Response{
					StatusCode: http.StatusOK,
					Body:       io.NopCloser(bytes.NewBufferString(mockXML)),
					Header:     make(http.Header),
				}, nil
			}

			if strings.Contains(urlStr, "calendar?week=") {
				return &http.Response{
					StatusCode: http.StatusOK,
					Body:       io.NopCloser(bytes.NewBufferString(mockHTML)),
					Header:     make(http.Header),
				}, nil
			}

			return &http.Response{
				StatusCode: http.StatusNotFound,
				Body:       io.NopCloser(bytes.NewBufferString("Not Found")),
				Header:     make(http.Header),
			}, nil
		},
	}

	mockHTTPClient := &http.Client{
		Transport: mockTransport,
		Timeout:   2 * time.Second,
	}

	client := NewClient(
		WithHTTPClient(mockHTTPClient),
		WithTimeLocation(time.UTC),
	)

	ctx := context.Background()

	// 1. Verify Offline XML Live Feed parsing
	liveEvents, err := client.FetchLiveFeed(ctx)
	if err != nil {
		t.Fatalf("FetchLiveFeed mock failed: %v", err)
	}

	if len(liveEvents) != 2 {
		t.Errorf("Expected 2 live events, got %d", len(liveEvents))
	}

	usdEvent := liveEvents[0]
	if usdEvent.Title != "Mock Interest Rate Decision" || usdEvent.Currency != "USD" || usdEvent.Impact != ImpactHigh {
		t.Errorf("Mismatch in parsed USD XML live event data: %+v", usdEvent)
	}

	eurEvent := liveEvents[1]
	if eurEvent.Currency != "EUR" || !eurEvent.IsAllDay || eurEvent.Impact != ImpactMedium {
		t.Errorf("Mismatch in parsed EUR XML live event data: %+v", eurEvent)
	}

	// 2. Verify Offline HTML Calendar parsing
	testDate := time.Date(2026, time.May, 26, 0, 0, 0, 0, time.UTC)
	weekEvents, err := client.FetchWeek(ctx, testDate)
	if err != nil {
		t.Fatalf("FetchWeek mock failed: %v", err)
	}

	if len(weekEvents) != 1 {
		t.Fatalf("Expected 1 weekly html event, got %d", len(weekEvents))
	}

	htmlEvent := weekEvents[0]
	if htmlEvent.ID != "99887" || htmlEvent.Title != "Mock CPI m/m" || htmlEvent.Impact != ImpactHigh {
		t.Errorf("Mismatch in parsed HTML event data: %+v", htmlEvent)
	}

	if htmlEvent.Actual != "0.3%" || htmlEvent.Forecast != "0.2%" || htmlEvent.Previous != "0.1%" {
		t.Errorf("Mismatch in parsed HTML stats: Actual=%s, Forecast=%s, Previous=%s", htmlEvent.Actual, htmlEvent.Forecast, htmlEvent.Previous)
	}
}
