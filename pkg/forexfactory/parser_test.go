package forexfactory

import (
	"bytes"
	"testing"
	"time"
)

func TestParseXML(t *testing.T) {
	mockXML := `<?xml version="1.0" encoding="utf-8"?>
<weeklyevents>
	<event>
		<title>Core Durable Goods Orders m/m</title>
		<country>USD</country>
		<date>05-26-2026</date>
		<time>8:30am</time>
		<impact>High</impact>
		<forecast>0.1%</forecast>
		<previous>0.2%</previous>
		<actual>0.3%</actual>
	</event>
	<event>
		<title>French Bank Holiday</title>
		<country>EUR</country>
		<date>05-26-2026</date>
		<time>All Day</time>
		<impact>Holiday</impact>
		<forecast></forecast>
		<previous></previous>
		<actual></actual>
	</event>
</weeklyevents>`

	events, err := ParseXML([]byte(mockXML), time.UTC)
	if err != nil {
		t.Fatalf("Failed to parse mock XML: %v", err)
	}

	if len(events) != 2 {
		t.Fatalf("Expected 2 events, got %d", len(events))
	}

	// Verify Event 1
	e1 := events[0]
	if e1.Title != "Core Durable Goods Orders m/m" {
		t.Errorf("Expected Title 'Core Durable Goods Orders m/m', got '%s'", e1.Title)
	}
	if e1.Country != "USD" {
		t.Errorf("Expected Country 'USD', got '%s'", e1.Country)
	}
	if e1.Impact != ImpactHigh {
		t.Errorf("Expected ImpactHigh, got '%s'", e1.Impact)
	}
	if e1.Forecast != "0.1%" || e1.Previous != "0.2%" || e1.Actual != "0.3%" {
		t.Errorf("Values mismatch: %s / %s / %s", e1.Forecast, e1.Previous, e1.Actual)
	}
	expectedTime := time.Date(2026, time.May, 26, 8, 30, 0, 0, time.UTC)
	if !e1.Date.Equal(expectedTime) {
		t.Errorf("Expected Date %v, got %v", expectedTime, e1.Date)
	}
	if e1.IsAllDay {
		t.Error("Expected Event 1 not to be All Day")
	}

	// Verify Event 2
	e2 := events[1]
	if e2.Title != "French Bank Holiday" {
		t.Errorf("Expected Title 'French Bank Holiday', got '%s'", e2.Title)
	}
	if e2.Country != "EUR" {
		t.Errorf("Expected Country 'EUR', got '%s'", e2.Country)
	}
	if e2.Impact != ImpactNone {
		t.Errorf("Expected ImpactNone, got '%s'", e2.Impact)
	}
	if !e2.IsAllDay {
		t.Error("Expected Event 2 to be All Day")
	}
}

func TestParseHTML(t *testing.T) {
	mockHTML := `<html>
<body>
	<table class="calendar__table">
		<tr class="calendar__row">
			<td class="calendar__cell calendar__date">Mon May 25</td>
			<td class="calendar__cell calendar__time">8:30am</td>
			<td class="calendar__cell calendar__currency">USD</td>
			<td class="calendar__cell calendar__impact"><span class="icon--impact-red">High</span></td>
			<td class="calendar__cell calendar__event"><a href="calendar.php?show=123456">CB Consumer Confidence</a></td>
			<td class="calendar__cell calendar__actual">102.5</td>
			<td class="calendar__cell calendar__forecast">101.0</td>
			<td class="calendar__cell calendar__previous">100.2</td>
		</tr>
		<tr class="calendar__row">
			<td class="calendar__cell calendar__date"></td>
			<td class="calendar__cell calendar__time"></td>
			<td class="calendar__cell calendar__currency">EUR</td>
			<td class="calendar__cell calendar__impact"><span class="icon--impact-orange">Medium</span></td>
			<td class="calendar__cell calendar__event"><a href="/789012">German Ifo Business Climate</a></td>
			<td class="calendar__cell calendar__actual">89.3</td>
			<td class="calendar__cell calendar__forecast">89.0</td>
			<td class="calendar__cell calendar__previous">88.5</td>
		</tr>
	</table>
</body>
</html>`

	r := bytes.NewReader([]byte(mockHTML))
	events, err := ParseHTML(r, 2026, time.UTC)
	if err != nil {
		t.Fatalf("Failed to parse mock HTML: %v", err)
	}

	if len(events) != 2 {
		t.Fatalf("Expected 2 events, got %d", len(events))
	}

	// Verify Event 1
	e1 := events[0]
	if e1.ID != "123456" {
		t.Errorf("Expected ID '123456', got '%s'", e1.ID)
	}
	if e1.Title != "CB Consumer Confidence" {
		t.Errorf("Expected Title 'CB Consumer Confidence', got '%s'", e1.Title)
	}
	if e1.Country != "USD" {
		t.Errorf("Expected Country 'USD', got '%s'", e1.Country)
	}
	if e1.Impact != ImpactHigh {
		t.Errorf("Expected ImpactHigh, got '%s'", e1.Impact)
	}
	expectedTime := time.Date(2026, time.May, 25, 8, 30, 0, 0, time.UTC)
	if !e1.Date.Equal(expectedTime) {
		t.Errorf("Expected Date %v, got %v", expectedTime, e1.Date)
	}

	// Verify Event 2 (inherits Date and Time from Event 1)
	e2 := events[1]
	if e2.ID != "789012" {
		t.Errorf("Expected ID '789012', got '%s'", e2.ID)
	}
	if e2.Title != "German Ifo Business Climate" {
		t.Errorf("Expected Title 'German Ifo Business Climate', got '%s'", e2.Title)
	}
	if e2.Country != "EUR" {
		t.Errorf("Expected Country 'EUR', got '%s'", e2.Country)
	}
	if e2.Impact != ImpactMedium {
		t.Errorf("Expected ImpactMedium, got '%s'", e2.Impact)
	}
	if !e2.Date.Equal(expectedTime) {
		t.Errorf("Expected Date %v to be inherited from row 1, got %v", expectedTime, e2.Date)
	}
}
