package main

import (
	"bytes"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/Nosvemos/tradingview-calendar-go/pkg/tvcalendar"
)

type roundTripFunc func(req *http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return f(req)
}

func TestNotifierImpactWeights(t *testing.T) {
	if !isImpactAllowed(tvcalendar.ImpactHigh, tvcalendar.ImpactMedium) {
		t.Errorf("Expected High impact to be allowed when minimum is Medium")
	}
	if isImpactAllowed(tvcalendar.ImpactLow, tvcalendar.ImpactHigh) {
		t.Errorf("Expected Low impact NOT to be allowed when minimum is High")
	}
	if !isImpactAllowed(tvcalendar.ImpactHigh, tvcalendar.ImpactHigh) {
		t.Errorf("Expected High impact to be allowed when minimum is High")
	}
	if !isImpactAllowed(tvcalendar.ImpactLow, tvcalendar.ImpactNone) {
		t.Errorf("Expected Low impact to be allowed when minimum is None")
	}
}

func TestNotifierCachePathAndPersistence(t *testing.T) {
	path := getNotifierCacheFilePath()
	if !strings.Contains(path, "tradingview-calendar-go") || !strings.HasSuffix(path, "notified_cache.json") {
		t.Errorf("Unexpected notifier cache file path: %s", path)
	}

	// Test save and load cache
	cacheMu.Lock()
	notifiedCache["test_key_1"] = time.Date(2025, 1, 1, 12, 0, 0, 0, time.UTC)
	cacheMu.Unlock()

	saveNotifiedCache()
	defer os.Remove(path)

	// Clear memory and reload
	cacheMu.Lock()
	notifiedCache = make(map[string]time.Time)
	cacheMu.Unlock()

	loadNotifiedCache()

	cacheMu.Lock()
	val, ok := notifiedCache["test_key_1"]
	cacheMu.Unlock()

	if !ok || val.Year() != 2025 {
		t.Errorf("Failed to persist and load cache: ok=%v, val=%v", ok, val)
	}

	// Test cleaning expired items from cache
	cacheMu.Lock()
	notifiedCache["old_key"] = time.Now().UTC().Add(-2 * time.Hour)
	notifiedCache["fresh_key"] = time.Now().UTC().Add(10 * time.Minute)
	cacheMu.Unlock()

	now := time.Now().UTC()
	changed := cleanCache(now)
	if !changed {
		t.Errorf("Expected cleanCache to report changes")
	}

	cacheMu.Lock()
	_, hasOld := notifiedCache["old_key"]
	_, hasFresh := notifiedCache["fresh_key"]
	cacheMu.Unlock()

	if hasOld {
		t.Errorf("Expected old_key to be purged")
	}
	if !hasFresh {
		t.Errorf("Expected fresh_key to be retained")
	}
}

func TestWebhooksDispatchAllImpactsAndErrors(t *testing.T) {
	tsSuccess := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer tsSuccess.Close()

	tsError := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer tsError.Close()

	impacts := []tvcalendar.Impact{
		tvcalendar.ImpactHigh,
		tvcalendar.ImpactMedium,
		tvcalendar.ImpactLow,
		tvcalendar.ImpactNone,
	}

	oldTransport := webhookHTTPClient.Transport
	defer func() { webhookHTTPClient.Transport = oldTransport }()

	for _, imp := range impacts {
		event := tvcalendar.Event{
			ID:       "999",
			Title:    "Rate Decision",
			Currency: "USD",
			Date:     time.Now().UTC().Add(10 * time.Minute),
			Impact:   imp,
			Forecast: "5.50%",
			Previous: "5.25%",
		}

		// 1. Success calls with 200 OK
		webhookHTTPClient.Transport = roundTripFunc(func(req *http.Request) (*http.Response, error) {
			return &http.Response{
				StatusCode: http.StatusOK,
				Status:     "200 OK",
				Body:       io.NopCloser(bytes.NewBufferString(`{"ok":true}`)),
				Header:     make(http.Header),
			}, nil
		})

		discordWebhookFlag = "https://discord.com/api/webhooks/dummy"
		if err := sendDiscordAlert(event, 10); err != nil {
			t.Errorf("Unexpected error from Discord: %v", err)
		}

		slackWebhookFlag = "https://hooks.slack.com/services/dummy"
		if err := sendSlackAlert(event, 10); err != nil {
			t.Errorf("Unexpected error from Slack: %v", err)
		}

		genericWebhookFlag = "https://example.com/webhook"
		if err := sendGenericWebhookAlert(event, 10); err != nil {
			t.Errorf("Unexpected error from generic webhook: %v", err)
		}

		telegramTokenFlag = "dummy"
		telegramChatFlag = "123"
		if err := sendTelegramAlert(event, 10); err != nil {
			t.Errorf("Unexpected error from Telegram: %v", err)
		}

		// 2. Error calls with 500
		webhookHTTPClient.Transport = roundTripFunc(func(req *http.Request) (*http.Response, error) {
			return &http.Response{
				StatusCode: http.StatusInternalServerError,
				Status:     "500 Internal Server Error",
				Body:       io.NopCloser(bytes.NewBufferString("error")),
				Header:     make(http.Header),
			}, nil
		})

		if err := sendDiscordAlert(event, 10); err == nil {
			t.Errorf("Expected error on 500 status from Discord")
		}

		if err := sendSlackAlert(event, 10); err == nil {
			t.Errorf("Expected error on 500 status from Slack")
		}

		if err := sendGenericWebhookAlert(event, 10); err == nil {
			t.Errorf("Expected error on 500 status from generic webhook")
		}

		if err := sendTelegramAlert(event, 10); err == nil {
			t.Errorf("Expected error on 500 status from Telegram")
		}

		// 3. Network connection errors
		webhookHTTPClient.Transport = roundTripFunc(func(req *http.Request) (*http.Response, error) {
			return nil, io.ErrClosedPipe
		})

		_ = sendDiscordAlert(event, 10)
		_ = sendSlackAlert(event, 10)
		_ = sendGenericWebhookAlert(event, 10)
		_ = sendTelegramAlert(event, 10)
	}

	// Test helpers
	esc := escapeMarkdown("Fed & FOMC *Rate* _Decision_ `code` [link]")
	if esc == "" {
		t.Errorf("escapeMarkdown returned empty")
	}
	if v := getValOrDash(""); v != "-" {
		t.Errorf("Expected '-' for empty value, got %q", v)
	}
	if v := getValOrDash("5.0%"); v != "5.0%" {
		t.Errorf("Expected '5.0%%', got %q", v)
	}

	// Reset flags
	discordWebhookFlag = ""
	slackWebhookFlag = ""
	genericWebhookFlag = ""
	telegramTokenFlag = ""
	telegramChatFlag = ""
}

