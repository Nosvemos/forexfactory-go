package main

import (
	"context"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/Nosvemos/forexfactory-go/pkg/forexfactory"
	"github.com/Nosvemos/forexfactory-go/pkg/storage"
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

	// Flags for dbload command
	dbFlag string

	// Global silent flag
	silentFlag bool
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

var dbloadCmd = &cobra.Command{
	Use:   "dbload",
	Short: "Download and load calendar data into a local SQLite database",
	Long: `Concurrently downloads economic calendar events across a specified date range 
and loads/updates them directly inside a CGO-free local SQLite database via storage SDK.`,
	Run: func(cmd *cobra.Command, args []string) {
		executeDbLoad()
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

	downloadCmd.Flags().BoolVarP(&silentFlag, "silent", "q", false, "Mute all logging and progress outputs")
	_ = downloadCmd.MarkFlagRequired("start")
	_ = downloadCmd.MarkFlagRequired("end")

	// Configure dbload command flags
	dbloadCmd.Flags().StringVarP(&startFlag, "start", "s", "", "Start date in YYYY-MM-DD format (Required)")
	dbloadCmd.Flags().StringVarP(&endFlag, "end", "e", "", "End date in YYYY-MM-DD format (Required)")
	dbloadCmd.Flags().StringVarP(&timezoneFlag, "timezone", "t", "", "Target timezone for event times (e.g. 'UTC', 'America/New_York')")
	dbloadCmd.Flags().IntVarP(&rateLimitFlag, "rate-limit", "r", 1, "Maximum HTTP requests per second allowed")
	dbloadCmd.Flags().IntVarP(&concurrencyFlag, "concurrency", "c", 3, "Number of concurrent downloading worker threads")
	dbloadCmd.Flags().StringVar(&cookieFlag, "cookie", "", "Cloudflare clearance cookie (cf_clearance=...) to bypass anti-bot blocks")
	dbloadCmd.Flags().StringVarP(&dbFlag, "db", "d", "forexfactory.db", "SQLite database file path")
	dbloadCmd.Flags().BoolVarP(&silentFlag, "silent", "q", false, "Mute all logging and progress outputs")

	_ = dbloadCmd.MarkFlagRequired("start")
	_ = dbloadCmd.MarkFlagRequired("end")

	// Configure live command flags
	liveCmd.Flags().DurationVarP(&intervalFlag, "interval", "i", 0, "Polling interval for live streaming (e.g. '60s', '5m'). If 0, fetches once and exits.")
	liveCmd.Flags().StringVarP(&liveTimeLoc, "timezone", "t", "", "Target timezone for event times (e.g. 'UTC', 'America/New_York')")
	liveCmd.Flags().StringVar(&cookieFlag, "cookie", "", "Cloudflare clearance cookie (cf_clearance=...) to bypass anti-bot blocks")

	// Bind commands to root
	rootCmd.AddCommand(downloadCmd)
	rootCmd.AddCommand(dbloadCmd)
	rootCmd.AddCommand(liveCmd)
}

func main() {
	log.SetFlags(0)
	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}

func fetchEventsConcurrently(startDate, endDate time.Time, targetLoc *time.Location) []forexfactory.Event {
	// Initialize Client options
	var clientOpts []forexfactory.Option
	clientOpts = append(clientOpts, forexfactory.WithRateLimit(rateLimitFlag))
	clientOpts = append(clientOpts, forexfactory.WithConcurrency(concurrencyFlag))
	clientOpts = append(clientOpts, forexfactory.WithTimeLocation(targetLoc))
	if cookieFlag != "" {
		clientOpts = append(clientOpts, forexfactory.WithHeader("Cookie", cookieFlag))
	}

	if !silentFlag {
		// Dynamic visual progress callback
		fmt.Fprintf(os.Stderr, "Downloading calendar data via %d concurrent workers...\n", concurrencyFlag)
		clientOpts = append(clientOpts, forexfactory.WithProgressCallback(func(current, total int) {
			pct := current * 100 / total
			barLen := 30
			filledLen := current * barLen / total
			bar := strings.Repeat("█", filledLen) + strings.Repeat("░", barLen-filledLen)
			fmt.Fprintf(os.Stderr, "\r[%s] %d%% Completed (%d/%d weeks)", bar, pct, current, total)
		}))
	}

	client := forexfactory.NewClient(clientOpts...)

	events, err := client.FetchRange(context.Background(), startDate, endDate)
	if err != nil {
		if !silentFlag {
			fmt.Fprintln(os.Stderr) // Move cursor past progress bar
		}
		log.Fatalf("\nError downloading events: %v", err)
	}

	if !silentFlag {
		fmt.Fprintln(os.Stderr, "\nProcessing, filtering and sorting events...")
	}
	return events
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

	filteredEvents := fetchEventsConcurrently(startDate, endDate, targetLoc)

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

	if !silentFlag {
		fmt.Fprintf(os.Stderr, "Successfully exported %d events.\n", len(filteredEvents))
	}
}

func executeDbLoad() {
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

	// 1. Fetch events concurrently
	filteredEvents := fetchEventsConcurrently(startDate, endDate, targetLoc)

	// 2. Initialize modular SQLite Storage driver via SDK
	store := storage.NewSQLiteStorage(dbFlag)
	ctx := context.Background()

	if err := store.Init(ctx); err != nil {
		log.Fatalf("Failed to initialize database: %v", err)
	}
	defer store.Close()

	// 3. Save events utilizing the modular Storage SDK interface
	if err := store.SaveEvents(ctx, filteredEvents); err != nil {
		log.Fatalf("Failed to load events into database: %v", err)
	}

	if !silentFlag {
		fmt.Fprintf(os.Stderr, "Successfully imported %d events into SQLite database %q via storage SDK.\n", len(filteredEvents), dbFlag)
	}
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

	client := forexfactory.NewClient(
		forexfactory.WithTimeLocation(targetLoc),
	)

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

	tzName := "UTC"
	if len(events) > 0 {
		_, offset := events[0].Date.Zone()
		tzName = fmt.Sprintf("Offset UTC%+d", offset/3600)
	}

	fmt.Println()
	fmt.Printf("\033[1;36m┌%s┐\033[0m\n", strings.Repeat("─", 88))
	headerText := fmt.Sprintf(" FOREX FACTORY LIVE WEEKLY ECONOMIC CALENDAR (%s) ", tzName)
	padding := (88 - len(headerText)) / 2
	if padding < 0 { padding = 0 }
	fmt.Printf("\033[1;36m│\033[0m%s\033[1;35m%s\033[0m%s\033[1;36m│\033[0m\n", strings.Repeat(" ", padding), headerText, strings.Repeat(" ", 88-len(headerText)-padding))
	fmt.Printf("\033[1;36m├%s┤\033[0m\n", strings.Repeat("─", 88))

	for _, e := range events {
		timeStr := e.Date.Format("2006-01-02 15:04")
		if e.IsAllDay {
			timeStr = e.Date.Format("2006-01-02") + " (All Day)"
		} else if e.IsTentative {
			timeStr = e.Date.Format("2006-01-02") + " (Tentative)"
		}

		var impactStr string
		switch e.Impact {
		case forexfactory.ImpactHigh:
			impactStr = "\033[1;31m🔴 HIGH\033[0m  "
		case forexfactory.ImpactMedium:
			impactStr = "\033[1;33m🟡 MEDIUM\033[0m"
		case forexfactory.ImpactLow:
			impactStr = "\033[1;32m🟢 LOW\033[0m   "
		default:
			impactStr = "\033[1;90m⚪ NONE\033[0m  "
		}

		titleText := e.Title
		if len(titleText) > 42 {
			titleText = titleText[:39] + "..."
		}
		
		fmt.Printf("\033[1;36m│\033[0m [\033[1;37m%-16s\033[0m] \033[1;34m%-3s\033[0m │ %s │ \033[1m%-42s\033[0m \033[1;36m│\033[0m\n", 
			timeStr, e.Country, impactStr, titleText)

		if e.Actual != "" || e.Forecast != "" || e.Previous != "" {
			act := e.Actual
			if act == "" { act = "-" }
			forc := e.Forecast
			if forc == "" { forc = "-" }
			prev := e.Previous
			if prev == "" { prev = "-" }

			fmt.Printf("\033[1;36m│\033[0m      \033[90m└─ Actual:\033[0m \033[1;32m%-10s\033[0m \033[90mForecast:\033[0m \033[1;33m%-10s\033[0m \033[90mPrevious:\033[0m \033[1;37m%-10s\033[0m \033[1;36m│\033[0m\n", 
				act, forc, prev)
		}
	}
	fmt.Printf("\033[1;36m└%s┘\033[0m\n", strings.Repeat("─", 88))
	fmt.Println()
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
