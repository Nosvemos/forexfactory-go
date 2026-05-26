package forexfactory

import "time"

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
	// Country is the currency code affected (e.g., "USD", "EUR", "GBP").
	Country string `json:"country"`
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
