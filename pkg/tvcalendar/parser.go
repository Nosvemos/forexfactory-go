package tvcalendar

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"time"
)

type tradingViewResponse struct {
	Status string             `json:"status"`
	Result []tradingViewEvent `json:"result"`
}

type tradingViewEvent struct {
	ID            string    `json:"id"`
	Title         string    `json:"title"`
	Country       string    `json:"country"`
	Indicator     string    `json:"indicator"`
	Ticker        string    `json:"ticker"`
	Comment       string    `json:"comment"`
	Category      string    `json:"category"`
	Period        string    `json:"period"`
	ReferenceDate *string   `json:"referenceDate"`
	Source        string    `json:"source"`
	SourceURL     string    `json:"source_url"`
	Actual        *float64  `json:"actual"`
	Previous      *float64  `json:"previous"`
	Forecast      *float64  `json:"forecast"`
	Currency      string    `json:"currency"`
	Unit          string    `json:"unit"`
	Importance    int       `json:"importance"`
	Date          time.Time `json:"date"`
}

// ParseJSON parses a TradingView JSON response payload into a slice of standardized Events.
func ParseJSON(data []byte, loc *time.Location) ([]Event, error) {
	if len(data) == 0 {
		return nil, fmt.Errorf("empty json payload")
	}

	var resp tradingViewResponse
	if err := json.Unmarshal(data, &resp); err != nil {
		return nil, fmt.Errorf("failed to decode economic calendar JSON: %w", err)
	}

	events := make([]Event, 0, len(resp.Result))
	for _, raw := range resp.Result {
		eventDate := raw.Date.UTC()
		if loc != nil {
			eventDate = eventDate.In(loc)
		}

		var impact Impact
		switch raw.Importance {
		case 1:
			impact = ImpactHigh
		case 0:
			impact = ImpactMedium
		case -1:
			if strings.EqualFold(raw.Category, "gov") || strings.Contains(strings.ToLower(raw.Title), "holiday") || strings.Contains(strings.ToLower(raw.Title), "day") {
				impact = ImpactNone
			} else {
				impact = ImpactLow
			}
		default:
			impact = ImpactLow
		}

		isAllDay := eventDate.Hour() == 0 && eventDate.Minute() == 0 && eventDate.Second() == 0 && raw.Actual == nil && raw.Forecast == nil

		unit := strings.TrimSpace(raw.Unit)
		events = append(events, Event{
			ID:          raw.ID,
			Title:       strings.TrimSpace(raw.Title),
			Country:     strings.ToUpper(strings.TrimSpace(raw.Country)),
			Currency:    strings.ToUpper(strings.TrimSpace(raw.Currency)),
			Date:        eventDate,
			Impact:      impact,
			Forecast:    formatNumeric(raw.Forecast, unit),
			Previous:    formatNumeric(raw.Previous, unit),
			Actual:      formatNumeric(raw.Actual, unit),
			Unit:        unit,
			Indicator:   strings.TrimSpace(raw.Indicator),
			Category:    strings.TrimSpace(raw.Category),
			Period:      strings.TrimSpace(raw.Period),
			Comment:     strings.TrimSpace(raw.Comment),
			Source:      strings.TrimSpace(raw.Source),
			SourceURL:   strings.TrimSpace(raw.SourceURL),
			Ticker:      strings.TrimSpace(raw.Ticker),
			IsAllDay:    isAllDay,
			IsTentative: false,
		})
	}

	return events, nil
}

func formatNumeric(val *float64, unit string) string {
	if val == nil {
		return ""
	}
	formatted := strconv.FormatFloat(*val, 'f', -1, 64)
	if unit != "" && unit != "Index" && unit != "points" && unit != "pts" {
		return formatted + unit
	}
	return formatted
}
