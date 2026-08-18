package main

import (
	"bytes"
	"context"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/Nosvemos/tradingview-calendar-go/pkg/tvcalendar"
)

type roundTripFunc func(req *http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return f(req)
}

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

	// 3. Test Error Writer branches
	ew := &errWriter{}
	if err := writeCSV(ew, events); err == nil {
		t.Errorf("Expected writeCSV to fail on errWriter")
	}
	if err := writeJSON(ew, events); err == nil {
		t.Errorf("Expected writeJSON to fail on errWriter")
	}
}

type errWriter struct{}

func (e *errWriter) Write(p []byte) (n int, err error) {
	return 0, io.ErrClosedPipe
}

func TestFetchAndPrintLive(t *testing.T) {
	now := time.Now().UTC().Add(10 * time.Minute)
	mockJSON := `{
		"status": "ok",
		"result": [
			{
				"id": "1",
				"title": "US GDP",
				"country": "US",
				"currency": "USD",
				"actual": "3.0%",
				"forecast": "2.8%",
				"previous": "2.5%",
				"unit": "%",
				"importance": 1,
				"date": "` + now.Format(time.RFC3339) + `"
			},
			{
				"id": "2",
				"title": "EU Retail Sales",
				"country": "EU",
				"currency": "EUR",
				"actual": "1.0%",
				"forecast": "1.5%",
				"previous": "1.2%",
				"unit": "%",
				"importance": 0,
				"date": "` + now.Add(30*time.Minute).Format(time.RFC3339) + `"
			},
			{
				"id": "3",
				"title": "UK BRC Price",
				"country": "GB",
				"currency": "GBP",
				"actual": "0.5%",
				"forecast": "0.5%",
				"previous": "0.5%",
				"importance": -1,
				"date": "` + now.Add(time.Hour).Format(time.RFC3339) + `"
			}
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

	client := tvcalendar.NewClient(
		tvcalendar.WithHTTPClient(mockClient),
		tvcalendar.WithTimeLocation(time.UTC),
	)
	fetchAndPrintLive(client)

	// 2. Test live error path
	errClient := tvcalendar.NewClient(
		tvcalendar.WithHTTPClient(&http.Client{
			Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
				return nil, io.ErrClosedPipe
			}),
		}),
	)
	defer errClient.Close()
	fetchAndPrintLive(errClient)

	// 3. Test empty live results path
	emptyMockJSON := `{"status":"ok","result":[]}`
	emptyClient := tvcalendar.NewClient(
		tvcalendar.WithHTTPClient(&http.Client{
			Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
				return &http.Response{
					StatusCode: http.StatusOK,
					Body:       io.NopCloser(bytes.NewBufferString(emptyMockJSON)),
					Header:     make(http.Header),
				}, nil
			}),
		}),
		tvcalendar.WithTimeLocation(time.UTC),
	)
	defer emptyClient.Close()
	fetchAndPrintLive(emptyClient)
}

func TestExecuteDownloadFormats(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "cli_test_*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	mockJSON := `{"status":"ok","result":[{"id":"1","title":"NFP","country":"US","currency":"USD","importance":1,"date":"2025-01-10T13:30:00Z"}]}`
	http.DefaultClient.Transport = roundTripFunc(func(req *http.Request) (*http.Response, error) {
		return &http.Response{
			StatusCode: http.StatusOK,
			Body:       io.NopCloser(bytes.NewBufferString(mockJSON)),
			Header:     make(http.Header),
		}, nil
	})

	startFlag = "2025-01-01"
	endFlag = "2025-01-15"
	silentFlag = false
	concurrencyFlag = 1
	rateLimitFlag = 100
	timezoneFlag = "UTC"
	impactsFlag = "High"
	currenciesFlag = "USD"
	countriesFlag = "US"

	// 1. JSON
	jsonFile := filepath.Join(tempDir, "out.json")
	formatFlag = "json"
	outputFlag = jsonFile
	executeDownload()
	if _, err := os.Stat(jsonFile); err != nil {
		t.Errorf("JSON output file not created: %v", err)
	}

	// 2. CSV
	csvFile := filepath.Join(tempDir, "out.csv")
	formatFlag = "csv"
	outputFlag = csvFile
	executeDownload()
	if _, err := os.Stat(csvFile); err != nil {
		t.Errorf("CSV output file not created: %v", err)
	}

	// 3. Parquet
	parquetFile := filepath.Join(tempDir, "out.parquet")
	formatFlag = "parquet"
	outputFlag = parquetFile
	executeDownload()
	if _, err := os.Stat(parquetFile); err != nil {
		t.Errorf("Parquet output file not created: %v", err)
	}

	// 4. XLSX
	xlsxFile := filepath.Join(tempDir, "out.xlsx")
	formatFlag = "xlsx"
	outputFlag = xlsxFile
	executeDownload()
	if _, err := os.Stat(xlsxFile); err != nil {
		t.Errorf("XLSX output file not created: %v", err)
	}

	// Reset flags
	formatFlag = "json"
	outputFlag = ""
	startFlag = ""
	endFlag = ""
	timezoneFlag = ""
	impactsFlag = ""
	currenciesFlag = ""
	countriesFlag = ""
	silentFlag = true
}

func TestExecuteDbLoad(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "cli_db_*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	mockJSON := `{"status":"ok","result":[{"id":"1","title":"NFP","country":"US","currency":"USD","importance":1,"date":"2025-01-10T13:30:00Z"}]}`
	http.DefaultClient.Transport = roundTripFunc(func(req *http.Request) (*http.Response, error) {
		return &http.Response{
			StatusCode: http.StatusOK,
			Body:       io.NopCloser(bytes.NewBufferString(mockJSON)),
			Header:     make(http.Header),
		}, nil
	})

	dbFile := filepath.Join(tempDir, "test.db")
	startFlag = "2025-01-01"
	endFlag = "2025-01-15"
	dbFlag = dbFile
	silentFlag = false
	concurrencyFlag = 1
	timezoneFlag = "UTC"

	executeDbLoad()

	if _, err := os.Stat(dbFile); err != nil {
		t.Errorf("SQLite DB file not created by dbload: %v", err)
	}

	// Reset flags
	startFlag = ""
	endFlag = ""
	dbFlag = "tvcalendar.db"
	timezoneFlag = ""
	silentFlag = true
}

func TestExecuteLive(t *testing.T) {
	mockJSON := `{"status":"ok","result":[{"id":"1","title":"NFP","country":"US","currency":"USD","importance":1,"date":"2025-01-10T13:30:00Z"}]}`
	http.DefaultClient.Transport = roundTripFunc(func(req *http.Request) (*http.Response, error) {
		return &http.Response{
			StatusCode: http.StatusOK,
			Body:       io.NopCloser(bytes.NewBufferString(mockJSON)),
			Header:     make(http.Header),
		}, nil
	})

	intervalFlag = 0
	liveTimeLoc = "UTC"
	executeLive()
}

func TestExecuteBridgeWithContext(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "cli_bridge_*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	mockJSON := `{"status":"ok","result":[{"id":"1","title":"NFP","country":"US","currency":"USD","importance":1,"date":"2025-01-10T13:30:00Z"}]}`
	http.DefaultClient.Transport = roundTripFunc(func(req *http.Request) (*http.Response, error) {
		return &http.Response{
			StatusCode: http.StatusOK,
			Body:       io.NopCloser(bytes.NewBufferString(mockJSON)),
			Header:     make(http.Header),
		}, nil
	})

	mt4DirFlag = tempDir
	minImpactFlag = "Medium"
	currenciesFlag = "USD,EUR"
	bridgeIntervalFlag = 50 * time.Millisecond

	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	_ = executeBridgeWithContext(ctx)

	// Reset
	mt4DirFlag = ""
	minImpactFlag = "High"
	currenciesFlag = ""
}

func TestCreateServerFromFlags(t *testing.T) {
	portFlag = "9090"
	rateLimitFlag = 20
	concurrencyFlag = 4

	srv := createServerFromFlags()
	if srv == nil {
		t.Fatalf("Expected non-nil server instance")
	}

	portFlag = ":9091"
	srv2 := createServerFromFlags()
	if srv2 == nil {
		t.Fatalf("Expected non-nil server instance with colon port")
	}
}

func TestRootAndVersionCommands(t *testing.T) {
	if rootCmd.Use != "tvcalendar" {
		t.Errorf("Expected rootCmd use 'tvcalendar', got %s", rootCmd.Use)
	}
	if versionCmd.Use != "version" {
		t.Errorf("Expected versionCmd use 'version', got %s", versionCmd.Use)
	}
	versionCmd.Run(versionCmd, []string{})

	// Test liveCmd runner
	intervalFlag = 0
	liveTimeLoc = "UTC"
	liveCmd.Run(liveCmd, []string{})
}

func TestParseDatesAndLocation(t *testing.T) {
	// 1. Valid case
	start, end, loc, err := parseDatesAndLocation("2025-01-01", "2025-01-15", "UTC")
	if err != nil || start.IsZero() || end.IsZero() || loc == nil {
		t.Errorf("parseDatesAndLocation failed on valid inputs: %v", err)
	}

	// 2. Invalid start date
	if _, _, _, err := parseDatesAndLocation("invalid-date", "2025-01-15", "UTC"); err == nil {
		t.Errorf("Expected error on invalid start date, got nil")
	}

	// 3. Invalid end date
	if _, _, _, err := parseDatesAndLocation("2025-01-01", "invalid-date", "UTC"); err == nil {
		t.Errorf("Expected error on invalid end date, got nil")
	}

	// 4. Start date after end date
	if _, _, _, err := parseDatesAndLocation("2025-02-01", "2025-01-01", "UTC"); err == nil {
		t.Errorf("Expected error when start is after end date, got nil")
	}

	// 5. Invalid timezone
	if _, _, _, err := parseDatesAndLocation("2025-01-01", "2025-01-15", "Invalid/Timezone_Name"); err == nil {
		t.Errorf("Expected error on invalid timezone, got nil")
	}
}

func TestExportEventsPermutations(t *testing.T) {
	events := []tvcalendar.Event{
		{ID: "1", Title: "GDP", Currency: "USD"},
	}

	// 1. Parquet missing output
	if err := exportEvents("parquet", "", events); err == nil {
		t.Errorf("Expected error for parquet with empty output path")
	}

	// 2. XLSX missing output
	if err := exportEvents("xlsx", "", events); err == nil {
		t.Errorf("Expected error for xlsx with empty output path")
	}

	// 3. Unknown format
	if err := exportEvents("unknown_format", "", events); err == nil {
		t.Errorf("Expected error for unknown format")
	}

	// 4. JSON to stdout (no output path)
	if err := exportEvents("json", "", events); err != nil {
		t.Errorf("exportEvents JSON to stdout failed: %v", err)
	}

	// 5. CSV to stdout (no output path)
	if err := exportEvents("csv", "", events); err != nil {
		t.Errorf("exportEvents CSV to stdout failed: %v", err)
	}
}