func TestCheckAndAlertFlow(t *testing.T) {
	now := time.Now().UTC().Add(5 * time.Minute)
	mockJSON := `{
		"status": "ok",
		"result": [
			{
				"id": "777",
				"title": "US CPI",
				"country": "US",
				"currency": "USD",
				"importance": 1,
				"date": "` + now.Format(time.RFC3339) + `"
			},
			{
				"id": "778",
				"title": "Medium Event",
				"country": "US",
				"currency": "USD",
				"importance": 0,
				"date": "` + now.Format(time.RFC3339) + `"
			},
			{
				"id": "779",
				"title": "Low Event",
				"country": "US",
				"currency": "USD",
				"importance": -1,
				"date": "` + now.Format(time.RFC3339) + `"
			},
			{
				"id": "780",
				"title": "All Day Event",
				"country": "JP",
				"currency": "JPY",
				"all_day": true,
				"importance": 1,
				"date": "` + now.Format(time.RFC3339) + `"
			}
		]
	}`

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer ts.Close()

	discordWebhookFlag = ts.URL
	slackWebhookFlag = ts.URL
	genericWebhookFlag = ts.URL
	telegramTokenFlag = "dummy"
	telegramChatFlag = "123"
	leadTimeFlag = 15 * time.Minute

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

	checkAndAlert(client, tvcalendar.ImpactHigh)
	checkAndAlert(client, tvcalendar.ImpactLow)

	// Call third time to verify deduplication cache
	checkAndAlert(client, tvcalendar.ImpactLow)

	// Test error client path
	errClient := tvcalendar.NewClient(
		tvcalendar.WithHTTPClient(&http.Client{
			Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
				return nil, io.ErrClosedPipe
			}),
		}),
	)
	defer errClient.Close()
	checkAndAlert(errClient, tvcalendar.ImpactHigh)

	discordWebhookFlag = ""
	slackWebhookFlag = ""
	genericWebhookFlag = ""
	telegramTokenFlag = ""
	telegramChatFlag = ""
}

func TestNotifierRootCmd(t *testing.T) {
	if rootCmd.Use != "tv-notifier" {
		t.Errorf("Expected rootCmd use 'tv-notifier', got %s", rootCmd.Use)
	}
}

func TestValidateEndpointsConfigured(t *testing.T) {
	discordWebhookFlag = ""
	telegramTokenFlag = ""
	telegramChatFlag = ""
	slackWebhookFlag = ""
	genericWebhookFlag = ""

	if validateEndpointsConfigured() {
		t.Errorf("Expected validateEndpointsConfigured to return false when no flags are set")
	}

	discordWebhookFlag = "https://discord.com/api/webhooks/123"
	if !validateEndpointsConfigured() {
		t.Errorf("Expected validateEndpointsConfigured to return true for discord")
	}
	discordWebhookFlag = ""

	telegramTokenFlag = "token"
	telegramChatFlag = "chat"
	if !validateEndpointsConfigured() {
		t.Errorf("Expected validateEndpointsConfigured to return true for telegram")
	}
	telegramTokenFlag = ""
	telegramChatFlag = ""

	slackWebhookFlag = "https://hooks.slack.com/services/123"
	if !validateEndpointsConfigured() {
		t.Errorf("Expected validateEndpointsConfigured to return true for slack")
	}
	slackWebhookFlag = ""

	genericWebhookFlag = "https://webhook.site/123"
	if !validateEndpointsConfigured() {
		t.Errorf("Expected validateEndpointsConfigured to return true for generic webhook")
	}
	genericWebhookFlag = ""
}
