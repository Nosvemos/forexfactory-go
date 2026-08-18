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
	"strings"
	"time"

	"github.com/Nosvemos/tradingview-calendar-go/pkg/bridge"
	"github.com/Nosvemos/tradingview-calendar-go/pkg/server"
	"github.com/Nosvemos/tradingview-calendar-go/pkg/storage"
	"github.com/Nosvemos/tradingview-calendar-go/pkg/tvcalendar"
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
	currenciesFlag  string
	countriesFlag   string
	impactsFlag     string

	// Flags for serve command
	portFlag string

	// Flags for bridge command
	mt4DirFlag         string
	minImpactFlag      string
	bridgeIntervalFlag time.Duration

	// Flags for live command
	intervalFlag time.Duration
	liveTimeLoc  string

	// Flags for dbload command
	dbFlag string

	// Global silent flag
	silentFlag bool
)

var rootCmd = &cobra.Command{
	Use:   "tvcalendar",
	Short: "Lightning-fast, Pure HTTP TradingView Economic Calendar CLI and Microservice",
	Long: `tradingview-calendar-go is a high-performance Go CLI and library designed to fetch, 
stream, and analyze global macroeconomic calendar data (12+ years history, 0 Cloudflare blocks, pure HTTP).`,
}

var downloadCmd = &cobra.Command{
	Use:   "download",
	Short: "Download historical and upcoming economic calendar data",
	Long: `Concurrently downloads historical economic calendar data across any date range (e.g. 10+ years)
and exports it to compact, chronologically sorted CSV, JSON, Parquet, or Excel (XLSX) files.`,
	Run: func(cmd *cobra.Command, args []string) {
		executeDownload()
	},
}

var serveCmd = &cobra.Command{
	Use:   "serve",
	Short: "Launch high-performance REST API and SSE streaming microservice",
	Long:  `Starts an embedded HTTP server exposing RESTful endpoints (/api/v1/calendar, /api/v1/live, /api/v1/stream) and live Server-Sent Events.`,
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
	Short: "Concurrently fetch economic events and bulk-insert into a SQLite database",
	Long:  `Downloads calendar events across a date range and inserts/updates them inside a local SQLite database using WAL mode and high-speed batch transactions.`,
	Run: func(cmd *cobra.Command, args []string) {
		executeDbLoad()
	},
}

var liveCmd = &cobra.Command{
	Use:   "live",
	Short: "Watch live economic calendar events with continuous updates",
	Long:  `Displays an interactive, real-time terminal dashboard of the current week's upcoming and recently published economic events.`,
	Run: func(cmd *cobra.Command, args []string) {
		executeLive()
	},
}

var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "Print the application version",
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Println("tradingview-calendar-go version 3.0.0 (Pure HTTP, 12+ Year Historical Depth)")
	},
}

