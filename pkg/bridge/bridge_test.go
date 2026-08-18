package bridge

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/Nosvemos/tradingview-calendar-go/pkg/tvcalendar"
)

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
}
