package forexfactory

import (
	"bytes"
	"encoding/xml"
	"fmt"
	"io"
	"regexp"
	"strings"
	"time"

	"github.com/PuerkitoBio/goquery"
)

// XMLEvents is the root element of the Forex Factory weekly XML feed.
type XMLEvents struct {
	XMLName xml.Name   `xml:"weeklyevents"`
	Events  []XMLEvent `xml:"event"`
}

// XMLEvent is an event represented in the XML feed.
type XMLEvent struct {
	Title    string `xml:"title"`
	Country  string `xml:"country"`
	Date     string `xml:"date"`     // Format: MM-DD-YYYY (e.g., 05-26-2026)
	Time     string `xml:"time"`     // Format: 10:00am, All Day, Tentative, etc.
	Impact   string `xml:"impact"`   // High, Medium, Low, Holiday
	Forecast string `xml:"forecast"`
	Previous string `xml:"previous"`
	Actual   string `xml:"actual"`
}

// parseDetailID regex to extract digits from show details URL
var showIDRegex = regexp.MustCompile(`show=(\d+)|\/(\d+)$`)

// ParseXML parses the XML feed data into a list of Event structs.
func ParseXML(data []byte, targetLoc *time.Location) ([]Event, error) {
	var xmlFeed XMLEvents
	reader := bytes.NewReader(data)
	decoder := xml.NewDecoder(reader)
	decoder.CharsetReader = func(charset string, input io.Reader) (io.Reader, error) {
		return input, nil
	}
	if err := decoder.Decode(&xmlFeed); err != nil {
		return nil, fmt.Errorf("failed to decode XML: %w", err)
	}

	events := make([]Event, 0, len(xmlFeed.Events))
	for _, x := range xmlFeed.Events {
		e := Event{
			Title:    strings.TrimSpace(x.Title),
			Country:  strings.TrimSpace(x.Country),
			Forecast: strings.TrimSpace(x.Forecast),
			Previous: strings.TrimSpace(x.Previous),
			Actual:   strings.TrimSpace(x.Actual),
		}

		// Parse Impact
		impStr := strings.ToLower(strings.TrimSpace(x.Impact))
		switch impStr {
		case "high":
			e.Impact = ImpactHigh
		case "medium":
			e.Impact = ImpactMedium
		case "low":
			e.Impact = ImpactLow
		default:
			e.Impact = ImpactNone
		}

		// Normalize Time & Date
		dateStr := strings.TrimSpace(x.Date) // MM-DD-YYYY
		timeStr := strings.ToLower(strings.TrimSpace(x.Time))

		if timeStr == "all day" || timeStr == "holiday" || timeStr == "" {
			e.IsAllDay = true
		} else if strings.Contains(timeStr, "tentative") {
			e.IsTentative = true
		}

		// Parse Date
		var parsedTime time.Time
		var err error

		// Assume XML dates are in UTC
		sourceLoc := time.UTC

		if e.IsAllDay || e.IsTentative {
			parsedTime, err = time.ParseInLocation("01-02-2006", dateStr, sourceLoc)
		} else {
			// e.g. "05-26-2026 10:00am"
			combined := fmt.Sprintf("%s %s", dateStr, timeStr)
			parsedTime, err = time.ParseInLocation("01-02-2006 3:04pm", combined, sourceLoc)
			if err != nil {
				// Fallback to try 24h format or 15:04 just in case
				parsedTime, err = time.ParseInLocation("01-02-2006 15:04", combined, sourceLoc)
			}
		}

		if err != nil {
			// If parsing fails, fall back to just parsing the date
			parsedTime, _ = time.ParseInLocation("01-02-2006", dateStr, sourceLoc)
		}

		// Convert to target timezone if provided
		if targetLoc != nil {
			e.Date = parsedTime.In(targetLoc)
		} else {
			e.Date = parsedTime
		}

		events = append(events, e)
	}

	return events, nil
}

