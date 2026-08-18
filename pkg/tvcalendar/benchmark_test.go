package tvcalendar

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func BenchmarkParseJSON(b *testing.B) {
	mockPayload := []byte(`{
		"status": "ok",
		"result": [
			{
				"id": "371946",
				"title": "Monthly CPI Indicator",
				"country": "AU",
				"indicator": "Monthly CPI Indicator",
				"ticker": "ECONOMICS:AUMCPI",
				"category": "prce",
				"period": "Nov",
				"source": "Bureau of Statistics",
				"source_url": "https://www.abs.gov.au/",
				"actual": 2.3,
				"previous": 2.1,
				"forecast": 2.2,
				"currency": "AUD",
				"unit": "%",
				"importance": 1,
				"date": "2025-01-08T00:30:00Z"
			},
			{
				"id": "371947",
				"title": "US Non Farm Payrolls",
				"country": "US",
				"indicator": "Employment",
				"ticker": "ECONOMICS:USNFP",
				"category": "labr",
				"period": "Jan",
				"source": "BLS",
				"actual": 216.0,
				"previous": 173.0,
				"forecast": 170.0,
				"currency": "USD",
				"unit": "K",
				"importance": 1,
				"date": "2025-01-10T13:30:00Z"
			}
		]
	}`)

	loc := time.UTC
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := ParseJSON(mockPayload, loc)
		if err != nil {
			b.Fatalf("ParseJSON failed: %v", err)
		}
	}
}

func BenchmarkParseFloat(b *testing.B) {
	testInputs := []string{
		"5.25%",
		"-3.4M",
		"−12.8B",
		"100.5K",
		"$1,250.75",
		"1.25e-3",
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		input := testInputs[i%len(testInputs)]
		_, _ = ParseFloat(input)
	}
}

func BenchmarkDeviationAndSurprise(b *testing.B) {
	e := Event{
		Actual:   "3.2%",
		Forecast: "3.0%",
		Previous: "2.8%",
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = e.Deviation()
		_, _ = e.Surprise()
	}
}

func BenchmarkMarketBias(b *testing.B) {
	e := Event{
		Title:    "US Non Farm Payrolls",
		Actual:   "250K",
		Forecast: "200K",
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = e.MarketBias()
	}
}

func BenchmarkWriteParquet(b *testing.B) {
	events := make([]Event, 100)
	now := time.Date(2025, 1, 15, 12, 0, 0, 0, time.UTC)
	for i := 0; i < 100; i++ {
		events[i] = Event{
			ID:       fmt.Sprintf("evt-%d", i),
			Title:    "Consumer Price Index",
			Country:  "US",
			Currency: "USD",
			Date:     now,
			Impact:   ImpactHigh,
			Actual:   "3.1%",
			Forecast: "3.0%",
			Previous: "2.9%",
			Unit:     "%",
		}
	}

	tempDir, err := os.MkdirTemp("", "bench_parquet_*")
	if err != nil {
		b.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	outFile := filepath.Join(tempDir, "bench.parquet")

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if err := WriteParquet(outFile, events); err != nil {
			b.Fatalf("WriteParquet failed: %v", err)
		}
	}
}
