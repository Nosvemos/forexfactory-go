package forexfactory

import (
	"fmt"
	"strconv"
	"strings"
	"time"
)

// Impact represents the market impact level of an economic event.
type Impact string

const (
	// ImpactHigh indicates a major market-moving event (e.g. Interest Rate Decision).
	ImpactHigh Impact = "High"
	// ImpactMedium indicates a moderate market-moving event.
	ImpactMedium Impact = "Medium"
	// ImpactLow indicates a low market-moving event.
	ImpactLow Impact = "Low"
	// ImpactNone indicates a non-economic event or holiday.
	ImpactNone Impact = "None"
)

// Event represents an economic calendar event scraped from Forex Factory.
type Event struct {
	// ID is the unique identifier for the event details page (parsed from detail URL).
	ID string `json:"id,omitempty"`
	// Title is the name of the economic news event (e.g., "CPI m/m").
	Title string `json:"title"`
	// Currency is the currency code affected (e.g., "USD", "EUR", "GBP").
	Currency string `json:"currency"`
	// Date represents the start time of the event.
	Date time.Time `json:"date"`
	// Impact is the volatility threat classification.
	Impact Impact `json:"impact"`
	// Forecast is the analyst consensus estimate.
	Forecast string `json:"forecast"`
	// Previous is the value of the previous release (possibly revised).
	Previous string `json:"previous"`
	// Actual is the final published value.
	Actual string `json:"actual"`
	// IsAllDay is true if the event has no fixed hour and lasts the whole day (e.g., Bank Holiday).
	IsAllDay bool `json:"is_all_day"`
	// IsTentative is true if the event time is not finalized (e.g. Tentative OPEC meetings).
	IsTentative bool `json:"is_tentative"`
}

// ParseActual returns the Actual value parsed into a float64.
// It handles negative numbers, thousands/millions/billions multipliers (K, M, B), and strips percent/comma symbols.
func (e Event) ParseActual() (float64, error) {
	return ParseFloat(e.Actual)
}

// ParseForecast returns the Forecast value parsed into a float64.
// It handles negative numbers, thousands/millions/billions multipliers (K, M, B), and strips percent/comma symbols.
func (e Event) ParseForecast() (float64, error) {
	return ParseFloat(e.Forecast)
}

// ParsePrevious returns the Previous value parsed into a float64.
// It handles negative numbers, thousands/millions/billions multipliers (K, M, B), and strips percent/comma symbols.
func (e Event) ParsePrevious() (float64, error) {
	return ParseFloat(e.Previous)
}

// ParseFloat is a utility function that parses standard economic calendar values (e.g., "-0.5%", "120K", "1.2M", "1,500.50")
// into a standardized float64 value.
func ParseFloat(val string) (float64, error) {
	val = strings.TrimSpace(val)
	if val == "" || val == "-" {
		return 0, fmt.Errorf("value is empty or nil")
	}

	// Normalize characters: strip commas, currency signs
	val = strings.ReplaceAll(val, ",", "")
	val = strings.ReplaceAll(val, "$", "")
	val = strings.ReplaceAll(val, " ", "")

	var multiplier float64 = 1.0
	lowerVal := strings.ToLower(val)

	if strings.HasSuffix(lowerVal, "%") {
		// Strip percent symbol but keep scale as 1.0 (standard for calendar stats)
		val = val[:len(val)-1]
	} else if strings.HasSuffix(lowerVal, "k") {
		multiplier = 1000.0
		val = val[:len(val)-1]
	} else if strings.HasSuffix(lowerVal, "m") {
		multiplier = 1000000.0
		val = val[:len(val)-1]
	} else if strings.HasSuffix(lowerVal, "b") {
		multiplier = 1000000000.0
		val = val[:len(val)-1]
	} else if strings.HasSuffix(lowerVal, "t") {
		multiplier = 1000000000000.0
		val = val[:len(val)-1]
	}

	parsed, err := strconv.ParseFloat(val, 64)
	if err != nil {
		return 0, fmt.Errorf("failed to parse float from %q: %w", val, err)
	}

	return parsed * multiplier, nil
}
