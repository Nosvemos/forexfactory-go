package forexcalendar

import (
	"fmt"
	"strconv"
	"strings"
	"time"
)

// Impact represents the market impact level of an economic event.
type Impact string

const (
	// ImpactHigh indicates a major market-moving event (e.g. Interest Rate Decision, CPI, NFP).
	ImpactHigh Impact = "High"
	// ImpactMedium indicates a moderate market-moving event (e.g. PMI, Retail Sales).
	ImpactMedium Impact = "Medium"
	// ImpactLow indicates a low market-moving event.
	ImpactLow Impact = "Low"
	// ImpactNone indicates a non-economic event or holiday.
	ImpactNone Impact = "None"
)

// Event represents an economic calendar event.
type Event struct {
	// ID is the unique identifier for the event.
	ID string `json:"id,omitempty"`
	// Title is the name of the economic news event (e.g., "CPI YoY", "Non-Farm Payrolls").
	Title string `json:"title"`
	// Country is the ISO country code (e.g., "US", "EU", "GB", "JP", "AU", "DE", "TR").
	Country string `json:"country,omitempty"`
	// Currency is the currency code affected (e.g., "USD", "EUR", "GBP", "JPY", "TRY").
	Currency string `json:"currency"`
	// Date represents the exact scheduled release timestamp in UTC.
	Date time.Time `json:"date"`
	// Impact is the volatility classification (High, Medium, Low, None).
	Impact Impact `json:"impact"`
	// Forecast is the analyst consensus estimate formatted as a string (e.g., "2.5%").
	Forecast string `json:"forecast"`
	// Previous is the value of the previous release (e.g., "2.3%").
	Previous string `json:"previous"`
	// Actual is the final published value (e.g., "2.6%").
	Actual string `json:"actual"`
	// Unit is the measurement unit (e.g. "%", "K", "M", "B", "Index").
	Unit string `json:"unit,omitempty"`
	// Indicator is the standardized economic indicator name.
	Indicator string `json:"indicator,omitempty"`
	// Category is the macroeconomic category code (e.g., "prce", "cnsm", "labr", "gdp").
	Category string `json:"category,omitempty"`
	// Period is the reference time period (e.g. "Nov", "Q3").
	Period string `json:"period,omitempty"`
	// Comment provides detailed macroeconomic context and definition.
	Comment string `json:"comment,omitempty"`
	// Source is the reporting organization or government agency.
	Source string `json:"source,omitempty"`
	// SourceURL is the official link to the reporting agency.
	SourceURL string `json:"source_url,omitempty"`
	// Ticker is the financial ticker symbol if available.
	Ticker string `json:"ticker,omitempty"`
	// IsAllDay is true if the event has no fixed hour and lasts the whole day (e.g., Bank Holiday).
	IsAllDay bool `json:"is_all_day"`
	// IsTentative is true if the event time is not finalized.
	IsTentative bool `json:"is_tentative"`
}

// ParseActual returns the Actual value parsed into a float64.
func (e Event) ParseActual() (float64, error) {
	return ParseFloat(e.Actual)
}

// ParseForecast returns the Forecast value parsed into a float64.
func (e Event) ParseForecast() (float64, error) {
	return ParseFloat(e.Forecast)
}

// ParsePrevious returns the Previous value parsed into a float64.
func (e Event) ParsePrevious() (float64, error) {
	return ParseFloat(e.Previous)
}

// Deviation returns (Actual - Forecast) as a float64.
func (e Event) Deviation() (float64, error) {
	actual, err := e.ParseActual()
	if err != nil {
		return 0, fmt.Errorf("cannot calculate deviation: %w", err)
	}
	forecast, err := e.ParseForecast()
	if err != nil {
		return 0, fmt.Errorf("cannot calculate deviation: %w", err)
	}
	return actual - forecast, nil
}

// Surprise returns the percentage surprise ((Actual - Forecast) / |Forecast| * 100).
func (e Event) Surprise() (float64, error) {
	actual, err := e.ParseActual()
	if err != nil {
		return 0, err
	}
	forecast, err := e.ParseForecast()
	if err != nil {
		return 0, err
	}
	if forecast == 0 {
		return actual, nil
	}
	absForecast := forecast
	if absForecast < 0 {
		absForecast = -absForecast
	}
	return ((actual - forecast) / absForecast) * 100.0, nil
}

// MarketBias returns "Bullish", "Bearish", or "Neutral" based on whether Actual outperformed Forecast.
func (e Event) MarketBias() string {
	dev, err := e.Deviation()
	if err != nil || dev == 0 {
		return "Neutral"
	}

	titleLower := strings.ToLower(e.Title)
	isInverted := strings.Contains(titleLower, "unemployment") ||
		strings.Contains(titleLower, "jobless") ||
		strings.Contains(titleLower, "claims")

	if isInverted {
		if dev < 0 {
			return "Bullish"
		}
		return "Bearish"
	}

	if dev > 0 {
		return "Bullish"
	}
	return "Bearish"
}

// ParseFloat is a utility function that parses standard economic calendar values (e.g., "-0.5%", "120K", "1.2M", "1,500.50", "−0.4%", "<0.1%", "€10.5M")
// into a standardized float64 value.
func ParseFloat(val string) (float64, error) {
	val = strings.TrimSpace(val)
	if val == "" || val == "-" || val == "--" || val == "---" || strings.EqualFold(val, "n/a") || strings.EqualFold(val, "nc") || strings.EqualFold(val, "none") || strings.EqualFold(val, "null") {
		return 0, fmt.Errorf("value is empty or placeholder (%q)", val)
	}

	// Remove non-breaking spaces and all unicode spaces
	val = strings.Map(func(r rune) rune {
		if r == '\u00a0' || r == '\u200b' || r == '\u3000' || r == '\t' || r == ' ' {
			return -1
		}
		return r
	}, val)

	// Normalize unicode minus signs and dashes to standard ASCII minus
	val = strings.ReplaceAll(val, "\u2212", "-") // Mathematical minus
	val = strings.ReplaceAll(val, "\u2013", "-") // En dash
	val = strings.ReplaceAll(val, "\u2014", "-") // Em dash

	// Strip commas, currency symbols, and inequality signs
	stripChars := []string{",", "$", "€", "£", "¥", "₹", "元", "<", ">", "≤", "≥", "~"}
	for _, sc := range stripChars {
		val = strings.ReplaceAll(val, sc, "")
	}

	// Strip parenthetical notes like (revised) or (preliminary)
	if idx := strings.Index(val, "("); idx != -1 {
		val = strings.TrimSpace(val[:idx])
	}

	if val == "" || val == "-" {
		return 0, fmt.Errorf("value has no numeric content")
	}

	var multiplier float64 = 1.0
	lowerVal := strings.ToLower(val)

	if strings.HasSuffix(lowerVal, "%") {
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
