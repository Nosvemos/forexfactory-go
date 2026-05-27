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
		{"", 0, true},
		{"-", 0, true},
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
