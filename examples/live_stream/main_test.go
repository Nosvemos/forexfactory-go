package main

import (
	"bytes"
	"io"
	"net/http"
	"testing"

	"github.com/Nosvemos/tradingview-calendar-go/pkg/tvcalendar"
)

type roundTripFunc func(req *http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return f(req)
}

func TestLiveStreamUpdate(t *testing.T) {
	mockJSON := `{"status":"ok","result":[{"id":"1","title":"CPI","country":"US","currency":"USD","importance":1,"actual":2.5,"forecast":2.4,"date":"2025-01-15T13:30:00Z"}]}`
	mockClient := &http.Client{
		Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
			return &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(bytes.NewBufferString(mockJSON)),
				Header:     make(http.Header),
			}, nil
		}),
	}

	client := tvcalendar.NewClient(
		tvcalendar.WithHTTPClient(mockClient),
		tvcalendar.WithTimeLocation(nil),
	)
	defer client.Close()

	updateLiveFeed(client)
}
