package main

import (
	"bytes"
	"io"
	"net/http"
	"os"
	"testing"
)

type roundTripFunc func(req *http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return f(req)
}

func TestRangeDownloadExample(t *testing.T) {
	mockJSON := `{"status":"ok","result":[{"id":"1","title":"NFP","country":"US","currency":"USD","importance":1,"date":"2024-01-10T13:30:00Z"}]}`
	http.DefaultClient.Transport = roundTripFunc(func(req *http.Request) (*http.Response, error) {
		return &http.Response{
			StatusCode: http.StatusOK,
			Body:       io.NopCloser(bytes.NewBufferString(mockJSON)),
			Header:     make(http.Header),
		}, nil
	})

	defer os.Remove("demo_calendar.db")
	main()

	// Run with 0 USD events
	mockEmptyJSON := `{"status":"ok","result":[{"id":"2","title":"EUR Rate","country":"EU","currency":"EUR","importance":0,"date":"2024-01-10T13:30:00Z"}]}`
	http.DefaultClient.Transport = roundTripFunc(func(req *http.Request) (*http.Response, error) {
		return &http.Response{
			StatusCode: http.StatusOK,
			Body:       io.NopCloser(bytes.NewBufferString(mockEmptyJSON)),
			Header:     make(http.Header),
		}, nil
	})
	main()
}
