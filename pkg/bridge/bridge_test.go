package bridge

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/Nosvemos/tradingview-calendar-go/pkg/tvcalendar"
)

type roundTripFunc func(req *http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return f(req)
}

func TestBridgeDefaultConfig(t *testing.T) {
	b := NewBridge(BridgeConfig{})
	if b.cfg.Interval != 1*time.Minute {
		t.Errorf("Expected default interval 1m, got %v", b.cfg.Interval)
	}
	if b.cfg.MinImpact != tvcalendar.ImpactHigh {
		t.Errorf("Expected default min impact High, got %v", b.cfg.MinImpact)
	}
	if b.cfg.Timezone != time.UTC {
		t.Errorf("Expected default timezone UTC, got %v", b.cfg.Timezone)
	}
}

func TestBridgeAtomicFiles(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "mt4_test_*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	b := NewBridge(BridgeConfig{
		OutputDir: tempDir,
		MinImpact: tvcalendar.ImpactHigh,
	})

	now := time.Now().UTC()
	payload := NewsFilterPayload{
		LastUpdatedUTC: now.Format("2006-01-02 15:04:05"),
		NextEvent: &UpcomingEvent{
			ID:          "123",
			Title:       "FOMC Statement",
			Currency:    "USD",
			DateUTC:     now.Add(15 * time.Minute).Format("2006-01-02 15:04:05"),
			Timestamp:   now.Add(15 * time.Minute).Unix(),
			MinutesLeft: 15,
			Impact:      "High",
			Forecast:    "5.25%",
			Previous:    "5.50%",
		},
		Events: []UpcomingEvent{
			{
				ID:          "123",
				Title:       "FOMC Statement",
				Currency:    "USD",
				DateUTC:     now.Add(15 * time.Minute).Format("2006-01-02 15:04:05"),
				Timestamp:   now.Add(15 * time.Minute).Unix(),
				MinutesLeft: 15,
				Impact:      "High",
				Forecast:    "5.25%",
				Previous:    "5.50%",
			},
		},
	}

	if err := b.writeAtomicFiles(payload); err != nil {
		t.Fatalf("writeAtomicFiles failed: %v", err)
	}

	// Verify JSON file exists and matches
	jsonPath := filepath.Join(tempDir, "ff_news_filter.json")
	data, err := os.ReadFile(jsonPath)
	if err != nil {
		t.Fatalf("Failed to read generated JSON: %v", err)
	}

	var parsed NewsFilterPayload
	if err := json.Unmarshal(data, &parsed); err != nil {
		t.Fatalf("Failed to parse generated JSON: %v", err)
	}

	if len(parsed.Events) != 1 || parsed.Events[0].Title != "FOMC Statement" {
		t.Errorf("JSON payload mismatch: %+v", parsed)
	}

	// Verify CSV file exists
	csvPath := filepath.Join(tempDir, "ff_news_filter.csv")
	csvData, err := os.ReadFile(csvPath)
	if err != nil {
		t.Fatalf("Failed to read generated CSV: %v", err)
	}

	if len(csvData) == 0 {
		t.Errorf("CSV file is empty")
	}

	// Test writing with empty OutputDir (falls back to .)
	bEmptyDir := NewBridge(BridgeConfig{OutputDir: ""})
	defer os.Remove("ff_news_filter.json")
	defer os.Remove("ff_news_filter.csv")
	if err := bEmptyDir.writeAtomicFiles(payload); err != nil {
		t.Fatalf("writeAtomicFiles with empty output dir failed: %v", err)
	}
}

