package tvcalendar

import (
	"testing"
	"time"
)

func TestParseJSON(t *testing.T) {
	mockPayload := []byte(`{
		"status": "ok",
		"result": [
			{
				"id": "371946",
				"title": "Monthly CPI Indicator",
				"country": "AU",
				"indicator": "Monthly CPI Indicator",
				"ticker": "ECONOMICS:AUMCPI",
				"comment": "Australian CPI monthly indicator.",
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
				"id": "366959",
				"title": "New Year’s Day",
				"country": "US",
				"indicator": "Holidays",
				"category": "gov",
				"period": "",
				"actual": null,
				"previous": null,
				"forecast": null,
				"currency": "USD",
				"importance": -1,
				"date": "2025-01-01T00:00:00Z"
			}
		]
	}`)

	events, err := ParseJSON(mockPayload, time.UTC)
	if err != nil {
		t.Fatalf("ParseJSON failed: %v", err)
	}

	if len(events) != 2 {
		t.Fatalf("Expected 2 events, got %d", len(events))
	}

	cpi := events[0]
	if cpi.ID != "371946" || cpi.Title != "Monthly CPI Indicator" || cpi.Currency != "AUD" || cpi.Country != "AU" {
		t.Errorf("Mismatch in parsed CPI: %+v", cpi)
	}
	if cpi.Impact != ImpactHigh {
		t.Errorf("Expected ImpactHigh, got %v", cpi.Impact)
	}
	if cpi.Actual != "2.3%" || cpi.Forecast != "2.2%" || cpi.Previous != "2.1%" {
		t.Errorf("Mismatch in formatted values: Actual=%q Forecast=%q Prev=%q", cpi.Actual, cpi.Forecast, cpi.Previous)
	}
	if cpi.Ticker != "ECONOMICS:AUMCPI" {
		t.Errorf("Mismatch in ticker: %q", cpi.Ticker)
	}

	holiday := events[1]
	if holiday.Impact != ImpactNone {
		t.Errorf("Expected ImpactNone for holiday, got %v", holiday.Impact)
	}
	if !holiday.IsAllDay {
		t.Errorf("Expected IsAllDay=true for holiday, got false")
	}
}

func TestParseJSONEmpty(t *testing.T) {
	_, err := ParseJSON([]byte(""), nil)
	if err == nil {
		t.Errorf("Expected error on empty payload, got nil")
	}
}
