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

	"github.com/Nosvemos/tradingview-calendar-go/pkg/tvcalendar"
)

var (
	clientRegistry = make(map[int64]*tvcalendar.Client)
	registryMu     sync.Mutex
	nextHandle     int64 = 1
)

func registerClient(c *tvcalendar.Client) int64 {
	registryMu.Lock()
	defer registryMu.Unlock()
	h := nextHandle
	clientRegistry[h] = c
	nextHandle++
	return h
}

func getClient(h int64) *tvcalendar.Client {
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

func initClientWithCustomOptions(goStr string, extraOpts ...tvcalendar.Option) int64 {
	var opts ClientOptions

	// Default values
	opts.RateLimit = 10
	opts.Concurrency = 5

	if goStr != "" {
		_ = json.Unmarshal([]byte(goStr), &opts)
	}

	var clientOpts []tvcalendar.Option

	if opts.UserAgent != "" {
		clientOpts = append(clientOpts, tvcalendar.WithUserAgent(opts.UserAgent))
	}
	if opts.ProxyURL != "" {
		clientOpts = append(clientOpts, tvcalendar.WithProxy(opts.ProxyURL))
	}
	if opts.RateLimit > 0 {
		clientOpts = append(clientOpts, tvcalendar.WithRateLimit(opts.RateLimit))
	}
	if opts.Concurrency > 0 {
		clientOpts = append(clientOpts, tvcalendar.WithConcurrency(opts.Concurrency))
	}
	if opts.Timezone != "" {
		if loc, err := time.LoadLocation(opts.Timezone); err == nil {
			clientOpts = append(clientOpts, tvcalendar.WithTimeLocation(loc))
		}
	}
	if len(opts.Impacts) > 0 {
		var imps []tvcalendar.Impact
		for _, impStr := range opts.Impacts {
			imps = append(imps, tvcalendar.Impact(impStr))
		}
		clientOpts = append(clientOpts, tvcalendar.WithImpactFilter(imps...))
	}
	if len(opts.Currencies) > 0 {
		clientOpts = append(clientOpts, tvcalendar.WithCurrencyFilter(opts.Currencies...))
	}
	if len(opts.Countries) > 0 {
		clientOpts = append(clientOpts, tvcalendar.WithCountryFilter(opts.Countries...))
	}

	clientOpts = append(clientOpts, extraOpts...)

	client := tvcalendar.NewClient(clientOpts...)
	return registerClient(client)
}

func initClientFromJSON(goStr string) int64 {
	return initClientWithCustomOptions(goStr)
}

func fetchWeekJSON(handle int64, timestamp int64) string {
	client := getClient(handle)
	if client == nil {
		return `{"error": "client handle not found"}`
	}

	date := time.Unix(timestamp, 0).UTC()
	events, err := client.FetchWeek(context.Background(), date)
	if err != nil {
		errMap := map[string]string{"error": err.Error()}
		errJSON, _ := json.Marshal(errMap)
		return string(errJSON)
	}

	resJSON, err := json.Marshal(events)
	if err != nil {
		errMap := map[string]string{"error": "failed to marshal response: " + err.Error()}
		errJSON, _ := json.Marshal(errMap)
		return string(errJSON)
	}

	return string(resJSON)
}

func fetchRangeJSON(handle int64, startTS, endTS int64) string {
	client := getClient(handle)
	if client == nil {
		return `{"error": "client handle not found"}`
	}

	start := time.Unix(startTS, 0).UTC()
	end := time.Unix(endTS, 0).UTC()

	events, err := client.FetchRange(context.Background(), start, end)
	if err != nil {
		errMap := map[string]string{"error": err.Error()}
		errJSON, _ := json.Marshal(errMap)
		return string(errJSON)
	}

	resJSON, err := json.Marshal(events)
	if err != nil {
		errMap := map[string]string{"error": "failed to marshal response: " + err.Error()}
		errJSON, _ := json.Marshal(errMap)
		return string(errJSON)
	}

	return string(resJSON)
}

func fetchLiveFeedJSON(handle int64) string {
	client := getClient(handle)
	if client == nil {
		return `{"error": "client handle not found"}`
	}

	events, err := client.FetchLiveFeed(context.Background())
	if err != nil {
		errMap := map[string]string{"error": err.Error()}
		errJSON, _ := json.Marshal(errMap)
		return string(errJSON)
	}

	resJSON, err := json.Marshal(events)
	if err != nil {
		errMap := map[string]string{"error": "failed to marshal response: " + err.Error()}
		errJSON, _ := json.Marshal(errMap)
		return string(errJSON)
	}

	return string(resJSON)
}

//export InitClient
func InitClient(optsJSON *C.char) C.longlong {
	var goStr string
	if optsJSON != nil {
		goStr = C.GoString(optsJSON)
	}
	handle := initClientFromJSON(goStr)
	return C.longlong(handle)
}

//export FreeClient
func FreeClient(handle C.longlong) {
	unregisterClient(int64(handle))
}

//export FetchWeekJSON
func FetchWeekJSON(handle C.longlong, timestamp C.longlong) *C.char {
	resStr := fetchWeekJSON(int64(handle), int64(timestamp))
	return C.CString(resStr)
}

//export FetchRangeJSON
func FetchRangeJSON(handle C.longlong, startTS C.longlong, endTS C.longlong) *C.char {
	resStr := fetchRangeJSON(int64(handle), int64(startTS), int64(endTS))
	return C.CString(resStr)
}

//export FetchLiveFeedJSON
func FetchLiveFeedJSON(handle C.longlong) *C.char {
	resStr := fetchLiveFeedJSON(int64(handle))
	return C.CString(resStr)
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