func init() {
	// Download flags
	downloadCmd.Flags().StringVarP(&startFlag, "start", "s", "", "Start date in YYYY-MM-DD format (Required)")
	downloadCmd.Flags().StringVarP(&endFlag, "end", "e", "", "End date in YYYY-MM-DD format (Required)")
	downloadCmd.Flags().StringVarP(&formatFlag, "format", "f", "json", "Output format: 'json', 'csv', 'parquet', or 'xlsx'")
	downloadCmd.Flags().StringVarP(&outputFlag, "output", "o", "", "Output file path (defaults to stdout)")
	downloadCmd.Flags().StringVarP(&timezoneFlag, "timezone", "t", "", "Target timezone for event times (e.g. 'UTC', 'America/New_York')")
	downloadCmd.Flags().IntVarP(&rateLimitFlag, "rate-limit", "r", 10, "Maximum HTTP requests per second allowed")
	downloadCmd.Flags().IntVarP(&concurrencyFlag, "concurrency", "c", 5, "Number of concurrent downloading worker threads")
	downloadCmd.Flags().StringVar(&currenciesFlag, "currency", "", "Comma-separated currency filter (e.g. 'USD,EUR,GBP')")
	downloadCmd.Flags().StringVar(&countriesFlag, "country", "", "Comma-separated ISO country filter (e.g. 'US,EU,GB')")
	downloadCmd.Flags().StringVar(&impactsFlag, "impact", "", "Comma-separated impact filter: 'High,Medium,Low'")
	downloadCmd.Flags().BoolVarP(&silentFlag, "silent", "q", false, "Mute all logging and progress outputs")
	_ = downloadCmd.MarkFlagRequired("start")
	_ = downloadCmd.MarkFlagRequired("end")

	// Serve flags
	serveCmd.Flags().StringVarP(&portFlag, "port", "p", "8080", "Port or address to bind HTTP API server")
	serveCmd.Flags().IntVarP(&rateLimitFlag, "rate-limit", "r", 10, "Maximum requests per second allowed")
	serveCmd.Flags().IntVarP(&concurrencyFlag, "concurrency", "c", 5, "Internal worker concurrency")

	// Bridge flags
	bridgeCmd.Flags().StringVar(&mt4DirFlag, "mt4-dir", "", "Path to MetaTrader MQL4/MQL5 Files folder (Required)")
	bridgeCmd.Flags().StringVar(&minImpactFlag, "min-impact", "High", "Minimum impact level to filter ('High', 'Medium', 'Low')")
	bridgeCmd.Flags().StringVar(&currenciesFlag, "currencies", "", "Comma-separated currencies to include (e.g. 'USD,EUR,GBP')")
	bridgeCmd.Flags().DurationVar(&bridgeIntervalFlag, "interval", 1*time.Minute, "Polling synchronization frequency")
	bridgeCmd.Flags().StringVarP(&timezoneFlag, "timezone", "t", "UTC", "Timezone for exported timestamp fields")
	_ = bridgeCmd.MarkFlagRequired("mt4-dir")

	// DBLoad flags
	dbloadCmd.Flags().StringVarP(&startFlag, "start", "s", "", "Start date in YYYY-MM-DD format (Required)")
	dbloadCmd.Flags().StringVarP(&endFlag, "end", "e", "", "End date in YYYY-MM-DD format (Required)")
	dbloadCmd.Flags().StringVar(&dbFlag, "db", "tvcalendar.db", "Target SQLite database file path")
	dbloadCmd.Flags().StringVarP(&timezoneFlag, "timezone", "t", "", "Target timezone for events")
	dbloadCmd.Flags().IntVarP(&rateLimitFlag, "rate-limit", "r", 10, "Maximum HTTP requests per second allowed")
	dbloadCmd.Flags().IntVarP(&concurrencyFlag, "concurrency", "c", 5, "Number of concurrent downloading worker threads")
	dbloadCmd.Flags().BoolVarP(&silentFlag, "silent", "q", false, "Mute all logging and progress outputs")
	_ = dbloadCmd.MarkFlagRequired("start")
	_ = dbloadCmd.MarkFlagRequired("end")

	// Live flags
	liveCmd.Flags().DurationVarP(&intervalFlag, "interval", "i", 0, "Polling interval for updates (e.g., '10s', '1m'). If 0, fetches once and exits")
	liveCmd.Flags().StringVarP(&liveTimeLoc, "timezone", "t", "UTC", "Display timezone for event times")

	rootCmd.AddCommand(downloadCmd)
	rootCmd.AddCommand(serveCmd)
	rootCmd.AddCommand(bridgeCmd)
	rootCmd.AddCommand(dbloadCmd)
	rootCmd.AddCommand(liveCmd)
	rootCmd.AddCommand(versionCmd)
}

func main() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}

func parseFilters() ([]tvcalendar.Impact, []string, []string) {
	var impacts []tvcalendar.Impact
	if impactsFlag != "" {
		for _, imp := range strings.Split(impactsFlag, ",") {
			switch strings.ToLower(strings.TrimSpace(imp)) {
			case "high":
				impacts = append(impacts, tvcalendar.ImpactHigh)
			case "medium":
				impacts = append(impacts, tvcalendar.ImpactMedium)
			case "low":
				impacts = append(impacts, tvcalendar.ImpactLow)
			case "none":
				impacts = append(impacts, tvcalendar.ImpactNone)
			}
		}
	}

	var currencies []string
	if currenciesFlag != "" {
		for _, c := range strings.Split(currenciesFlag, ",") {
			if trimmed := strings.ToUpper(strings.TrimSpace(c)); trimmed != "" {
				currencies = append(currencies, trimmed)
			}
		}
	}

	var countries []string
	if countriesFlag != "" {
		for _, cnt := range strings.Split(countriesFlag, ",") {
			if trimmed := strings.ToUpper(strings.TrimSpace(cnt)); trimmed != "" {
				countries = append(countries, trimmed)
			}
		}
	}

	return impacts, currencies, countries
}

