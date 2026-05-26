package main

import (
	"context"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"os"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/Nosvemos/forexfactory-go/pkg/forexfactory"
	"github.com/spf13/cobra"
)

var (
	// Flags for download command
	startFlag       string
	endFlag         string
	formatFlag      string
	outputFlag      string
	timezoneFlag    string
	rateLimitFlag   int
	concurrencyFlag int
	cookieFlag      string // Cookie for Cloudflare bypass

	// Flags for live command
	intervalFlag time.Duration
	liveTimeLoc  string
)

var rootCmd = &cobra.Command{
	Use:   "forexfactory",
	Short: "forexfactory-go: Premium Economic Calendar Downloader & Streamer",
	Long: `forexfactory-go is a high-performance Go CLI and library designed to fetch, 
stream, and analyze economic calendar data from Forex Factory concurrently.`,
}

var downloadCmd = &cobra.Command{
	Use:   "download",
	Short: "Download historical economic calendar data",
	Long: `Concurrently downloads historical economic calendar data across a specified 
date range and exports it to compact, chronologically sorted CSV or JSON files.`,
	Run: func(cmd *cobra.Command, args []string) {
		executeDownload()
	},
}

var liveCmd = &cobra.Command{
	Use:   "live",
	Short: "Stream or watch real-time economic events",
	Long:  `Streams real-time economic events from the lightweight XML feed with optional custom watch loops.`,
	Run: func(cmd *cobra.Command, args []string) {
		executeLive()
	},
}

func init() {
	// Configure download command flags
	downloadCmd.Flags().StringVarP(&startFlag, "start", "s", "", "Start date in YYYY-MM-DD format (Required)")
	downloadCmd.Flags().StringVarP(&endFlag, "end", "e", "", "End date in YYYY-MM-DD format (Required)")
	downloadCmd.Flags().StringVarP(&formatFlag, "format", "f", "json", "Output format: 'json' or 'csv'")
	downloadCmd.Flags().StringVarP(&outputFlag, "output", "o", "", "Output file path (defaults to stdout)")
	downloadCmd.Flags().StringVarP(&timezoneFlag, "timezone", "t", "", "Target timezone for event times (e.g. 'UTC', 'America/New_York')")
	downloadCmd.Flags().IntVarP(&rateLimitFlag, "rate-limit", "r", 1, "Maximum HTTP requests per second allowed")
	downloadCmd.Flags().IntVarP(&concurrencyFlag, "concurrency", "c", 3, "Number of concurrent downloading worker threads")
	downloadCmd.Flags().StringVar(&cookieFlag, "cookie", "", "Cloudflare clearance cookie (cf_clearance=...) to bypass anti-bot blocks")

	_ = downloadCmd.MarkFlagRequired("start")
	_ = downloadCmd.MarkFlagRequired("end")

	// Configure live command flags
	liveCmd.Flags().DurationVarP(&intervalFlag, "interval", "i", 0, "Polling interval for live streaming (e.g. '60s', '5m'). If 0, fetches once and exits.")
	liveCmd.Flags().StringVarP(&liveTimeLoc, "timezone", "t", "", "Target timezone for event times (e.g. 'UTC', 'America/New_York')")
	liveCmd.Flags().StringVar(&cookieFlag, "cookie", "", "Cloudflare clearance cookie (cf_clearance=...) to bypass anti-bot blocks")

	// Bind commands to root
	rootCmd.AddCommand(downloadCmd)
	rootCmd.AddCommand(liveCmd)
}

func main() {
	log.SetFlags(0)
	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}

type job struct {
	sunday time.Time
}

type result struct {
	events []forexfactory.Event
	err    error
	sunday time.Time
}

