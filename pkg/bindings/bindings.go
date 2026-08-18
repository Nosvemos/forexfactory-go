package main

/*
#include <stdlib.h>
*/
import "C"
import (
	"context"
	"encoding/json"
	"sync"
	"time"
	"unsafe"

	"github.com/Nosvemos/forexcalendar-go/pkg/forexcalendar"
)

var (
	clientRegistry = make(map[int64]*forexcalendar.Client)
	registryMu     sync.Mutex
	nextHandle     int64 = 1
)

func registerClient(c *forexcalendar.Client) int64 {
	registryMu.Lock()
	defer registryMu.Unlock()
	h := nextHandle
	clientRegistry[h] = c
	nextHandle++
	return h
}

func getClient(h int64) *forexcalendar.Client {
	registryMu.Lock()
	defer registryMu.Unlock()
	return clientRegistry[h]
}

func unregisterClient(h int64) {
	registryMu.Lock()
	defer registryMu.Unlock()
	if c, ok := clientRegistry[h]; ok {
		_ = c.Close()
		delete(clientRegistry, h)
	}
}

// ClientOptions represents the configuration payload passed from Python/C.
type ClientOptions struct {
	UserAgent   string   `json:"user_agent"`
	ProxyURL    string   `json:"proxy_url"`
	RateLimit   int      `json:"rate_limit"`
	Concurrency int      `json:"concurrency"`
	Timezone    string   `json:"timezone"`
	Impacts     []string `json:"impacts"`
	Currencies  []string `json:"currencies"`
	Countries   []string `json:"countries"`
}

//export InitClient
func InitClient(optsJSON *C.char) C.longlong {
	var goStr string
	if optsJSON != nil {
		goStr = C.GoString(optsJSON)
	}
	var opts ClientOptions

	// Default values
	opts.RateLimit = 10
	opts.Concurrency = 5

	if goStr != "" {
		_ = json.Unmarshal([]byte(goStr), &opts)
	}

	var clientOpts []forexcalendar.Option

	if opts.UserAgent != "" {
		clientOpts = append(clientOpts, forexcalendar.WithUserAgent(opts.UserAgent))
	}
	if opts.ProxyURL != "" {
		clientOpts = append(clientOpts, forexcalendar.WithProxy(opts.ProxyURL))
	}
	if opts.RateLimit > 0 {
		clientOpts = append(clientOpts, forexcalendar.WithRateLimit(opts.RateLimit))
	}
	if opts.Concurrency > 0 {
		clientOpts = append(clientOpts, forexcalendar.WithConcurrency(opts.Concurrency))
	}
	if opts.Timezone != "" {
		if loc, err := time.LoadLocation(opts.Timezone); err == nil {
			clientOpts = append(clientOpts, forexcalendar.WithTimeLocation(loc))
		}
	}
	if len(opts.Impacts) > 0 {
		var imps []forexcalendar.Impact
		for _, impStr := range opts.Impacts {
			imps = append(imps, forexcalendar.Impact(impStr))
		}
		clientOpts = append(clientOpts, forexcalendar.WithImpactFilter(imps...))
	}
	if len(opts.Currencies) > 0 {
		clientOpts = append(clientOpts, forexcalendar.WithCurrencyFilter(opts.Currencies...))
	}
	if len(opts.Countries) > 0 {
		clientOpts = append(clientOpts, forexcalendar.WithCountryFilter(opts.Countries...))
	}

	client := forexcalendar.NewClient(clientOpts...)
	handle := registerClient(client)

	return C.longlong(handle)
}

//export FreeClient
func FreeClient(handle C.longlong) {
	unregisterClient(int64(handle))
}

//export FetchWeekJSON
func FetchWeekJSON(handle C.longlong, timestamp C.longlong) *C.char {
	client := getClient(int64(handle))
	if client == nil {
		return C.CString(`{"error": "client handle not found"}`)
	}

	date := time.Unix(int64(timestamp), 0).UTC()
	events, err := client.FetchWeek(context.Background(), date)
	if err != nil {
		errMap := map[string]string{"error": err.Error()}
		errJSON, _ := json.Marshal(errMap)
		return C.CString(string(errJSON))
	}

	resJSON, err := json.Marshal(events)
	if err != nil {
		errMap := map[string]string{"error": "failed to marshal response: " + err.Error()}
		errJSON, _ := json.Marshal(errMap)
		return C.CString(string(errJSON))
	}

	return C.CString(string(resJSON))
}

//export FetchRangeJSON
func FetchRangeJSON(handle C.longlong, startTS C.longlong, endTS C.longlong) *C.char {
	client := getClient(int64(handle))
	if client == nil {
		return C.CString(`{"error": "client handle not found"}`)
	}

	start := time.Unix(int64(startTS), 0).UTC()
	end := time.Unix(int64(endTS), 0).UTC()

	events, err := client.FetchRange(context.Background(), start, end)
	if err != nil {
		errMap := map[string]string{"error": err.Error()}
		errJSON, _ := json.Marshal(errMap)
		return C.CString(string(errJSON))
	}

	resJSON, err := json.Marshal(events)
	if err != nil {
		errMap := map[string]string{"error": "failed to marshal response: " + err.Error()}
		errJSON, _ := json.Marshal(errMap)
		return C.CString(string(errJSON))
	}

	return C.CString(string(resJSON))
}

//export FetchLiveFeedJSON
func FetchLiveFeedJSON(handle C.longlong) *C.char {
	client := getClient(int64(handle))
	if client == nil {
		return C.CString(`{"error": "client handle not found"}`)
	}

	events, err := client.FetchLiveFeed(context.Background())
	if err != nil {
		errMap := map[string]string{"error": err.Error()}
		errJSON, _ := json.Marshal(errMap)
		return C.CString(string(errJSON))
	}

	resJSON, err := json.Marshal(events)
	if err != nil {
		errMap := map[string]string{"error": "failed to marshal response: " + err.Error()}
		errJSON, _ := json.Marshal(errMap)
		return C.CString(string(errJSON))
	}

	return C.CString(string(resJSON))
}

//export FreeString
func FreeString(str *C.char) {
	if str != nil {
		C.free(unsafe.Pointer(str))
	}
}

func main() {
	// Mandatory for c-shared build modes
}