// ParseHTMLWithSunday parses the calendar HTML from an io.Reader mapping events to the exact 7-day chronological span of a week.
// This mathematically eliminates the year-straddling week bug when crossing from December to January.
func ParseHTMLWithSunday(r io.Reader, sunday time.Time, targetLoc *time.Location) ([]Event, error) {
	doc, err := goquery.NewDocumentFromReader(r)
	if err != nil {
		return nil, fmt.Errorf("failed to load HTML document: %w", err)
	}

	// Calculate the 7 dates for the week Sunday -> Saturday
	weekDates := make([]time.Time, 7)
	for i := 0; i < 7; i++ {
		weekDates[i] = sunday.AddDate(0, 0, i)
	}

	var events []Event
	var lastDateStr string
	var lastTimeStr string

	// Iterate over the calendar table rows
	doc.Find("table.calendar__table tr.calendar__row").Each(func(i int, s *goquery.Selection) {
		// Skip rows that do not contain an actual event (e.g. header, space rows)
		currencyCell := s.Find(".calendar__currency")
		if currencyCell.Length() == 0 {
			return
		}

		currency := strings.TrimSpace(currencyCell.Text())
		title := strings.TrimSpace(s.Find(".calendar__event").Text())

		// If both currency and title are empty, it's not a valid event row
		if currency == "" && title == "" {
			return
		}

		// Extract Date
		dateCell := s.Find(".calendar__date")
		dateStr := strings.TrimSpace(dateCell.Text())
		if dateStr != "" {
			lastDateStr = dateStr
		}

		// Extract Time
		timeCell := s.Find(".calendar__time")
		timeStr := strings.TrimSpace(timeCell.Text())
		if timeStr != "" {
			lastTimeStr = timeStr
		}

		// Extract Impact
		impact := ImpactNone
		impactSpan := s.Find(".calendar__impact span")
		if impactSpan.Length() > 0 {
			classAttr, _ := impactSpan.Attr("class")
			classAttr = strings.ToLower(classAttr)
			if strings.Contains(classAttr, "red") || strings.Contains(classAttr, "high") {
				impact = ImpactHigh
			} else if strings.Contains(classAttr, "orange") || strings.Contains(classAttr, "medium") {
				impact = ImpactMedium
			} else if strings.Contains(classAttr, "yellow") || strings.Contains(classAttr, "low") {
				impact = ImpactLow
			}
		}

		// Extract Detail ID
		var detailID string
		detailAnchor := s.Find(".calendar__event a")
		if detailAnchor.Length() > 0 {
			href, _ := detailAnchor.Attr("href")
			matches := showIDRegex.FindStringSubmatch(href)
			if len(matches) > 1 {
				for _, match := range matches[1:] {
					if match != "" {
						detailID = match
						break
					}
				}
			}
		}

		// Extract values
		actual := strings.TrimSpace(s.Find(".calendar__actual").Text())
		forecast := strings.TrimSpace(s.Find(".calendar__forecast").Text())
		previous := strings.TrimSpace(s.Find(".calendar__previous").Text())

		// Setup Event struct
		e := Event{
			ID:       detailID,
			Title:    title,
			Country:  currency,
			Impact:   impact,
			Forecast: forecast,
			Previous: previous,
			Actual:   actual,
		}

		// Process Date and Time fields
		lowTime := strings.ToLower(lastTimeStr)
		if lowTime == "all day" || lowTime == "holiday" || lowTime == "" {
			e.IsAllDay = true
		} else if strings.Contains(lowTime, "tentative") {
			e.IsTentative = true
		}

		// Parse combined date and time
		// lastDateStr format is usually "Mon May 25" or "May 25"
		// We remove the day prefix (e.g. "Mon ") to make it cleaner
		cleanDateStr := lastDateStr
		if idx := strings.Index(cleanDateStr, " "); idx != -1 && idx < 4 {
			cleanDateStr = cleanDateStr[idx+1:]
		}

		// Find exact date in weekDates by matching month and day
		var eventDate time.Time
		foundDate := false
		parsedMD, err := time.Parse("Jan 2", cleanDateStr)
		if err == nil {
			for _, d := range weekDates {
				if d.Month() == parsedMD.Month() && d.Day() == parsedMD.Day() {
					eventDate = d
					foundDate = true
					break
				}
			}
		}

		// Fallback if matching fails
		if !foundDate {
			combinedDateStr := fmt.Sprintf("%s %d", cleanDateStr, sunday.Year())
			eventDate, _ = time.ParseInLocation("Jan 2 2006", combinedDateStr, time.UTC)
		}

		var parsedTime time.Time
		sourceLoc := time.UTC // default source is UTC or Eastern, we align via UTC cookies

		if e.IsAllDay || e.IsTentative {
			parsedTime = time.Date(eventDate.Year(), eventDate.Month(), eventDate.Day(), 0, 0, 0, 0, sourceLoc)
		} else {
			// clean lastTimeStr from characters like "pm", "am"
			combinedTimeStr := fmt.Sprintf("%d-%02d-%02d %s", eventDate.Year(), eventDate.Month(), eventDate.Day(), strings.ToLower(lastTimeStr))
			parsedTime, err = time.ParseInLocation("2006-01-02 3:04pm", combinedTimeStr, sourceLoc)
			if err != nil {
				parsedTime, err = time.ParseInLocation("2006-01-02 15:04", combinedTimeStr, sourceLoc)
			}
			if err != nil {
				// Fallback to date-only parsing to prevent zero value dates
				parsedTime = time.Date(eventDate.Year(), eventDate.Month(), eventDate.Day(), 0, 0, 0, 0, sourceLoc)
			}
		}

		// Apply target location conversion if provided
		if targetLoc != nil {
			e.Date = parsedTime.In(targetLoc)
		} else {
			e.Date = parsedTime
		}

		events = append(events, e)
	})

	return events, nil
}

