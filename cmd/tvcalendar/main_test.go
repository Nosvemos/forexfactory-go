package main

import (
	"bytes"
	"strings"
	"testing"
	"time"

	"github.com/Nosvemos/tradingview-calendar-go/pkg/tvcalendar"
)

func TestParseFilters(t *testing.T) {
	impactsFlag = "High,medium,LOW,none"
	currenciesFlag = "usd, eur,gbp "
	countriesFlag = "us, de, fr"

	impacts, currs, countries := parseFilters()

	if len(impacts) != 4 {
		t.Errorf("Expected 4 impacts, got %d", len(impacts))
	}
	if len(currs) != 3 || currs[0] != "USD" || currs[1] != "EUR" || currs[2] != "GBP" {
		t.Errorf("Unexpected parsed currencies: %v", currs)
	}
	if len(countries) != 3 || countries[0] != "US" || countries[1] != "DE" || countries[2] != "FR" {
		t.Errorf("Unexpected parsed countries: %v", countries)
	}

	// Reset flags
	impactsFlag = ""
	currenciesFlag = ""
	countriesFlag = ""
}

func TestWriteCSVAndJSON(t *testing.T) {
	events := []tvcalendar.Event{
		{
			ID:       "101",
			Title:    "US CPI",
			Country:  "US",
			Currency: "USD",
			Date:     time.Date(2025, 1, 15, 13, 30, 0, 0, time.UTC),
			Impact:   tvcalendar.ImpactHigh,
			Actual:   "3.2%",
			Forecast: "3.0%",
			Previous: "2.9%",
			Unit:     "%",
		},
	}

	// 1. Test CSV
	var csvBuf bytes.Buffer
	if err := writeCSV(&csvBuf, events); err != nil {
		t.Fatalf("writeCSV failed: %v", err)
	}
	csvOut := csvBuf.String()
	if !strings.Contains(csvOut, "US CPI") || !strings.Contains(csvOut, "3.2%") {
		t.Errorf("CSV output missing expected fields: %s", csvOut)
	}

	// 2. Test JSON
	var jsonBuf bytes.Buffer
	if err := writeJSON(&jsonBuf, events); err != nil {
		t.Fatalf("writeJSON failed: %v", err)
	}
	jsonOut := jsonBuf.String()
	if !strings.Contains(jsonOut, "\"title\": \"US CPI\"") {
		t.Errorf("JSON output missing expected title: %s", jsonOut)
	}
}

func TestRootAndVersionCommands(t *testing.T) {
	if rootCmd.Use != "tvcalendar" {
		t.Errorf("Expected rootCmd use 'tvcalendar', got %s", rootCmd.Use)
	}
	if versionCmd.Use != "version" {
		t.Errorf("Expected versionCmd use 'version', got %s", versionCmd.Use)
	}
}
