package forexcalendar

import (
	"testing"
	"time"
)

func TestParseFloat(t *testing.T) {
	tests := []struct {
		input    string
		expected float64
		hasError bool
	}{
		{"12.5%", 12.5, false},
		{"-0.4%", -0.4, false},
		{"\u22120.4%", -0.4, false}, // Unicode minus sign
		{"120K", 120000.0, false},
		{"1.5M", 1500000.0, false},
		{"2.4B", 2400000000.0, false},
		{"1.2T", 1200000000000.0, false},
		{"1,500.50", 1500.50, false},
		{"$10.5", 10.5, false},
		{"€20.0M", 20000000.0, false},
		{"<0.1%", 0.1, false},
		{"3.5% (revised)", 3.5, false},
		{"", 0, true},
		{"-", 0, true},
		{"--", 0, true},
		{"N/A", 0, true},
	}

	for _, tt := range tests {
		got, err := ParseFloat(tt.input)
		if (err != nil) != tt.hasError {
			t.Errorf("ParseFloat(%q) error = %v, wantErr %v", tt.input, err, tt.hasError)
			continue
		}
		if !tt.hasError && got != tt.expected {
			t.Errorf("ParseFloat(%q) = %v, want %v", tt.input, got, tt.expected)
		}
	}
}

func TestEventDeviationAndSurpriseAndBias(t *testing.T) {
	e := Event{
		Title:    "CPI YoY",
		Actual:   "3.2%",
		Forecast: "3.0%",
		Previous: "2.8%",
	}

	dev, err := e.Deviation()
	if err != nil || dev < 0.19 || dev > 0.21 {
		t.Errorf("Deviation() = %v, want 0.2", dev)
	}

	surp, err := e.Surprise()
	if err != nil || surp < 6.6 || surp > 6.7 {
		t.Errorf("Surprise() = %v, want ~6.66", surp)
	}

	if bias := e.MarketBias(); bias != "Bullish" {
		t.Errorf("MarketBias() = %q, want 'Bullish'", bias)
	}

	unemp := Event{
		Title:    "Unemployment Rate",
		Actual:   "4.2%",
		Forecast: "4.0%",
		Previous: "3.9%",
	}
	if bias := unemp.MarketBias(); bias != "Bearish" {
		t.Errorf("MarketBias(unemployment) = %q, want 'Bearish'", bias)
	}
}

func TestEventDateHelpers(t *testing.T) {
	now := time.Now().UTC()
	e := Event{
		ID:       "12345",
		Title:    "Fed Interest Rate Decision",
		Country:  "US",
		Currency: "USD",
		Date:     now,
		Impact:   ImpactHigh,
		Actual:   "5.50%",
		Forecast: "5.50%",
		Previous: "5.25%",
	}

	act, err := e.ParseActual()
	if err != nil || act != 5.5 {
		t.Errorf("ParseActual() = %v, want 5.5", act)
	}

	fc, err := e.ParseForecast()
	if err != nil || fc != 5.5 {
		t.Errorf("ParseForecast() = %v, want 5.5", fc)
	}

	prev, err := e.ParsePrevious()
	if err != nil || prev != 5.25 {
		t.Errorf("ParsePrevious() = %v, want 5.25", prev)
	}
}