func fetchEventsConcurrently(startDate, endDate time.Time, targetLoc *time.Location) []tvcalendar.Event {
	var clientOpts []tvcalendar.Option

	if timezoneFlag != "" && targetLoc != nil {
		clientOpts = append(clientOpts, tvcalendar.WithTimeLocation(targetLoc))
	}
	if rateLimitFlag > 0 {
		clientOpts = append(clientOpts, tvcalendar.WithRateLimit(rateLimitFlag))
	}
	if concurrencyFlag > 0 {
		clientOpts = append(clientOpts, tvcalendar.WithConcurrency(concurrencyFlag))
	}

	impacts, currencies, countries := parseFilters()
	if len(impacts) > 0 {
		clientOpts = append(clientOpts, tvcalendar.WithImpactFilter(impacts...))
	}
	if len(currencies) > 0 {
		clientOpts = append(clientOpts, tvcalendar.WithCurrencyFilter(currencies...))
	}
	if len(countries) > 0 {
		clientOpts = append(clientOpts, tvcalendar.WithCountryFilter(countries...))
	}

	if !silentFlag {
		fmt.Fprintf(os.Stderr, "Downloading calendar data via %d concurrent workers...\n", concurrencyFlag)
		clientOpts = append(clientOpts, tvcalendar.WithProgressCallback(func(current, total int) {
			if total <= 0 {
				return
			}
			pct := current * 100 / total
			barLen := 30
			filledLen := current * barLen / total
			bar := strings.Repeat("█", filledLen) + strings.Repeat("░", barLen-filledLen)
			fmt.Fprintf(os.Stderr, "\r[%s] %d%% Completed (%d/%d months)", bar, pct, current, total)
		}))
	}

	client := tvcalendar.NewClient(clientOpts...)
	defer client.Close()

	events, err := client.FetchRange(context.Background(), startDate, endDate)
	if err != nil {
		if !silentFlag {
			fmt.Fprintln(os.Stderr)
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
		if err := tvcalendar.WriteParquet(outputFlag, filteredEvents); err != nil {
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
		if err := tvcalendar.WriteExcel(filteredEvents, outputFlag); err != nil {
			log.Fatalf("Error writing Excel: %v", err)
		}
		if !silentFlag {
			fmt.Fprintf(os.Stderr, "Successfully exported %d events to Excel (.xlsx).\n", len(filteredEvents))
		}
		return
	}

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

func executeServe() {
	addr := ":" + portFlag
	if strings.Contains(portFlag, ":") {
		addr = portFlag
	}

	fmt.Printf("\033[1;36m================================================================================\033[0m\n")
	fmt.Printf("\033[1;32m  TRADINGVIEW-CALENDAR REST & SSE MICROSERVICE SERVER\033[0m\n")
	fmt.Printf("\033[1;36m================================================================================\033[0m\n")
	fmt.Printf("  • Listening on        : http://localhost%s\n", addr)
	fmt.Printf("  • Health Check        : GET  http://localhost%s/health\n", addr)
	fmt.Printf("  • Calendar API        : GET  http://localhost%s/api/v1/calendar?start=YYYY-MM-DD&end=YYYY-MM-DD\n", addr)
	fmt.Printf("  • Live Feed API       : GET  http://localhost%s/api/v1/live\n", addr)
	fmt.Printf("  • SSE Live Stream     : GET  http://localhost%s/api/v1/stream\n", addr)
	fmt.Printf("  • Server Stats        : GET  http://localhost%s/api/v1/stats\n", addr)
	fmt.Printf("\033[1;36m================================================================================\033[0m\n")
	fmt.Println("Ready to serve requests... Press Ctrl+C to terminate.")

	srv := server.NewServer(server.Config{
		Addr:        addr,
		RateLimit:   rateLimitFlag,
		Concurrency: concurrencyFlag,
	})

	if err := srv.Start(); err != nil && err != http.ErrServerClosed {
		log.Fatalf("Server error: %v", err)
	}
}

func executeBridge() {
	var currs []string
	if currenciesFlag != "" {
		for _, c := range strings.Split(currenciesFlag, ",") {
			if trimmed := strings.ToUpper(strings.TrimSpace(c)); trimmed != "" {
				currs = append(currs, trimmed)
			}
		}
	}

	imp := tvcalendar.ImpactHigh
	switch strings.ToLower(minImpactFlag) {
	case "medium":
		imp = tvcalendar.ImpactMedium
	case "low":
		imp = tvcalendar.ImpactLow
	}

	fmt.Printf("\033[1;36m================================================================================\033[0m\n")
	fmt.Printf("\033[1;32m  TRADINGVIEW-CALENDAR METATRADER 4 / 5 NEWS FILTER BRIDGE\033[0m\n")
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
	})

	if err := b.Start(context.Background()); err != nil && err != context.Canceled {
		log.Fatalf("Bridge error: %v", err)
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

	filteredEvents := fetchEventsConcurrently(startDate, endDate, targetLoc)

	store := storage.NewSQLiteStorage(dbFlag)
	ctx := context.Background()

	if err := store.Init(ctx); err != nil {
		log.Fatalf("Failed to initialize database: %v", err)
	}
	defer store.Close()

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

	client := tvcalendar.NewClient(
		tvcalendar.WithTimeLocation(targetLoc),
	)

	if intervalFlag <= 0 {
		fetchAndPrintLive(client)
		return
	}

	ticker := time.NewTicker(intervalFlag)
	defer ticker.Stop()

	fetchAndPrintLive(client)
	fmt.Printf("\nWatching economic feed every %v. Press Ctrl+C to stop...\n", intervalFlag)

	for range ticker.C {
		fmt.Println("\n--- Updating Live Calendar ---")
		fetchAndPrintLive(client)
	}
}

func fetchAndPrintLive(client *tvcalendar.Client) {
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
	var nextUpcoming *tvcalendar.Event
	var shortestDiff time.Duration

	for _, e := range events {
		if e.Impact == tvcalendar.ImpactHigh || e.Impact == tvcalendar.ImpactMedium {
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
	fmt.Printf("\033[1;36m=== TRADINGVIEW ECONOMIC CALENDAR LIVE FEED (%s) ===\033[0m\n", tzName)

	if nextUpcoming != nil {
		hours := int(shortestDiff.Hours())
		mins := int(shortestDiff.Minutes()) % 60
		secs := int(shortestDiff.Seconds()) % 60
		emoji := "🔴"
		if nextUpcoming.Impact == tvcalendar.ImpactMedium {
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
	table.SetHeader([]string{"Date & Time", "Country", "Currency", "Impact", "Event Title", "Actual", "Forecast", "Previous"})
	table.SetAutoWrapText(true)
	table.SetBorder(true)
	table.SetCenterSeparator("┼")
	table.SetColumnSeparator("│")
	table.SetRowSeparator("─")

	cyanHeader := tablewriter.Colors{tablewriter.Bold, tablewriter.FgCyanColor}
	table.SetHeaderColor(cyanHeader, cyanHeader, cyanHeader, cyanHeader, cyanHeader, cyanHeader, cyanHeader, cyanHeader)

	for _, e := range events {
		timeStr := e.Date.Format("2006-01-02 15:04")
		if e.IsAllDay {
			timeStr = e.Date.Format("2006-01-02") + " (All Day)"
		}

		var impactStr string
		var impactColor tablewriter.Colors
		switch e.Impact {
		case tvcalendar.ImpactHigh:
			impactStr = "🔴 HIGH"
			impactColor = tablewriter.Colors{tablewriter.Bold, tablewriter.FgRedColor}
		case tvcalendar.ImpactMedium:
			impactStr = "🟡 MEDIUM"
			impactColor = tablewriter.Colors{tablewriter.Bold, tablewriter.FgYellowColor}
		case tvcalendar.ImpactLow:
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

		actColor := tablewriter.Colors{}
		if act != "-" && forc != "-" {
			actualVal, err1 := tvcalendar.ParseFloat(act)
			forecastVal, err2 := tvcalendar.ParseFloat(forc)
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
			e.Country,
			e.Currency,
			impactStr,
			e.Title,
			act,
			forc,
			prev,
		}, []tablewriter.Colors{
			{}, // Time
			{tablewriter.Bold},                           // Country
			{tablewriter.Bold, tablewriter.FgHiCyanColor}, // Currency
			impactColor,        // Impact
			{tablewriter.Bold}, // Title
			actColor,           // Actual
			{},                 // Forecast
			{},                 // Previous
		})
	}

	table.Render()
	fmt.Println()
}

func writeCSV(w io.Writer, events []tvcalendar.Event) error {
	writer := csv.NewWriter(w)
	defer writer.Flush()

	err := writer.Write([]string{"id", "title", "country", "currency", "date", "impact", "forecast", "previous", "actual", "unit", "category", "indicator", "source"})
	if err != nil {
		return err
	}

	for _, e := range events {
		err = writer.Write([]string{
			e.ID,
			e.Title,
			e.Country,
			e.Currency,
			e.Date.Format(time.RFC3339),
			string(e.Impact),
			e.Forecast,
			e.Previous,
			e.Actual,
			e.Unit,
			e.Category,
			e.Indicator,
			e.Source,
		})
		if err != nil {
			return err
		}
	}
	return nil
}

func writeJSON(w io.Writer, events []tvcalendar.Event) error {
	encoder := json.NewEncoder(w)
	encoder.SetIndent("", "  ")
	return encoder.Encode(events)
}
