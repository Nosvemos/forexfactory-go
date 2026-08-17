package main

import (
	"context"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/Nosvemos/forexfactory-go/pkg/bridge"
	"github.com/Nosvemos/forexfactory-go/pkg/forexfactory"
	"github.com/Nosvemos/forexfactory-go/pkg/server"
	"github.com/Nosvemos/forexfactory-go/pkg/storage"
	"github.com/olekukonko/tablewriter"
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

	// Flags for serve command
	portFlag string

	// Flags for bridge command
	mt4DirFlag         string
	minImpactFlag      string
	currenciesFlag     string
	bridgeIntervalFlag time.Duration

	// Flags for live command
	intervalFlag time.Duration
	liveTimeLoc  string

	// Flags for dbload command
	dbFlag string

	// Flags for event command
	eventIDFlag string

	// Global silent flag
	silentFlag bool

	// Chrome options
	headlessFlag bool
)

var rootCmd = &cobra.Command{
	Use:   "forexfactory-go",
	Short: "High-performance Forex Factory Calendar Scraper and Streamer",
	Long: `forexfactory-go is a high-performance Go CLI and library designed to fetch, 
stream, and analyze economic calendar data from Forex Factory concurrently.`,
}

var downloadCmd = &cobra.Command{
	Use:   "download",
	Short: "Download historical economic calendar data",
	Long: `Concurrently downloads historical economic calendar data across a specified 
date range and exports it to compact, chronologically sorted CSV, JSON, Parquet, or Excel (XLSX) files.`,
	Run: func(cmd *cobra.Command, args []string) {
		executeDownload()
	},
}

var eventCmd = &cobra.Command{
	Use:   "event [eventID]",
	Short: "Fetch deep-dive metadata and historical releases for a specific event",
	Long:  `Scrapes detailed specifications (Source, Measures, Usual Effect, Frequency, Next Release) and past historical releases table for a specific Forex Factory event ID.`,
	Run: func(cmd *cobra.Command, args []string) {
		executeEvent(args)
	},
}

var serveCmd = &cobra.Command{
	Use:   "serve",
	Short: "Launch high-performance REST API and SSE streaming microservice",
	Long:  `Starts an embedded HTTP server exposing RESTful endpoints (/api/v1/calendar, /api/v1/event, /api/v1/live, /api/v1/stream) and live Server-Sent Events.`,
	Run: func(cmd *cobra.Command, args []string) {
		executeServe()
	},
}

