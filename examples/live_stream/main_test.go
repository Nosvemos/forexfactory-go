package main

import (
	"bytes"
	"context"
	"io"
	"net/http"
	"testing"
	"time"

	"github.com/Nosvemos/tradingview-calendar-go/pkg/tvcalendar"
)

type roundTripFunc func(req *http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return f(req)
}

func TestLiveStreamLoopAndFeed(t *testing.T) {
	mockJSON := `{"status":"ok","result":[{"id":"1","title":"CPI","country":"US","currency":"USD","importance":1,"actual":2.5,"forecast":2.4,"previous":2.3,"date":"2025-01-15T13:30:00Z"}]}`
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
		tvcalendar.WithTimeLocation(time.UTC),
	)
	defer client.Close()

	// Test runLiveStream ticker loop
	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	runLiveStream(ctx, client, 20*time.Millisecond)

	// Test updateLiveFeed with 0 high-impact events
	mockLowJSON := `{"status":"ok","result":[{"id":"2","title":"Low Impact","country":"US","currency":"USD","importance":-1,"date":"2025-01-15T13:30:00Z"}]}`
	lowClient := tvcalendar.NewClient(
		tvcalendar.WithHTTPClient(&http.Client{
			Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
				return &http.Response{
					StatusCode: http.StatusOK,
					Body:       io.NopCloser(bytes.NewBufferString(mockLowJSON)),
					Header:     make(http.Header),
				}, nil
			}),
		}),
	)
	defer lowClient.Close()
	updateLiveFeed(lowClient)

	// Test updateLiveFeed error
	errClient := tvcalendar.NewClient(
		tvcalendar.WithHTTPClient(&http.Client{
			Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
				return nil, io.ErrClosedPipe
			}),
		}),
	)
	defer errClient.Close()
	updateLiveFeed(errClient)
}