func executeDownload() {
	startDate, err := time.Parse("2006-01-02", startFlag)
	if err != nil {
		log.Fatalf("Invalid start date %q: must be YYYY-MM-DD", startFlag)
	}

	endDate, err := time.Parse("2006-01-02", endFlag)
	if err != nil {
		log.Fatalf("Invalid end date %q: must be YYYY-MM-DD", endFlag)
	}

	if startDate.After(endDate) {
		log.Fatalf("Error: start date cannot be after end date")
	}

	var targetLoc *time.Location
	if timezoneFlag != "" {
		targetLoc, err = time.LoadLocation(timezoneFlag)
		if err != nil {
			log.Fatalf("Failed to load timezone %q: %v", timezoneFlag, err)
		}
	}

	// Calculate all Sundays spanning the range week-by-week
	var sundays []time.Time
	currentDate := startDate.AddDate(0, 0, -int(startDate.Weekday())) // Sun of start week
	for currentDate.Before(endDate) || currentDate.Equal(endDate) {
		sundays = append(sundays, currentDate)
		currentDate = currentDate.AddDate(0, 0, 7)
	}

	// Initialize Client
	var clientOpts []forexfactory.Option
	clientOpts = append(clientOpts, forexfactory.WithRateLimit(rateLimitFlag))
	clientOpts = append(clientOpts, forexfactory.WithTimeLocation(targetLoc))
	if cookieFlag != "" {
		clientOpts = append(clientOpts, forexfactory.WithHeader("Cookie", cookieFlag))
	}
	client := forexfactory.NewClient(clientOpts...)

	// Concurrency Worker Pool Setup
	numJobs := len(sundays)
	jobsChan := make(chan job, numJobs)
	resultsChan := make(chan result, numJobs)

	workers := concurrencyFlag
	if workers > numJobs {
		workers = numJobs
	}
	if workers < 1 {
		workers = 1
	}

	// Start concurrent workers
	for w := 1; w <= workers; w++ {
		go func() {
			for j := range jobsChan {
				events, err := client.FetchWeek(context.Background(), j.sunday)
				resultsChan <- result{events: events, err: err, sunday: j.sunday}
			}
		}()
	}

	// Dispatch jobs
	for _, s := range sundays {
		jobsChan <- job{sunday: s}
	}
	close(jobsChan)

	// Collect results with dynamic visual ASCII progress bar
	fmt.Fprintf(os.Stderr, "Downloading calendar data via %d concurrent workers...\n", workers)
	
	var allEvents []forexfactory.Event
	for i := 0; i < numJobs; i++ {
		res := <-resultsChan
		if res.err != nil {
			log.Fatalf("\nError downloading week of %s: %v", res.sunday.Format("2006-01-02"), res.err)
		}

		allEvents = append(allEvents, res.events...)

		// Render interactive terminal progress indicator
		pct := (i + 1) * 100 / numJobs
		barLen := 30
		filledLen := (i + 1) * barLen / numJobs
		bar := strings.Repeat("█", filledLen) + strings.Repeat("░", barLen-filledLen)
		fmt.Fprintf(os.Stderr, "\r[%s] %d%% Completed (%d/%d weeks)", bar, pct, i+1, numJobs)
	}
	fmt.Fprintln(os.Stderr, "\nProcessing, filtering and sorting events...")

	// Filter out events falling strictly outside of start/end range
	var filteredEvents []forexfactory.Event
	for _, e := range allEvents {
		eventDate := time.Date(e.Date.Year(), e.Date.Month(), e.Date.Day(), 0, 0, 0, 0, time.UTC)
		filterStart := time.Date(startDate.Year(), startDate.Month(), startDate.Day(), 0, 0, 0, 0, time.UTC)
		filterEnd := time.Date(endDate.Year(), endDate.Month(), endDate.Day(), 0, 0, 0, 0, time.UTC)

		if (eventDate.After(filterStart) || eventDate.Equal(filterStart)) && (eventDate.Before(filterEnd) || eventDate.Equal(filterEnd)) {
			filteredEvents = append(filteredEvents, e)
		}
	}

	// Sort events chronologically to restore order from parallel downloads
	sort.Slice(filteredEvents, func(i, j int) bool {
		return filteredEvents[i].Date.Before(filteredEvents[j].Date)
	})

	// Format output destination
	var out io.Writer = os.Stdout
	if outputFlag != "" {
		f, err := os.Create(outputFlag)
		if err != nil {
			log.Fatalf("Failed to create output file %q: %v", outputFlag, err)
		}
		defer f.Close()
		out = f
	}

	format := strings.ToLower(strings.TrimSpace(formatFlag))
	switch format {
	case "csv":
		if err := writeCSV(out, filteredEvents); err != nil {
			log.Fatalf("Error writing CSV: %v", err)
		}
	case "json":
		if err := writeJSON(out, filteredEvents); err != nil {
			log.Fatalf("Error writing JSON: %v", err)
		}
	default:
		log.Fatalf("Unknown output format %q: use 'json' or 'csv'", formatFlag)
	}

	fmt.Fprintf(os.Stderr, "Successfully exported %d events.\n", len(filteredEvents))
}

func executeLive() {
	var targetLoc *time.Location
	var err error
	if liveTimeLoc != "" {
		targetLoc, err = time.LoadLocation(liveTimeLoc)
		if err != nil {
			log.Fatalf("Failed to load timezone %q: %v", liveTimeLoc, err)
		}
	}

	var clientOpts []forexfactory.Option
	clientOpts = append(clientOpts, forexfactory.WithTimeLocation(targetLoc))
	if cookieFlag != "" {
		clientOpts = append(clientOpts, forexfactory.WithHeader("Cookie", cookieFlag))
	}
	client := forexfactory.NewClient(clientOpts...)

	if intervalFlag <= 0 {
		fetchAndPrintLive(client)
		return
	}

	// Polling mode
	ticker := time.NewTicker(intervalFlag)
	defer ticker.Stop()

	fetchAndPrintLive(client)
	fmt.Printf("\nWatching XML economic feed every %v. Press Ctrl+C to stop...\n", intervalFlag)

	for range ticker.C {
		fmt.Println("\n--- Updating Live Calendar ---")
		fetchAndPrintLive(client)
	}
}

func fetchAndPrintLive(client *forexfactory.Client) {
	events, err := client.FetchLiveFeed(context.Background())
	if err != nil {
		log.Printf("Error: failed to fetch live feed: %v\n", err)
		return
	}

	fmt.Println(strings.Repeat("=", 80))
	tzName := "UTC"
	if len(events) > 0 {
		_, offset := events[0].Date.Zone()
		tzName = fmt.Sprintf("Offset UTC%d", offset/3600)
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
