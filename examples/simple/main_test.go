package main

import (
	"bytes"
	"io"
	"net/http"
	"testing"
)

type roundTripFunc func(req *http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return f(req)
}

func TestSimpleExample(t *testing.T) {
	mockJSON := `{"status":"ok","result":[{"id":"1","title":"CPI","country":"US","currency":"USD","importance":1,"date":"2025-01-15T13:30:00Z"}]}`
	http.DefaultClient.Transport = roundTripFunc(func(req *http.Request) (*http.Response, error) {
		return &http.Response{
			StatusCode: http.StatusOK,
			Body:       io.NopCloser(bytes.NewBufferString(mockJSON)),
			Header:     make(http.Header),
		}, nil
	})

	main()
}
