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
	if e1.Currency != "USD" {
		t.Errorf("Expected Currency 'USD', got '%s'", e1.Currency)
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
	if e2.Currency != "EUR" {
		t.Errorf("Expected Currency 'EUR', got '%s'", e2.Currency)
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
	if e1.Currency != "USD" {
		t.Errorf("Expected Currency 'USD', got '%s'", e1.Currency)
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
	if e2.Currency != "EUR" {
		t.Errorf("Expected Currency 'EUR', got '%s'", e2.Currency)
	}
	if e2.Impact != ImpactMedium {
		t.Errorf("Expected ImpactMedium, got '%s'", e2.Impact)
	}
	if !e2.Date.Equal(expectedTime) {
		t.Errorf("Expected Date %v to be inherited from row 1, got %v", expectedTime, e2.Date)
	}
}

func TestParseHTMLYearStraddling(t *testing.T) {
	mockHTML := `<html>
<body>
	<table class="calendar__table">
		<tr class="calendar__row">
			<td class="calendar__cell calendar__date">Wed Dec 31</td>
			<td class="calendar__cell calendar__time">10:00pm</td>
			<td class="calendar__cell calendar__currency">USD</td>
			<td class="calendar__cell calendar__impact"><span class="icon--impact-red">High</span></td>
			<td class="calendar__cell calendar__event"><a href="calendar.php?show=1">New Year Eve Event</a></td>
			<td class="calendar__cell calendar__actual">1</td>
			<td class="calendar__cell calendar__forecast">2</td>
			<td class="calendar__cell calendar__previous">3</td>
		</tr>
		<tr class="calendar__row">
			<td class="calendar__cell calendar__date">Thu Jan 1</td>
			<td class="calendar__cell calendar__time">8:30am</td>
			<td class="calendar__cell calendar__currency">EUR</td>
			<td class="calendar__cell calendar__impact"><span class="icon--impact-orange">Medium</span></td>
			<td class="calendar__cell calendar__event"><a href="calendar.php?show=2">New Year Event</a></td>
			<td class="calendar__cell calendar__actual">4</td>
			<td class="calendar__cell calendar__forecast">5</td>
			<td class="calendar__cell calendar__previous">6</td>
		</tr>
	</table>
</body>
</html>`

	// 1. Test ParseHTMLWithSunday (Exact Mapping)
	sunday := time.Date(2025, time.December, 28, 0, 0, 0, 0, time.UTC)
	r1 := bytes.NewReader([]byte(mockHTML))
	events1, err := ParseHTMLWithSunday(r1, sunday, time.UTC)
	if err != nil {
		t.Fatalf("ParseHTMLWithSunday failed: %v", err)
	}

	if len(events1) != 2 {
		t.Fatalf("Expected 2 events, got %d", len(events1))
	}

	if events1[0].Date.Year() != 2025 || events1[0].Date.Month() != time.December || events1[0].Date.Day() != 31 {
		t.Errorf("Expected Dec 31 2025, got %v", events1[0].Date)
	}

	if events1[1].Date.Year() != 2026 || events1[1].Date.Month() != time.January || events1[1].Date.Day() != 1 {
		t.Errorf("Expected Jan 1 2026, got %v", events1[1].Date)
	}

	// 2. Test legacy ParseHTML (Transition Heuristic)
	r2 := bytes.NewReader([]byte(mockHTML))
	events2, err := ParseHTML(r2, 2025, time.UTC)
	if err != nil {
		t.Fatalf("ParseHTML failed: %v", err)
	}

	if len(events2) != 2 {
		t.Fatalf("Expected 2 events, got %d", len(events2))
	}

	if events2[0].Date.Year() != 2025 {
		t.Errorf("Expected legacy Dec 31 to be in 2025, got year %d", events2[0].Date.Year())
	}

	if events2[1].Date.Year() != 2026 {
		t.Errorf("Expected legacy Jan 1 to transition to 2026, got year %d", events2[1].Date.Year())
	}
}

func TestParseHTMLStandaloneMonthDate(t *testing.T) {
	// Tests HTML calendar where date column contains only "May 25" (no weekday name)
	mockHTML := `<html><body>
	<table class="calendar__table">
		<tr class="calendar__row">
			<td class="calendar__cell calendar__date">May 25</td>
			<td class="calendar__cell calendar__time">8:30am</td>
			<td class="calendar__cell calendar__currency">USD</td>
			<td class="calendar__cell calendar__impact"><span class="icon--impact-red">High</span></td>
			<td class="calendar__cell calendar__event"><a href="/calendar/123456-cpi">Core CPI m/m</a></td>
			<td class="calendar__cell calendar__actual">0.3%</td>
			<td class="calendar__cell calendar__forecast">0.2%</td>
			<td class="calendar__cell calendar__previous">0.1%</td>
		</tr>
	</table></body></html>`

	sunday := time.Date(2026, time.May, 24, 0, 0, 0, 0, time.UTC)
	r := bytes.NewReader([]byte(mockHTML))
	events, err := ParseHTMLWithSunday(r, sunday, time.UTC)
	if err != nil {
		t.Fatalf("ParseHTMLWithSunday failed: %v", err)
	}

	if len(events) != 1 {
		t.Fatalf("Expected 1 event, got %d", len(events))
	}

	e := events[0]
	if e.Date.Year() != 2026 || e.Date.Month() != time.May || e.Date.Day() != 25 {
		t.Errorf("Expected parsed date 2026-05-25, got %v", e.Date)
	}
	if e.ID != "123456" {
		t.Errorf("Expected ID '123456', got %q", e.ID)
	}
}

func TestParseHTMLTimeLeakPrevention(t *testing.T) {
	// Tests that event on Monday at 10:00pm does NOT leak its time to Tuesday's all-day event
	mockHTML := `<html><body>
	<table class="calendar__table">
		<tr class="calendar__row">
			<td class="calendar__cell calendar__date">Mon May 25</td>
			<td class="calendar__cell calendar__time">10:00pm</td>
			<td class="calendar__cell calendar__currency">USD</td>
			<td class="calendar__cell calendar__impact"><span class="icon--impact-red">High</span></td>
			<td class="calendar__cell calendar__event"><a href="show=111">Late Event</a></td>
			<td class="calendar__cell calendar__actual"></td>
			<td class="calendar__cell calendar__forecast"></td>
			<td class="calendar__cell calendar__previous"></td>
		</tr>
		<tr class="calendar__row">
			<td class="calendar__cell calendar__date">Tue May 26</td>
			<td class="calendar__cell calendar__time"></td>
			<td class="calendar__cell calendar__currency">EUR</td>
			<td class="calendar__cell calendar__impact"><span class="icon--impact-yellow">Low</span></td>
			<td class="calendar__cell calendar__event"><a href="show=222">Holiday Event</a></td>
			<td class="calendar__cell calendar__actual"></td>
			<td class="calendar__cell calendar__forecast"></td>
			<td class="calendar__cell calendar__previous"></td>
		</tr>
	</table></body></html>`

	sunday := time.Date(2026, time.May, 24, 0, 0, 0, 0, time.UTC)
	r := bytes.NewReader([]byte(mockHTML))
	events, err := ParseHTMLWithSunday(r, sunday, time.UTC)
	if err != nil {
		t.Fatalf("ParseHTMLWithSunday failed: %v", err)
	}

	if len(events) != 2 {
		t.Fatalf("Expected 2 events, got %d", len(events))
	}

	e1 := events[0]
	e2 := events[1]

	if e1.Date.Hour() != 22 || e1.Date.Minute() != 0 {
		t.Errorf("Expected Event 1 to be at 22:00, got %v", e1.Date)
	}

	if !e2.IsAllDay {
		t.Errorf("Expected Event 2 with empty time on new day to be marked IsAllDay")
	}
	if e2.Date.Hour() != 0 || e2.Date.Minute() != 0 {
		t.Errorf("Expected Event 2 to be at 00:00 (All Day), got %v (leaked from Event 1)", e2.Date)
	}
}

func TestParseHTMLMultiDayAndSlugs(t *testing.T) {
	mockHTML := `<html><body>
	<table class="calendar__table">
		<tr class="calendar__row">
			<td class="calendar__cell calendar__date">Wed May 27</td>
			<td class="calendar__cell calendar__time">Day 1</td>
			<td class="calendar__cell calendar__currency">ALL</td>
			<td class="calendar__cell calendar__impact"><span class="icon--impact-red">High</span></td>
			<td class="calendar__cell calendar__event"><a href="/calendar/event/778899-opec-meeting">OPEC-JMMC Meetings</a></td>
			<td class="calendar__cell calendar__actual"></td>
			<td class="calendar__cell calendar__forecast"></td>
			<td class="calendar__cell calendar__previous"></td>
		</tr>
	</table></body></html>`

	sunday := time.Date(2026, time.May, 24, 0, 0, 0, 0, time.UTC)
	r := bytes.NewReader([]byte(mockHTML))
	events, err := ParseHTMLWithSunday(r, sunday, time.UTC)
	if err != nil {
		t.Fatalf("ParseHTMLWithSunday failed: %v", err)
	}

	if len(events) != 1 {
		t.Fatalf("Expected 1 event, got %d", len(events))
	}

	e := events[0]
	if !e.IsAllDay {
		t.Errorf("Expected 'Day 1' event to be IsAllDay = true")
	}
	if e.ID != "778899" {
		t.Errorf("Expected extracted ID '778899', got %q", e.ID)
	}
}

func TestParseEventDetail(t *testing.T) {
	mockDetailHTML := `<html><body>
		<h1 class="calendar__event-title">Non-Farm Employment Change</h1>
		<div class="calendarspecs">
			<div class="calendarspecs__spec">
				<div class="calendarspecs__specname">Source</div>
				<div class="calendarspecs__specdescription">Bureau of Labor Statistics</div>
			</div>
			<div class="calendarspecs__spec">
				<div class="calendarspecs__specname">Measures</div>
				<div class="calendarspecs__specdescription">Change in the number of employed people</div>
			</div>
			<div class="calendarspecs__spec">
				<div class="calendarspecs__specname">Usual Effect</div>
				<div class="calendarspecs__specdescription">'Actual' > 'Forecast' is good for currency</div>
			</div>
			<div class="calendarspecs__spec">
				<div class="calendarspecs__specname">Frequency</div>
				<div class="calendarspecs__specdescription">Released monthly</div>
			</div>
			<div class="calendarspecs__spec">
				<div class="calendarspecs__specname">Next Release</div>
				<div class="calendarspecs__specdescription">Jun 5, 2026</div>
			</div>
			<div class="calendarspecs__spec">
				<div class="calendarspecs__specname">Why Traders Care</div>
				<div class="calendarspecs__specdescription">Job creation is the foremost indicator of consumer spending</div>
			</div>
		</div>
		<table class="calendar__history">
			<tr>
				<th>Date</th><th>Actual</th><th>Forecast</th><th>Previous</th>
			</tr>
			<tr>
				<td class="history__date">May 8, 2026</td>
				<td class="history__actual">175K</td>
				<td class="history__forecast">180K</td>
				<td class="history__previous">315K</td>
			</tr>
			<tr>
				<td class="history__date">Apr 3, 2026</td>
				<td class="history__actual">303K</td>
				<td class="history__forecast">212K</td>
				<td class="history__previous">270K</td>
			</tr>
		</table>
	</body></html>`

	r := bytes.NewReader([]byte(mockDetailHTML))
	detail, err := ParseEventDetail(r, "554433")
	if err != nil {
		t.Fatalf("ParseEventDetail failed: %v", err)
	}

	if detail.ID != "554433" {
		t.Errorf("Expected ID '554433', got %s", detail.ID)
	}
	if detail.Title != "Non-Farm Employment Change" {
		t.Errorf("Expected Title 'Non-Farm Employment Change', got %s", detail.Title)
	}
	if detail.Source != "Bureau of Labor Statistics" {
		t.Errorf("Expected Source 'Bureau of Labor Statistics', got %s", detail.Source)
	}
	if detail.UsualEffect != "'Actual' > 'Forecast' is good for currency" {
		t.Errorf("Expected UsualEffect matched, got %s", detail.UsualEffect)
	}

	if len(detail.History) != 2 {
		t.Fatalf("Expected 2 history rows, got %d", len(detail.History))
	}

	h1 := detail.History[0]
	if h1.Date.Year() != 2026 || h1.Date.Month() != time.May || h1.Date.Day() != 8 {
		t.Errorf("Expected date 2026-05-08, got %v", h1.Date)
	}
	if h1.Actual != "175K" || h1.Forecast != "180K" || h1.Previous != "315K" {
		t.Errorf("Unexpected history values: %v", h1)
	}
}
