package forexfactory

import (
	"os"
	"testing"
	"time"

	"github.com/xuri/excelize/v2"
)

func TestWriteExcel(t *testing.T) {
	testFile := "test_export.xlsx"
	defer os.Remove(testFile)

	events := []Event{
		{
			ID:       "1001",
			Title:    "CPI m/m",
			Currency: "USD",
			Date:     time.Date(2026, time.May, 25, 12, 30, 0, 0, time.UTC),
			Impact:   ImpactHigh,
			Forecast: "0.3%",
			Previous: "0.2%",
			Actual:   "0.4%",
		},
		{
			ID:       "1002",
			Title:    "Retail Sales m/m",
			Currency: "EUR",
			Date:     time.Date(2026, time.May, 25, 9, 0, 0, 0, time.UTC),
			Impact:   ImpactMedium,
			Forecast: "0.5%",
			Previous: "0.1%",
			Actual:   "0.2%",
		},
	}

	err := WriteExcel(events, testFile)
	if err != nil {
		t.Fatalf("WriteExcel failed: %v", err)
	}

	// Verify the file was generated and can be opened with excelize
	f, err := excelize.OpenFile(testFile)
	if err != nil {
		t.Fatalf("Failed to open generated excel file: %v", err)
	}
	defer f.Close()

	rows, err := f.GetRows("Economic Calendar")
	if err != nil {
		t.Fatalf("Failed to read sheet rows: %v", err)
	}

	// Header + 2 rows = 3 rows
	if len(rows) != 3 {
		t.Errorf("Expected 3 rows in excel file, got %d", len(rows))
	}

	if rows[0][0] != "ID" || rows[0][1] != "Title" {
		t.Errorf("Header mismatch: %v", rows[0])
	}
}
