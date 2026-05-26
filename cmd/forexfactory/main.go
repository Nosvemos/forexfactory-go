package main

import (
	"context"
	"encoding/csv"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"log"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/Nosvemos/forexfactory-go/pkg/forexfactory"
)

func main() {
	log.SetFlags(0) // Clean logs without prefix timestamps

	if len(os.Args) < 2 {
		printMainHelp()
		os.Exit(1)
	}

	subcommand := os.Args[1]
	switch subcommand {
	case "download":
		handleDownload(os.Args[2:])
	case "live":
		handleLive(os.Args[2:])
	case "help", "-h", "--help":
		printMainHelp()
	default:
		fmt.Fprintf(os.Stderr, "Error: unknown command %q\n\n", subcommand)
		printMainHelp()
		os.Exit(1)
	}
}

func printMainHelp() {
	fmt.Println(`forexfactory-go: Premium Economic Calendar Downloader & Streamer

Usage:
  forexfactory <command> [arguments]

Commands:
  download    Download historical economic calendar data as CSV or JSON
  live        Stream or watch real-time economic events from the XML feed
  help        Print this help menu

Use "forexfactory <command> --help" for detailed documentation on a command.`)
}

func handleDownload(args []string) {
	fs := flag.NewFlagSet("download", flag.ExitOnError)
	startFlag := fs.String("start", "", "Start date in YYYY-MM-DD format (Required)")
	endFlag := fs.String("end", "", "End date in YYYY-MM-DD format (Required)")
	formatFlag := fs.String("format", "json", "Output format: 'json' or 'csv' (default: 'json')")
	outputFlag := fs.String("output", "", "Output file path (defaults to stdout)")
	timezoneFlag := fs.String("timezone", "", "Target timezone for event times (e.g. 'UTC', 'America/New_York', 'Local')")
	rateLimitFlag := fs.Int("rate-limit", 1, "Maximum requests per second (default: 1)")

	fs.Usage = func() {
		fmt.Fprintf(os.Stderr, "Usage of 'forexfactory download':\n")
		fs.PrintDefaults()
	}

	if err := fs.Parse(args); err != nil {
		log.Fatalf("Error parsing arguments: %v", err)
	}

	if *startFlag == "" || *endFlag == "" {
		fmt.Fprintln(os.Stderr, "Error: Both --start and --end flags are required.")
		fs.Usage()
		os.Exit(1)
	}

	startDate, err := time.Parse("2006-01-02", *startFlag)
	if err != nil {
		log.Fatalf("Invalid start date %q: must be YYYY-MM-DD", *startFlag)
	}

	endDate, err := time.Parse("2006-01-02", *endFlag)
	if err != nil {
		log.Fatalf("Invalid end date %q: must be YYYY-MM-DD", *endFlag)
	}

	if startDate.After(endDate) {
		log.Fatalf("Error: --start date cannot be after --end date")
	}

	var targetLoc *time.Location
	if *timezoneFlag != "" {
		targetLoc, err = time.LoadLocation(*timezoneFlag)
		if err != nil {
			log.Fatalf("Failed to load timezone %q: %v", *timezoneFlag, err)
		}
	}

	// Initialize client
	client := forexfactory.NewClient(
		forexfactory.WithRateLimit(*rateLimitFlag),
		forexfactory.WithTimeLocation(targetLoc),
	)

	// Determine output destination
	var out io.Writer = os.Stdout
	if *outputFlag != "" {
		f, err := os.Create(*outputFlag)
		if err != nil {
			log.Fatalf("Failed to create output file %q: %v", *outputFlag, err)
		}
		defer f.Close()
		out = f
	}

	fmt.Fprintf(os.Stderr, "Downloading economic calendar from %s to %s...\n", startDate.Format("2006-01-02"), endDate.Format("2006-01-02"))

	var allEvents []forexfactory.Event
	currentDate := startDate.AddDate(0, 0, -int(startDate.Weekday())) // Jump to Sunday of start date week

	for currentDate.Before(endDate) || currentDate.Equal(endDate) {
		fmt.Fprintf(os.Stderr, " -> Fetching week of %s...\n", currentDate.Format("2006-01-02"))
		events, err := client.FetchWeek(context.Background(), currentDate)
		if err != nil {
			log.Fatalf("Error fetching week of %s: %v", currentDate.Format("2006-01-02"), err)
		}

		// Filter events that fall outside the user's specific requested start/end range
		for _, e := range events {
			// Compare purely by YYYY-MM-DD date components in the source timezone or UTC
			eventDate := time.Date(e.Date.Year(), e.Date.Month(), e.Date.Day(), 0, 0, 0, 0, time.UTC)
			filterStart := time.Date(startDate.Year(), startDate.Month(), startDate.Day(), 0, 0, 0, 0, time.UTC)
			filterEnd := time.Date(endDate.Year(), endDate.Month(), endDate.Day(), 0, 0, 0, 0, time.UTC)

			if (eventDate.After(filterStart) || eventDate.Equal(filterStart)) && (eventDate.Before(filterEnd) || eventDate.Equal(filterEnd)) {
				allEvents = append(allEvents, e)
			}
		}

		currentDate = currentDate.AddDate(0, 0, 7) // Go to next week
	}

	fmt.Fprintf(os.Stderr, "Successfully retrieved %d events. Writing to output...\n", len(allEvents))

	format := strings.ToLower(strings.TrimSpace(*formatFlag))
	switch format {
	case "csv":
		if err := writeCSV(out, allEvents); err != nil {
			log.Fatalf("Error writing CSV: %v", err)
		}
	case "json":
		if err := writeJSON(out, allEvents); err != nil {
			log.Fatalf("Error writing JSON: %v", err)
		}
	default:
		log.Fatalf("Unknown output format %q: use 'json' or 'csv'", *formatFlag)
	}
}

