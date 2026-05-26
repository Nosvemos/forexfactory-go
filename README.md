# forexfactory-go

[![Go Reference](https://pkg.go.dev/badge/github.com/Nosvemos/forexfactory-go.svg)](https://pkg.go.dev/github.com/Nosvemos/forexfactory-go)
[![Go Report Card](https://goreportcard.com/badge/github.com/Nosvemos/forexfactory-go)](https://goreportcard.com/report/github.com/Nosvemos/forexfactory-go)
[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](https://opensource.org/licenses/MIT)

A robust, enterprise-grade Go library and premium command-line tool (CLI) to scrape, stream, and query economic calendar data from **Forex Factory** (`https://www.forexfactory.com/calendar`). Designed to fetch both historical data and stream live economic events seamlessly.

Highly inspired by modular, high-performance tick-data downloader designs (like `dukascopy-go`), this module is designed for concurrent scraping, customizability, rate-limit safety, and database-ready storage querying.

---

## Features

- **Historical Range Downloader**: Fetch events concurrently across broad date ranges via a built-in worker pool.
- **Real-Time XML Stream Tracker**: Pull the current week's events from a lightweight, low-overhead XML stream.
- **Dual-Use Storage SDK**: Decoupled persistence interface featuring high-speed bulk insertions and rich range/currency query methods using a CGO-free, **WAL (Write-Ahead Logging) optimized SQLite driver**.
- **Programmatic Hooks & Custom Options**: Exposes thread-safe rate-limiting, custom proxy bindings, and progress update listeners (`ProgressCallback`) for seamless integrations in web and background applications.
- **Automated Cloudflare Resolution**: Integrated fallback mechanics utilizing headless Chromium drivers to seamlessly navigate Cloudflare anti-bot verification.
- **Timezone Standardization**: Shifting calendar datetimes to any target location (e.g. UTC, local, or financial center zones).

---

## Installation

### As a Library
To import and use `forexfactory-go` in your Go project:

```bash
go get github.com/Nosvemos/forexfactory-go
```

### As a CLI Tool
To install the CLI tool directly to your `$GOPATH/bin`:

```bash
go install github.com/Nosvemos/forexfactory-go/cmd/forexfactory@latest
```

---

## CLI Usage

The executable provides three modular commands: `download`, `dbload`, and `live`.

### 1. Download Historical Calendar Data
Download calendar events within a specific date range concurrently to a JSON or CSV file.

```bash
# Download a month of data concurrently using 4 workers, exporting to a CSV file
forexfactory download --start 2026-05-01 --end 2026-05-31 --concurrency 4 --format csv --output calendar_may26.csv

# Download historical events as JSON using Europe/Istanbul timezone and custom cookies
forexfactory download -s 2026-01-01 -e 2026-01-31 --timezone "Europe/Istanbul" --cookie "cf_clearance=your_cf_clearance" --format json
```

### 2. Download and Load directly into SQLite Database
```bash
# Downloads and load historical events directly inside a CGO-free local SQLite database
forexfactory dbload --start 2026-05-01 --end 2026-05-31 --db forexfactory.db
```

### 3. Live Economic Feed Polling
```bash
# Watch live economic events, polling every 60 seconds
forexfactory live --interval 60s --timezone Local
```

---

## Library Usage Examples

### 1. Programmatic Range Downloading with Progress Updates
This example shows how to configure a code-controlled concurrent downloader client using rate limiters and progress tracking.

```go
package main

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/Nosvemos/forexfactory-go/pkg/forexfactory"
)

func main() {
	// Initialize a client with custom options
	client := forexfactory.NewClient(
		forexfactory.WithRateLimit(2),             // Cap at 2 requests per second
		forexfactory.WithConcurrency(4),           // Run 4 concurrent downloading worker threads
		forexfactory.WithTimeLocation(time.Local), // Convert event times to local timezone
		forexfactory.WithProgressCallback(func(current, total int) {
			fmt.Printf("Download progress: %d/%d weeks completed\n", current, total)
		}),
	)

	start := time.Date(2026, time.May, 1, 0, 0, 0, 0, time.UTC)
	end := time.Date(2026, time.May, 20, 23, 59, 59, 0, time.UTC)

	// Fetch a specific range's events concurrently
	events, err := client.FetchRange(context.Background(), start, end)
	if err != nil {
		log.Fatalf("Failed to fetch events: %v", err)
	}

	fmt.Printf("Successfully fetched %d events!\n", len(events))
}
```

### 2. Programmatic Database Persistence and Querying
You can easily write and query data through the storage driver:

```go
package main

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/Nosvemos/forexfactory-go/pkg/storage"
)

func main() {
	ctx := context.Background()
	store := storage.NewSQLiteStorage("forexfactory.db")

	if err := store.Init(ctx); err != nil {
		log.Fatalf("Database initialization failed: %v", err)
	}
	defer store.Close()

	// Query events falling in a date range
	start := time.Date(2026, time.May, 1, 0, 0, 0, 0, time.UTC)
	end := time.Date(2026, time.May, 20, 23, 59, 59, 0, time.UTC)

	events, err := store.GetEvents(ctx, start, end)
	if err != nil {
		log.Fatalf("Failed to query events: %v", err)
	}

	// Query events by currency/country
	usdEvents, err := store.GetEventsByCountry(ctx, "USD")
	if err != nil {
		log.Fatalf("Failed to query USD events: %v", err)
	}

	for _, e := range usdEvents {
		fmt.Printf("[%s] USD %s (Impact: %s)\n", e.Date.Format("2006-01-02"), e.Title, e.Impact)
	}
}
```

---

## Configuration Options

When initializing the client with `forexfactory.NewClient(...)`, you can pass various functional options:

| Option | Description | Example |
| :--- | :--- | :--- |
| `WithHTTPClient(client *http.Client)` | Use a custom HTTP client | `WithHTTPClient(&http.Client{Timeout: 10 * time.Second})` |
| `WithUserAgent(ua string)` | Rotate or set custom user agent headers | `WithUserAgent("CustomAgent/1.0")` |
| `WithProxy(proxyURL string)` | Direct requests through a proxy | `WithProxy("socks5://127.0.0.1:9050")` |
| `WithRateLimit(limit int)` | Set maximum queries per second | `WithRateLimit(2)` |
| `WithConcurrency(workers int)` | Set the concurrent workers for range fetching | `WithConcurrency(4)` |
| `WithProgressCallback(fn)` | Attach a custom progress listener | `WithProgressCallback(func(c, t int) {})` |
| `WithTimeLocation(loc *time.Location)`| Shift parsed datetimes to target location | `WithTimeLocation(time.UTC)` |
| `WithHeader(key, value string)`| Inject custom header (like `Cookie`, `Referer`, `Authorization`) | `WithHeader("Cookie", "cf_clearance=...")` |

---

## Contributing

1. Fork the repository
2. Create your feature branch (`git checkout -b feature/cool-feature`)
3. Commit your changes (`git commit -m 'Add cool feature'`)
4. Push to the branch (`git push origin feature/cool-feature`)
5. Create a Pull Request

---

## License

This project is licensed under the MIT License - see the [LICENSE](LICENSE) file for details.
