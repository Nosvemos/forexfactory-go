package main

import (
	"testing"

	"github.com/Nosvemos/tradingview-calendar-go/pkg/tvcalendar"
)

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

func TestClientOptionsStruct(t *testing.T) {
	opts := ClientOptions{
		UserAgent:   "TestAgent",
		ProxyURL:    "http://127.0.0.1:8080",
		RateLimit:   20,
		Concurrency: 8,
		Timezone:    "UTC",
		Impacts:     []string{"High"},
		Currencies:  []string{"USD"},
		Countries:   []string{"US"},
	}

	if opts.RateLimit != 20 || opts.Concurrency != 8 {
		t.Errorf("Unexpected options values: %+v", opts)
	}
}
