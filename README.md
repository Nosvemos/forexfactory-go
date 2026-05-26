# forexfactory-go

[![Go Reference](https://pkg.go.dev/badge/github.com/Nosvemos/forexfactory-go.svg)](https://pkg.go.dev/github.com/Nosvemos/forexfactory-go)
[![Go Report Card](https://goreportcard.com/badge/github.com/Nosvemos/forexfactory-go)](https://goreportcard.com/report/github.com/Nosvemos/forexfactory-go)
[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](https://opensource.org/licenses/MIT)

A robust, premium Go library and CLI tool to scrape and stream economic calendar data from **Forex Factory** (`https://www.forexfactory.com/calendar`). Designed to fetch both historical data and stream live economic events seamlessly.

Highly inspired by modular data retrieval designs (like `dukascopy-go`), this module is designed for performance, customizability, rate-limit safety, and absolute ease of use.

---

## Features

- **Historical Calendar Scraping**: Fetch weeks/days of historical economic calendar events.
- **Real-Time/Live Event Streaming**: Stream current week events directly from a lightweight XML feed, bypassing heavy HTML/Cloudflare overhead.
- **Robust Rate Limiting**: Built-in, configurable rate-limit controls and random backoffs to prevent IP blocks.
- **Advanced Configuration**: Configure user agents, proxy lists, timeouts, and custom HTTP clients easily.
- **Multi-Timezone Support**: Automatically shift event times to your local timezone or any specified timezone.
- **Unified CLI Tool**: Quick commands to download historical events (CSV, JSON) or monitor live updates directly from your terminal.

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

The executable provides two simple commands: `download` and `live`.

### 1. Download Historical Calendar Data
Download calendar events within a specific date range as a compact CSV or JSON file.

```bash
# Download a week of data in CSV format
forexfactory download --start 2026-05-18 --end 2026-05-24 --format csv --output calendar_may26.csv

# Download historical events as JSON to stdout
forexfactory download --start 2026-01-01 --end 2026-01-31 --format json
```

**Options**:
- `--start`: Start date (`YYYY-MM-DD`). *Required*.
- `--end`: End date (`YYYY-MM-DD`). *Required*.
- `--format`: Output format, either `json` or `csv` (default: `json`).
- `--output`: File path to write data (default: prints to stdout).
- `--timezone`: Target timezone (e.g., `America/New_York`, `Europe/Istanbul`) (default: Local / ForexFactory default).
- `--rate-limit`: Maximum HTTP requests per second (default: `1`).

### 2. Live Calendar Streaming / Watching
Monitor the current week's economic events in real time.

```bash
# Watch live economic events, polling every 60 seconds
forexfactory live --interval 60s
```

---

## Library Usage Examples

### Simple Usage
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
		forexfactory.WithRateLimit(1), // 1 request per second
		forexfactory.WithTimeLocation(time.Local), // Convert event times to local time
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
| `WithHTTPClient(client *http.Client)` | Use a custom HTTP client (e.g., with proxy/ja3/cookies) | `WithHTTPClient(&http.Client{Timeout: 10 * time.Second})` |
| `WithUserAgent(ua string)` | Rotate or set custom user agent headers | `WithUserAgent("CustomAgent/1.0")` |
| `WithProxy(proxyURL string)` | Direct requests through a proxy | `WithProxy("http://user:pass@host:port")` |
| `WithRateLimit(limit int)` | Set maximum queries per second | `WithRateLimit(2)` |
| `WithTimeLocation(loc *time.Location)`| Shift parsed datetimes to target location | `WithTimeLocation(time.UTC)` |

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