var bridgeCmd = &cobra.Command{
	Use:   "bridge",
	Short: "Synchronize live news filter files for MetaTrader 4 / 5 Expert Advisors",
	Long:  `Periodically polls live economic events and publishes atomic ff_news_filter.json & ff_news_filter.csv files into your MT4/MT5 MQL Files directory for algorithmic EA news filtering.`,
	Run: func(cmd *cobra.Command, args []string) {
		executeBridge()
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
	downloadCmd.Flags().StringVarP(&formatFlag, "format", "f", "json", "Output format: 'json', 'csv', 'parquet', or 'xlsx'")
	downloadCmd.Flags().StringVarP(&outputFlag, "output", "o", "", "Output file path (defaults to stdout)")
	downloadCmd.Flags().StringVarP(&timezoneFlag, "timezone", "t", "", "Target timezone for event times (e.g. 'UTC', 'America/New_York')")
	downloadCmd.Flags().IntVarP(&rateLimitFlag, "rate-limit", "r", 1, "Maximum HTTP requests per second allowed")
	downloadCmd.Flags().IntVarP(&concurrencyFlag, "concurrency", "c", 3, "Number of concurrent downloading worker threads")
	downloadCmd.Flags().StringVar(&cookieFlag, "cookie", "", "Cloudflare clearance cookie (cf_clearance=...) to bypass anti-bot blocks")

	downloadCmd.Flags().BoolVarP(&silentFlag, "silent", "q", false, "Mute all logging and progress outputs")
	downloadCmd.Flags().BoolVar(&headlessFlag, "headless", true, "Use headless browser mode for Cloudflare bypass (automatically falls back to headed mode if blocked)")
	_ = downloadCmd.MarkFlagRequired("start")
	_ = downloadCmd.MarkFlagRequired("end")

	// Configure event command flags
	eventCmd.Flags().StringVarP(&eventIDFlag, "id", "i", "", "Forex Factory Event ID (e.g. 123456)")
	eventCmd.Flags().StringVar(&cookieFlag, "cookie", "", "Cloudflare clearance cookie (cf_clearance=...) to bypass anti-bot blocks")
	eventCmd.Flags().BoolVar(&headlessFlag, "headless", true, "Use headless browser mode for Cloudflare bypass")

	// Configure serve command flags
	serveCmd.Flags().StringVarP(&portFlag, "port", "p", "8080", "HTTP port to listen on (e.g. 8080)")
	serveCmd.Flags().IntVarP(&rateLimitFlag, "rate-limit", "r", 1, "Maximum HTTP requests per second allowed")
	serveCmd.Flags().IntVarP(&concurrencyFlag, "concurrency", "c", 3, "Number of concurrent downloading worker threads")
	serveCmd.Flags().StringVar(&cookieFlag, "cookie", "", "Cloudflare clearance cookie (cf_clearance=...) to bypass anti-bot blocks")
	serveCmd.Flags().BoolVar(&headlessFlag, "headless", true, "Use headless browser mode for Cloudflare bypass")

	// Configure bridge command flags
	bridgeCmd.Flags().StringVarP(&mt4DirFlag, "mt4-dir", "m", ".", "MT4/MT5 Terminal MQL Files directory path")
	bridgeCmd.Flags().StringVarP(&minImpactFlag, "min-impact", "l", "High", "Minimum impact level to include (High, Medium, Low)")
	bridgeCmd.Flags().StringVarP(&currenciesFlag, "currency", "u", "", "Comma-separated list of currencies to filter (e.g. 'USD,EUR,GBP')")
	bridgeCmd.Flags().DurationVarP(&bridgeIntervalFlag, "interval", "i", 60*time.Second, "Sync interval to check and refresh news filter")
	bridgeCmd.Flags().StringVar(&cookieFlag, "cookie", "", "Cloudflare clearance cookie (cf_clearance=...) to bypass anti-bot blocks")
	bridgeCmd.Flags().BoolVar(&headlessFlag, "headless", true, "Use headless browser mode for Cloudflare bypass")

	// Configure dbload command flags
	dbloadCmd.Flags().StringVarP(&startFlag, "start", "s", "", "Start date in YYYY-MM-DD format (Required)")
	dbloadCmd.Flags().StringVarP(&endFlag, "end", "e", "", "End date in YYYY-MM-DD format (Required)")
	dbloadCmd.Flags().StringVarP(&timezoneFlag, "timezone", "t", "", "Target timezone for event times (e.g. 'UTC', 'America/New_York')")
	dbloadCmd.Flags().IntVarP(&rateLimitFlag, "rate-limit", "r", 1, "Maximum HTTP requests per second allowed")
	dbloadCmd.Flags().IntVarP(&concurrencyFlag, "concurrency", "c", 3, "Number of concurrent downloading worker threads")
	dbloadCmd.Flags().StringVar(&cookieFlag, "cookie", "", "Cloudflare clearance cookie (cf_clearance=...) to bypass anti-bot blocks")
	dbloadCmd.Flags().StringVarP(&dbFlag, "db", "d", "forexfactory.db", "SQLite database file path")
	dbloadCmd.Flags().BoolVarP(&silentFlag, "silent", "q", false, "Mute all logging and progress outputs")
	dbloadCmd.Flags().BoolVar(&headlessFlag, "headless", true, "Use headless browser mode for Cloudflare bypass (automatically falls back to headed mode if blocked)")

	_ = dbloadCmd.MarkFlagRequired("start")
	_ = dbloadCmd.MarkFlagRequired("end")

	// Configure live command flags
	liveCmd.Flags().DurationVarP(&intervalFlag, "interval", "i", 0, "Polling interval for live streaming (e.g. '60s', '5m'). If 0, fetches once and exits.")
	liveCmd.Flags().StringVarP(&liveTimeLoc, "timezone", "t", "", "Target timezone for event times (e.g. 'UTC', 'America/New_York')")
	liveCmd.Flags().StringVar(&cookieFlag, "cookie", "", "Cloudflare clearance cookie (cf_clearance=...) to bypass anti-bot blocks")

	// Bind commands to root
	rootCmd.AddCommand(downloadCmd)
	rootCmd.AddCommand(eventCmd)
	rootCmd.AddCommand(serveCmd)
	rootCmd.AddCommand(bridgeCmd)
	rootCmd.AddCommand(dbloadCmd)
	rootCmd.AddCommand(liveCmd)
}

func executeBridge() {
	if mt4DirFlag == "" {
		mt4DirFlag = "."
	}

	var currs []string
	if currenciesFlag != "" {
		for _, c := range strings.Split(currenciesFlag, ",") {
			if trimmed := strings.TrimSpace(c); trimmed != "" {
				currs = append(currs, strings.ToUpper(trimmed))
			}
		}
	}

	imp := forexfactory.ImpactHigh
	switch strings.ToLower(minImpactFlag) {
	case "medium":
		imp = forexfactory.ImpactMedium
	case "low":
		imp = forexfactory.ImpactLow
	}

	fmt.Printf("\033[1;36m================================================================================\033[0m\n")
	fmt.Printf("\033[1;32m  FOREXFACTORY-GO METATRADER 4 / 5 NEWS FILTER BRIDGE\033[0m\n")
	fmt.Printf("\033[1;36m================================================================================\033[0m\n")
	fmt.Printf("  • Target Directory    : %s\n", mt4DirFlag)
	fmt.Printf("  • Minimum Impact      : %s\n", imp)
	fmt.Printf("  • Sync Interval       : %v\n", bridgeIntervalFlag)
	if len(currs) > 0 {
		fmt.Printf("  • Currency Filter     : %s\n", strings.Join(currs, ", "))
	} else {
		fmt.Printf("  • Currency Filter     : ALL\n")
	}
	fmt.Printf("  • Published Files     : ff_news_filter.json & ff_news_filter.csv\n")
	fmt.Printf("\033[1;36m================================================================================\033[0m\n")
	fmt.Println("Bridge active and synchronizing with MT4/MT5... Press Ctrl+C to terminate.")

	b := bridge.NewBridge(bridge.BridgeConfig{
		OutputDir:  mt4DirFlag,
		MinImpact:  imp,
		Interval:   bridgeIntervalFlag,
		Currencies: currs,
		Cookie:     cookieFlag,
		Headless:   headlessFlag,
	})

	if err := b.Start(context.Background()); err != nil && err != context.Canceled {
		log.Fatalf("Bridge error: %v", err)
	}
}

func executeServe() {
	addr := ":" + portFlag
	if strings.Contains(portFlag, ":") {
		addr = portFlag
	}

	fmt.Printf("\033[1;36m================================================================================\033[0m\n")
	fmt.Printf("\033[1;32m  FOREXFACTORY-GO REST & SSE MICROSERVICE SERVER\033[0m\n")
	fmt.Printf("\033[1;36m================================================================================\033[0m\n")
	fmt.Printf("  • Listening on        : http://localhost%s\n", addr)
	fmt.Printf("  • Health Check        : GET  http://localhost%s/health\n", addr)
	fmt.Printf("  • Calendar API        : GET  http://localhost%s/api/v1/calendar?start=YYYY-MM-DD&end=YYYY-MM-DD\n", addr)
	fmt.Printf("  • Event Detail API    : GET  http://localhost%s/api/v1/event?id=123456\n", addr)
	fmt.Printf("  • Live Feed API       : GET  http://localhost%s/api/v1/live\n", addr)
	fmt.Printf("  • SSE Live Stream     : GET  http://localhost%s/api/v1/stream\n", addr)
	fmt.Printf("  • Server Stats        : GET  http://localhost%s/api/v1/stats\n", addr)
	fmt.Printf("\033[1;36m================================================================================\033[0m\n")
	fmt.Println("Ready to serve requests... Press Ctrl+C to terminate.")

	srv := server.NewServer(server.Config{
		Addr:        addr,
		RateLimit:   rateLimitFlag,
		Concurrency: concurrencyFlag,
		Cookie:      cookieFlag,
		Headless:    headlessFlag,
	})

	if err := srv.Start(); err != nil && err != http.ErrServerClosed {
		log.Fatalf("Server error: %v", err)
	}
}

func main() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}