// ParseHTML parses the calendar HTML from an io.Reader.
// The refYear is needed because the calendar HTML does not contain the year explicitly.
// By default, it assumes Sunday falls in refYear, and creates a simulated Sunday time.Time
// to call ParseHTMLWithSunday. It also supports year transition tracking in case of cross-year weeks.
func ParseHTML(r io.Reader, refYear int, targetLoc *time.Location) ([]Event, error) {
	// Parse with a smart year transition heuristic if Sunday's exact date is not provided
	doc, err := goquery.NewDocumentFromReader(r)
	if err != nil {
		return nil, fmt.Errorf("failed to load HTML document: %w", err)
	}

	var events []Event
	var lastDateStr string
	var lastTimeStr string
	var previousMonth time.Month = 0
	currentYear := refYear

	// Iterate over the calendar table rows
	doc.Find("table.calendar__table tr.calendar__row").Each(func(i int, s *goquery.Selection) {
		// Skip rows that do not contain an actual event (e.g. header, space rows)
		currencyCell := s.Find(".calendar__currency")
		if currencyCell.Length() == 0 {
			return
		}

		currency := strings.TrimSpace(currencyCell.Text())
		title := strings.TrimSpace(s.Find(".calendar__event").Text())

		// If both currency and title are empty, it's not a valid event row
		if currency == "" && title == "" {
			return
		}

		// Extract Date
		dateCell := s.Find(".calendar__date")
		dateStr := strings.TrimSpace(dateCell.Text())
		if dateStr != "" {
			lastDateStr = dateStr
		}

		// Extract Time
		timeCell := s.Find(".calendar__time")
		timeStr := strings.TrimSpace(timeCell.Text())
		if timeStr != "" {
			lastTimeStr = timeStr
		}

		// Extract Impact
		impact := ImpactNone
		impactSpan := s.Find(".calendar__impact span")
		if impactSpan.Length() > 0 {
			classAttr, _ := impactSpan.Attr("class")
			classAttr = strings.ToLower(classAttr)
			if strings.Contains(classAttr, "red") || strings.Contains(classAttr, "high") {
				impact = ImpactHigh
			} else if strings.Contains(classAttr, "orange") || strings.Contains(classAttr, "medium") {
				impact = ImpactMedium
			} else if strings.Contains(classAttr, "yellow") || strings.Contains(classAttr, "low") {
				impact = ImpactLow
			}
		}

		// Extract Detail ID
		var detailID string
		detailAnchor := s.Find(".calendar__event a")
		if detailAnchor.Length() > 0 {
			href, _ := detailAnchor.Attr("href")
			matches := showIDRegex.FindStringSubmatch(href)
			if len(matches) > 1 {
				for _, match := range matches[1:] {
					if match != "" {
						detailID = match
						break
					}
				}
			}
		}

		// Extract values
		actual := strings.TrimSpace(s.Find(".calendar__actual").Text())
		forecast := strings.TrimSpace(s.Find(".calendar__forecast").Text())
		previous := strings.TrimSpace(s.Find(".calendar__previous").Text())

		// Setup Event struct
		e := Event{
			ID:       detailID,
			Title:    title,
			Country:  currency,
			Impact:   impact,
			Forecast: forecast,
			Previous: previous,
			Actual:   actual,
		}

		// Process Date and Time fields
		lowTime := strings.ToLower(lastTimeStr)
		if lowTime == "all day" || lowTime == "holiday" || lowTime == "" {
			e.IsAllDay = true
		} else if strings.Contains(lowTime, "tentative") {
			e.IsTentative = true
		}

		// Parse combined date and time
		// lastDateStr format is usually "Mon May 25" or "May 25"
		// We remove the day prefix (e.g. "Mon ") to make it cleaner
		cleanDateStr := lastDateStr
		if idx := strings.Index(cleanDateStr, " "); idx != -1 && idx < 4 {
			cleanDateStr = cleanDateStr[idx+1:]
		}

		// Smart Year Straddling detection
		parsedMD, err := time.Parse("Jan 2", cleanDateStr)
		if err == nil {
			if parsedMD.Month() == time.January && previousMonth == time.December {
				currentYear = refYear + 1
			}
			previousMonth = parsedMD.Month()
		}

		// Format clean date as "May 25 2026"
		combinedDateStr := fmt.Sprintf("%s %d", cleanDateStr, currentYear)

		var parsedTime time.Time
		sourceLoc := time.UTC // default source is UTC or Eastern, we align via UTC cookies

		if e.IsAllDay || e.IsTentative {
			parsedTime, _ = time.ParseInLocation("Jan 2 2006", combinedDateStr, sourceLoc)
		} else {
			// clean lastTimeStr from characters like "pm", "am"
			combinedTimeStr := fmt.Sprintf("%s %s", combinedDateStr, strings.ToLower(lastTimeStr))
			parsedTime, err = time.ParseInLocation("Jan 2 2006 3:04pm", combinedTimeStr, sourceLoc)
			if err != nil {
				parsedTime, err = time.ParseInLocation("Jan 2 2006 15:04", combinedTimeStr, sourceLoc)
			}
		}

		if err != nil {
			// Fallback to date-only parsing to prevent zero value dates
			parsedTime, _ = time.ParseInLocation("Jan 2 2006", combinedDateStr, sourceLoc)
		}

		// Apply target location conversion if provided
		if targetLoc != nil {
			e.Date = parsedTime.In(targetLoc)
		} else {
			e.Date = parsedTime
		}

		events = append(events, e)
	})

	return events, nil
}