func handleLive(args []string) {
	fs := flag.NewFlagSet("live", flag.ExitOnError)
	intervalFlag := fs.Duration("interval", 0, "Poll interval for watching events live (e.g. '60s', '5m'). If 0, fetches once and exits.")
	timezoneFlag := fs.String("timezone", "", "Target timezone for event times (e.g. 'UTC', 'America/New_York', 'Local')")

	fs.Usage = func() {
		fmt.Fprintf(os.Stderr, "Usage of 'forexfactory live':\n")
		fs.PrintDefaults()
	}

	if err := fs.Parse(args); err != nil {
		log.Fatalf("Error parsing arguments: %v", err)
	}

	var targetLoc *time.Location
	var err error
	if *timezoneFlag != "" {
		targetLoc, err = time.LoadLocation(*timezoneFlag)
		if err != nil {
			log.Fatalf("Failed to load timezone %q: %v", *timezoneFlag, err)
		}
	}

	client := forexfactory.NewClient(
		forexfactory.WithTimeLocation(targetLoc),
	)

	if *intervalFlag <= 0 {
		fetchAndPrintLive(client)
		return
	}

	// Polling / Watching Mode
	ticker := time.NewTicker(*intervalFlag)
	defer ticker.Stop()

	fetchAndPrintLive(client)
	fmt.Printf("\nWatching for changes every %v. Press Ctrl+C to stop...\n", *intervalFlag)

	for range ticker.C {
		fmt.Println("\n--- Updating Live Calendar ---")
		fetchAndPrintLive(client)
	}
}

func fetchAndPrintLive(client *forexfactory.Client) {
	events, err := client.FetchLiveFeed(context.Background())
	if err != nil {
		log.Printf("Error updating live feed: %v\n", err)
		return
	}

	fmt.Println(strings.Repeat("=", 80))
	tzName := "UTC"
	if client.FetchWeek != nil { // placeholder check to get timezone name if set
		// We will extract location from events if timezone location is set
		if len(events) > 0 {
			_, offset := events[0].Date.Zone()
			tzName = fmt.Sprintf("Offset UTC%d", offset/3600)
		}
	}
	fmt.Printf("FOREX FACTORY LIVE WEEKLY ECONOMIC CALENDAR (%s)\n", tzName)
	fmt.Println(strings.Repeat("=", 80))

	for _, e := range events {
		timeStr := e.Date.Format("2006-01-02 15:04")
		if e.IsAllDay {
			timeStr = e.Date.Format("2006-01-02") + " (All Day)"
		} else if e.IsTentative {
			timeStr = e.Date.Format("2006-01-02") + " (Tentative)"
		}

		fmt.Printf("[%s] %-3s | %-6s | %s\n", timeStr, e.Country, e.Impact, e.Title)
		if e.Actual != "" || e.Forecast != "" || e.Previous != "" {
			fmt.Printf("      └─ Actual: %-8s | Forecast: %-8s | Previous: %s\n", e.Actual, e.Forecast, e.Previous)
		}
	}
	fmt.Println(strings.Repeat("=", 80))
}

func writeCSV(w io.Writer, events []forexfactory.Event) error {
	writer := csv.NewWriter(w)
	defer writer.Flush()

	// Compact CSV Header
	err := writer.Write([]string{"id", "title", "country", "date", "impact", "forecast", "previous", "actual", "all_day", "tentative"})
	if err != nil {
		return err
	}

	for _, e := range events {
		err = writer.Write([]string{
			e.ID,
			e.Title,
			e.Country,
			e.Date.Format(time.RFC3339),
			string(e.Impact),
			e.Forecast,
			e.Previous,
			e.Actual,
			strconv.FormatBool(e.IsAllDay),
			strconv.FormatBool(e.IsTentative),
		})
		if err != nil {
			return err
		}
	}
	return nil
}

func writeJSON(w io.Writer, events []forexfactory.Event) error {
	encoder := json.NewEncoder(w)
	encoder.SetIndent("", "  ")
	return encoder.Encode(events)
}