func TestBridgeSyncOnce(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "mt4_sync_*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	now := time.Now().UTC()
	recentEvt := now.Add(-30 * time.Minute).Format(time.RFC3339)
	futureEvt := now.Add(30 * time.Minute).Format(time.RFC3339)
	farPastEvt := now.Add(-120 * time.Minute).Format(time.RFC3339)
	farFutureEvt := now.Add(10 * 24 * time.Hour).Format(time.RFC3339)

	mockJSON := `{
		"status": "ok",
		"result": [
			{"id": "500", "title": "US NFP", "country": "US", "currency": "USD", "importance": 1, "date": "` + futureEvt + `"},
			{"id": "501", "title": "Past Event", "country": "US", "currency": "USD", "importance": 1, "date": "` + recentEvt + `"},
			{"id": "502", "title": "Too Old Event", "country": "US", "currency": "USD", "importance": 1, "date": "` + farPastEvt + `"},
			{"id": "503", "title": "Too Far Future Event", "country": "US", "currency": "USD", "importance": 1, "date": "` + farFutureEvt + `"},
			{"id": "504", "title": "Unmatched Currency", "country": "JP", "currency": "JPY", "importance": 1, "date": "` + futureEvt + `"},
			{"id": "505", "title": "Low Impact Event", "country": "US", "currency": "USD", "importance": -1, "date": "` + futureEvt + `"}
		]
	}`

	mockClient := &http.Client{
		Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
			return &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(bytes.NewBufferString(mockJSON)),
				Header:     make(http.Header),
			}, nil
		}),
	}

	b := &Bridge{
		cfg: BridgeConfig{
			OutputDir:  tempDir,
			MinImpact:  tvcalendar.ImpactHigh,
			Currencies: []string{"USD"},
		},
		client: tvcalendar.NewClient(tvcalendar.WithHTTPClient(mockClient)),
	}

	ctx := context.Background()
	if err := b.SyncOnce(ctx); err != nil {
		t.Fatalf("SyncOnce failed: %v", err)
	}

	jsonPath := filepath.Join(tempDir, "ff_news_filter.json")
	if _, err := os.Stat(jsonPath); err != nil {
		t.Errorf("Expected news filter json file to be created, got error: %v", err)
	}
}

func TestBridgeStartAndCancel(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "mt4_cancel_*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	mockJSON := `{"status":"ok","result":[]}`
	mockClient := &http.Client{
		Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
			return &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(bytes.NewBufferString(mockJSON)),
				Header:     make(http.Header),
			}, nil
		}),
	}

	b := &Bridge{
		cfg: BridgeConfig{
			OutputDir:  tempDir,
			MinImpact:  tvcalendar.ImpactMedium,
			Interval:   20 * time.Millisecond,
			Currencies: []string{"USD", "EUR"},
		},
		client: tvcalendar.NewClient(tvcalendar.WithHTTPClient(mockClient)),
	}

	ctx, cancel := context.WithTimeout(context.Background(), 75*time.Millisecond)
	defer cancel()

	_ = b.Start(ctx)
}

func TestBridgeErrorHandling(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "mt4_err_*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	mockClient := &http.Client{
		Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
			return nil, fmt.Errorf("network error")
		}),
	}

	b := &Bridge{
		cfg: BridgeConfig{
			OutputDir: tempDir,
		},
		client: tvcalendar.NewClient(tvcalendar.WithHTTPClient(mockClient)),
	}

	ctx := context.Background()
	if err := b.SyncOnce(ctx); err == nil {
		t.Errorf("Expected SyncOnce to return error on network failure, got nil")
	}

	// Test Start loop with error
	ctxLoop, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()
	_ = b.Start(ctxLoop)
}

func TestIsImpactEligible(t *testing.T) {
	if !isImpactEligible(tvcalendar.ImpactHigh, tvcalendar.ImpactMedium) {
		t.Errorf("Expected High impact to be eligible when target is Medium")
	}
	if isImpactEligible(tvcalendar.ImpactLow, tvcalendar.ImpactHigh) {
		t.Errorf("Expected Low impact NOT to be eligible when target is High")
	}
	if !isImpactEligible(tvcalendar.ImpactHigh, tvcalendar.ImpactHigh) {
		t.Errorf("Expected High impact to be eligible when target is High")
	}
	if !isImpactEligible(tvcalendar.ImpactLow, tvcalendar.ImpactNone) {
		t.Errorf("Expected Low impact to be eligible when target is None")
	}
}
