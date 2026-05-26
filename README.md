# forexfactory-go

[![Go Reference](https://pkg.go.dev/badge/github.com/Nosvemos/forexfactory-go.svg)](https://pkg.go.dev/github.com/Nosvemos/forexfactory-go)
[![Go Report Card](https://goreportcard.com/badge/github.com/Nosvemos/forexfactory-go)](https://goreportcard.com/report/github.com/Nosvemos/forexfactory-go)
[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](https://opensource.org/licenses/MIT)

A robust, enterprise-grade Go library and premium command-line tool (CLI) to scrape and stream economic calendar data from **Forex Factory** (`https://www.forexfactory.com/calendar`). Designed to fetch both historical data and stream live economic events seamlessly.

Highly inspired by modular, high-performance data retrieval designs (like `dukascopy-go`), this module is designed for performance, concurrent scraping, customizability, rate-limit safety, and absolute ease of use.

---

## Features

- **Historical Calendar Scraping**: Fetch weeks/days of historical economic calendar events.
- **Concurrent Worker Pool Scraper**: Parallelizes historical downloading across multiple worker goroutines, delivering up to **Nx speed increases** for large-scale range fetches.
- **Cobra CLI Integration**: Implements a high-quality CLI utilizing the industry-standard **Cobra** framework, featuring nested subcommands, robust flag handling, and automatic help page generation.
- **Custom ASCII Progress Indicator**: Renders highly interactive, beautiful, and responsive progress tracking directly in your terminal.
- **Cloudflare Bypass Support**: Features a `--cookie` flag (and client option `WithHeader`) to directly accept clearance cookies (`cf_clearance`, etc.) to bypass strict anti-bot protections.
- **Real-Time/Live Event Streaming**: Stream current week events directly from a lightweight XML feed, bypassing heavy HTML/Cloudflare overhead.
- **Robust Rate Limiting**: Built-in, thread-safe rate-limit controls to prevent IP blocks.
- **Multi-Timezone Support**: Automatically shift event times to your local timezone or any specified timezone.

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

The executable provides two highly modular commands: `download` and `live`.

### 1. Download Historical Calendar Data
Download calendar events within a specific date range as a compact CSV or JSON file concurrently.

```bash
# Download a month of data concurrently using 4 workers, exporting to a CSV file
forexfactory download --start 2026-05-01 --end 2026-05-31 --concurrency 4 --format csv --output calendar_may26.csv

# Download historical events as JSON using a custom timezone and Cloudflare bypass cookies
forexfactory download -s 2026-01-01 -e 2026-01-31 --timezone "Europe/Istanbul" --cookie "cf_clearance=your_cf_clearance_value" --format json
```

**Flags**:
- `-s, --start`: Start date (`YYYY-MM-DD`). *Required*.
- `-e, --end`: End date (`YYYY-MM-DD`). *Required*.
- `-f, --format`: Output format: `json` or `csv` (default: `json`).
- `-o, --output`: File path to write data (default: prints to stdout).
- `-t, --timezone`: Target timezone (e.g. `America/New_York`, `UTC`, `Local`).
- `-c, --concurrency`: Number of concurrent downloading worker threads (default: `3`).
- `-r, --rate-limit`: Maximum HTTP requests per second allowed per worker (default: `1`).
- `--cookie`: Custom Cloudflare clearance cookie to bypass anti-bot blocks.

### 2. Live Calendar Streaming / Watching
Monitor the current week's economic events in real time.

```bash
# Watch live economic events, polling every 60 seconds
forexfactory live --interval 60s --timezone Local
```

**Flags**:
- `-i, --interval`: Polling interval for live streaming (e.g., `60s`, `5m`). If `0`, fetches once and exits.
- `-t, --timezone`: Target timezone (e.g. `America/New_York`, `Local`).
- `--cookie`: Custom Cloudflare clearance cookie if the feed rate-limits or blocks.

---

## Library Usage Examples

### Fetching Calendar Data
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
		forexfactory.WithTimeLocation(time.Local), // Convert event times to local timezone
		forexfactory.WithHeader("Cookie", "cf_clearance=your_cookie_here"), // Bypass Cloudflare
	)

	// Fetch a specific week's events
	targetDate := time.Date(2026, time.May, 26, 0, 0, 0, 0, time.UTC)
	events, err := client.FetchWeek(context.Background(), targetDate)
	if err != nil {
		log.Fatalf("Failed to fetch events: %v", err)
	}

	for _, event := range events {
		fmt.Printf("[%s] %s (%s) - Impact: %s | Actual: %s, Forecast: %s\n",
			event.Date.Format("2006-01-02 15:04"),
			event.Title,
			event.Country,
			event.Impact,
			event.Actual,
			event.Forecast,
		)
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
