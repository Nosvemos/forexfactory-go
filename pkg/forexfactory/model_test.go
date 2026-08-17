package forexfactory

import (
	"math"
	"testing"
)

func TestParseFloat(t *testing.T) {
	tests := []struct {
		input    string
		expected float64
		hasError bool
	}{
		{"0.1%", 0.1, false},
		{"-0.5%", -0.5, false},
		{"120K", 120000.0, false},
		{"-15.5k", -15500.0, false},
		{"1.2M", 1200000.0, false},
		{"-0.8m", -800000.0, false},
		{"15B", 15000000000.0, false},
		{"-2.5b", -2500000000.0, false},
		{"1.5T", 1500000000000.0, false},
		{"1,500.50", 1500.50, false},
		{"$10.5M", 10500000.0, false},
		{"€5.2B", 5200000000.0, false},
		{"£150M", 150000000.0, false},
		{"¥20.5T", 20500000000000.0, false},
		{"−0.4%", -0.4, false},       // Unicode minus \u2212
		{"–1.5M", -1500000.0, false},   // En dash \u2013
		{"<0.1%", 0.1, false},         // Inequality <
		{">50.0", 50.0, false},        // Inequality >
		{"\u00a0120K\u00a0", 120000.0, false}, // Non-breaking space
		{"1.2% (revised)", 1.2, false}, // Parenthetical note
		{"", 0, true},
		{"-", 0, true},
		{"--", 0, true},
		{"---", 0, true},
		{"N/A", 0, true},
		{"NC", 0, true},
		{"None", 0, true},
		{"invalid", 0, true},
	}

	for _, tt := range tests {
		result, err := ParseFloat(tt.input)
		if tt.hasError {
			if err == nil {
				t.Errorf("Expected error for input %q, but got nil", tt.input)
			}
		} else {
			if err != nil {
				t.Errorf("Unexpected error for input %q: %v", tt.input, err)
			}
			if math.Abs(result-tt.expected) > 1e-9 {
				t.Errorf("For input %q, expected %f, got %f", tt.input, tt.expected, result)
			}
		}
	}
}

func TestEventParseHelpers(t *testing.T) {
	e := Event{
		Actual:   "120K",
		Forecast: "100K",
		Previous: "90K",
	}

	actual, err := e.ParseActual()
	if err != nil || actual != 120000.0 {
		t.Errorf("ParseActual failed: got %f, err: %v", actual, err)
	}

	forecast, err := e.ParseForecast()
	if err != nil || forecast != 100000.0 {
		t.Errorf("ParseForecast failed: got %f, err: %v", forecast, err)
	}

	previous, err := e.ParsePrevious()
	if err != nil || previous != 90000.0 {
		t.Errorf("ParsePrevious failed: got %f, err: %v", previous, err)
	}
}

func TestEventDeviationAndSurpriseAndBias(t *testing.T) {
	// Standard positive indicator (e.g. GDP, Retail Sales, NFP)
	e1 := Event{
		Title:    "Retail Sales m/m",
		Actual:   "0.8%",
		Forecast: "0.5%",
	}
	dev1, err := e1.Deviation()
	if err != nil || math.Abs(dev1-0.3) > 1e-9 {
		t.Errorf("Expected deviation 0.3, got %f (err: %v)", dev1, err)
	}
	surprise1, err := e1.Surprise()
	if err != nil || math.Abs(surprise1-60.0) > 1e-9 {
		t.Errorf("Expected surprise 60%%, got %f (err: %v)", surprise1, err)
	}
	if bias1 := e1.MarketBias(); bias1 != "Bullish" {
		t.Errorf("Expected Bullish bias for positive retail sales beat, got %s", bias1)
	}

	// Inverted indicator (e.g. Unemployment Rate, Jobless Claims)
	e2 := Event{
		Title:    "Unemployment Rate",
		Actual:   "3.8%",
		Forecast: "4.0%",
	}
	dev2, _ := e2.Deviation()
	if math.Abs(dev2-(-0.2)) > 1e-9 {
		t.Errorf("Expected deviation -0.2, got %f", dev2)
	}
	// Lower unemployment is Bullish
	if bias2 := e2.MarketBias(); bias2 != "Bullish" {
		t.Errorf("Expected Bullish bias for lower unemployment, got %s", bias2)
	}

	// Higher unemployment is Bearish
	e3 := Event{
		Title:    "Unemployment Claims",
		Actual:   "240K",
		Forecast: "220K",
	}
	if bias3 := e3.MarketBias(); bias3 != "Bearish" {
		t.Errorf("Expected Bearish bias for higher unemployment claims, got %s", bias3)
	}
}