func executeEvent(args []string) {
	targetID := eventIDFlag
	if len(args) > 0 {
		targetID = args[0]
	}
	if targetID == "" {
		log.Fatalf("Error: Event ID is required. Usage: forexfactory-go event <id> or --id <id>")
	}

	var clientOpts []forexfactory.Option
	if cookieFlag != "" {
		clientOpts = append(clientOpts, forexfactory.WithHeader("Cookie", cookieFlag))
	}
	clientOpts = append(clientOpts, forexfactory.WithHeadless(headlessFlag))

	client := forexfactory.NewClient(clientOpts...)
	defer client.Close()

	if !silentFlag {
		fmt.Fprintf(os.Stderr, "Fetching event specifications and release history for ID %s...\n\n", targetID)
	}

	detail, err := client.FetchEventDetail(context.Background(), targetID)
	if err != nil {
		log.Fatalf("Failed to fetch event detail: %v", err)
	}

	// Print Event Specs Card
	fmt.Printf("\033[1;36m================================================================================\033[0m\n")
	title := detail.Title
	if title == "" {
		title = fmt.Sprintf("Event #%s", detail.ID)
	}
	fmt.Printf("\033[1;32m  EVENT: %s\033[0m (ID: %s)\n", title, detail.ID)
	fmt.Printf("\033[1;36m================================================================================\033[0m\n")
	if detail.Source != "" {
		fmt.Printf("  • Source          : %s\n", detail.Source)
	}
	if detail.Measures != "" {
		fmt.Printf("  • Measures        : %s\n", detail.Measures)
	}
	if detail.UsualEffect != "" {
		fmt.Printf("  • Usual Effect    : %s\n", detail.UsualEffect)
	}
	if detail.Frequency != "" {
		fmt.Printf("  • Frequency       : %s\n", detail.Frequency)
	}
	if detail.NextRelease != "" {
		fmt.Printf("  • Next Release    : %s\n", detail.NextRelease)
	}
	if detail.WhyTradersCare != "" {
		fmt.Printf("  • Why Traders Care: %s\n", detail.WhyTradersCare)
	}
	fmt.Printf("\033[1;36m--------------------------------------------------------------------------------\033[0m\n")

	// Print History Table
	if len(detail.History) > 0 {
		fmt.Printf("\n\033[1mHISTORICAL RELEASES (%d past records):\033[0m\n\n", len(detail.History))
		table := tablewriter.NewWriter(os.Stdout)
		table.SetHeader([]string{"Date", "Actual", "Forecast", "Previous"})
		table.SetBorders(tablewriter.Border{Left: true, Top: false, Right: true, Bottom: false})
		table.SetCenterSeparator("|")

		for _, h := range detail.History {
			dateStr := h.Date.Format("2006-01-02")
			if h.Date.IsZero() {
				dateStr = "-"
			}
			table.Append([]string{dateStr, h.Actual, h.Forecast, h.Previous})
		}
		table.Render()
	}
}

func fetchEventsConcurrently(startDate, endDate time.Time, targetLoc *time.Location) []forexfactory.Event {
	var clientOpts []forexfactory.Option

	if timezoneFlag != "" && targetLoc != nil {
		clientOpts = append(clientOpts, forexfactory.WithTimeLocation(targetLoc))
	}
	if rateLimitFlag > 0 {
		clientOpts = append(clientOpts, forexfactory.WithRateLimit(rateLimitFlag))
	}
	if concurrencyFlag > 0 {
		clientOpts = append(clientOpts, forexfactory.WithConcurrency(concurrencyFlag))
	}
	if cookieFlag != "" {
		clientOpts = append(clientOpts, forexfactory.WithHeader("Cookie", cookieFlag))
	}
	clientOpts = append(clientOpts, forexfactory.WithHeadless(headlessFlag))

	if !silentFlag {
		// Dynamic visual progress callback
		fmt.Fprintf(os.Stderr, "Downloading calendar data via %d concurrent workers...\n", concurrencyFlag)
		clientOpts = append(clientOpts, forexfactory.WithProgressCallback(func(current, total int) {
			if total <= 0 {
				return
			}
			pct := current * 100 / total
			barLen := 30
			filledLen := current * barLen / total
			bar := strings.Repeat("█", filledLen) + strings.Repeat("░", barLen-filledLen)
			fmt.Fprintf(os.Stderr, "\r[%s] %d%% Completed (%d/%d weeks)", bar, pct, current, total)
		}))
	}

	client := forexfactory.NewClient(clientOpts...)
	defer client.Close()

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

	format := strings.ToLower(strings.TrimSpace(formatFlag))
	if format == "parquet" {
		if outputFlag == "" {
			log.Fatalf("Error: --output file path is required when exporting to parquet format")
		}
		if err := forexfactory.WriteParquet(outputFlag, filteredEvents); err != nil {
			log.Fatalf("Error writing Parquet: %v", err)
		}
		if !silentFlag {
			fmt.Fprintf(os.Stderr, "Successfully exported %d events to parquet.\n", len(filteredEvents))
		}
		return
	}

	if format == "xlsx" || format == "excel" {
		if outputFlag == "" {
			log.Fatalf("Error: --output file path is required when exporting to xlsx format")
		}
		if err := forexfactory.WriteExcel(filteredEvents, outputFlag); err != nil {
			log.Fatalf("Error writing Excel: %v", err)
		}
		if !silentFlag {
			fmt.Fprintf(os.Stderr, "Successfully exported %d events to Excel (.xlsx).\n", len(filteredEvents))
		}
		return
	}

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
		log.Fatalf("Unknown output format %q: use 'json', 'csv', 'parquet', or 'xlsx'", formatFlag)
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

	now := time.Now().UTC()
	var nextUpcoming *forexfactory.Event
	var shortestDiff time.Duration

	for _, e := range events {
		if e.Impact == forexfactory.ImpactHigh || e.Impact == forexfactory.ImpactMedium {
			diff := e.Date.UTC().Sub(now)
			if diff > 0 {
				if nextUpcoming == nil || diff < shortestDiff {
					copyEvt := e
					nextUpcoming = &copyEvt
					shortestDiff = diff
				}
			}
		}
	}

	fmt.Println()
	fmt.Printf("\033[1;36m=== FOREX FACTORY LIVE WEEKLY ECONOMIC CALENDAR (%s) ===\033[0m\n", tzName)

	if nextUpcoming != nil {
		hours := int(shortestDiff.Hours())
		mins := int(shortestDiff.Minutes()) % 60
		secs := int(shortestDiff.Seconds()) % 60
		emoji := "🔴"
		if nextUpcoming.Impact == forexfactory.ImpactMedium {
			emoji = "🟡"
		}
		fmt.Printf("\033[1;33m  ⏳ NEXT EVENT:\033[0m %s \033[1m[%s]\033[0m \033[1;37m%s\033[0m in \033[1;31m%02dh %02dm %02ds\033[0m (at %s)\n",
			emoji,
			nextUpcoming.Currency,
			nextUpcoming.Title,
			hours, mins, secs,
			nextUpcoming.Date.UTC().Format("15:04 UTC"),
		)
	}

	table := tablewriter.NewWriter(os.Stdout)
	table.SetHeader([]string{"Date & Time", "Currency", "Impact", "Haber / Gelişme", "Actual", "Forecast", "Previous"})
	table.SetAutoWrapText(true)
	table.SetBorder(true)
	table.SetCenterSeparator("┼")
	table.SetColumnSeparator("│")
	table.SetRowSeparator("─")

	// Header Styling: Cyan headers
	cyanHeader := tablewriter.Colors{tablewriter.Bold, tablewriter.FgCyanColor}
	table.SetHeaderColor(cyanHeader, cyanHeader, cyanHeader, cyanHeader, cyanHeader, cyanHeader, cyanHeader)

	for _, e := range events {
		timeStr := e.Date.Format("2006-01-02 15:04")
		if e.IsAllDay {
			timeStr = e.Date.Format("2006-01-02") + " (All Day)"
		} else if e.IsTentative {
			timeStr = e.Date.Format("2006-01-02") + " (Tentative)"
		}

		var impactStr string
		var impactColor tablewriter.Colors
		switch e.Impact {
		case forexfactory.ImpactHigh:
			impactStr = "🔴 HIGH"
			impactColor = tablewriter.Colors{tablewriter.Bold, tablewriter.FgRedColor}
		case forexfactory.ImpactMedium:
			impactStr = "🟡 MEDIUM"
			impactColor = tablewriter.Colors{tablewriter.Bold, tablewriter.FgYellowColor}
		case forexfactory.ImpactLow:
			impactStr = "🟢 LOW"
			impactColor = tablewriter.Colors{tablewriter.Bold, tablewriter.FgGreenColor}
		default:
			impactStr = "⚪ NONE"
			impactColor = tablewriter.Colors{tablewriter.FgHiBlackColor}
		}

		act := e.Actual
		if act == "" {
			act = "-"
		}
		forc := e.Forecast
		if forc == "" {
			forc = "-"
		}
		prev := e.Previous
		if prev == "" {
			prev = "-"
		}

		// Premium quant feature: compare Actual and Forecast dynamically to color cells!
		actColor := tablewriter.Colors{}
		if act != "-" && forc != "-" {
			actualVal, err1 := forexfactory.ParseFloat(act)
			forecastVal, err2 := forexfactory.ParseFloat(forc)
			if err1 == nil && err2 == nil {
				if actualVal > forecastVal {
					actColor = tablewriter.Colors{tablewriter.Bold, tablewriter.FgGreenColor}
				} else if actualVal < forecastVal {
					actColor = tablewriter.Colors{tablewriter.Bold, tablewriter.FgRedColor}
				}
			}
		}

		table.Rich([]string{
			timeStr,
			e.Currency,
			impactStr,
			e.Title,
			act,
			forc,
			prev,
		}, []tablewriter.Colors{
			{}, // Time
			{tablewriter.Bold, tablewriter.FgHiCyanColor}, // Currency
			impactColor,        // Impact
			{tablewriter.Bold}, // Title
			actColor,           // Actual (beats: green, misses: red)
			{},                 // Forecast
			{},                 // Previous
		})
	}

	table.Render()
	fmt.Println()
}

func writeCSV(w io.Writer, events []forexfactory.Event) error {
	writer := csv.NewWriter(w)
	defer writer.Flush()

	err := writer.Write([]string{"id", "title", "currency", "date", "impact", "forecast", "previous", "actual", "all_day", "tentative"})
	if err != nil {
		return err
	}

	for _, e := range events {
		err = writer.Write([]string{
			e.ID,
			e.Title,
			e.Currency,
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
