# forexfactory-go 🚀

<div align="center">
  <h1>forexfactory-go 📊</h1>
  <p><b>The fastest, most robust, and enterprise-grade tool to concurrently scrape, stream, and persist Forex Factory economic calendar data — with automated Cloudflare bypass.</b></p>

  <p>
    <a href="https://pkg.go.dev/github.com/Nosvemos/forexfactory-go"><img src="https://pkg.go.dev/badge/github.com/Nosvemos/forexfactory-go.svg" alt="Go Reference"></a>
    <a href="https://goreportcard.com/report/github.com/Nosvemos/forexfactory-go"><img src="https://goreportcard.com/badge/github.com/Nosvemos/forexfactory-go" alt="Go Report Card"></a>
    <a href="https://opensource.org/licenses/MIT"><img src="https://img.shields.io/badge/License-MIT-yellow.svg" alt="License: MIT"></a>
  </p>
  <p><i>Concurrently Scrape Historical Calendar • Stream Live XML Feed • Discord & Telegram Alarms • ClickHouse, Postgres, InfluxDB & SQLite SDK Drivers • Parquet, CSV & JSON Export • Python Pandas Wrapper</i></p>
</div>

---

## ⚡ Why `forexfactory-go`?

Compared to standard single-threaded scrappers or manual download tasks, `forexfactory-go` is built for **speed, industrial durability, and seamless analytical storage integration**.

| Feature | `forexfactory-go` | Standard Scrapers |
|---|---|---|
| **Speed & Concurrency** | 🚀 Concurrent Multi-Worker Pool with Rate Limiting | 🐢 Slow, single-threaded queries |
| **Cloudflare Bypass** | 🛡️ Long-Lived Headless Browser Fallback + Automated Cookie Cache | ❌ Blocks instantly |
| **Database SDK Drivers** | 🗄️ **SQLite** (CGO-free WAL), **PostgreSQL**, **ClickHouse** & **InfluxDB** | ❌ None (Requires custom code) |
| **Format Support** | 📊 **Apache Parquet**, CSV, JSON | ⚠️ Raw text or JSON only |
| **Live Notifications** | 🔔 Real-time Alarm Daemon (`ff-notifier`) for **Telegram** & **Discord** | ❌ None |
| **Cross-Language Bindings**| 🐍 Native Go Library + C-Shared bindings and **Python Pandas Wrapper** | ❌ Go/Node only |
| **Timezone Precision** | 🌍 Force UTC visual scrape + Target timezone translation | ⚠️ Timezone shifts cause errors |
| **Offline Testing** | 🧪 Bulletproof `mockRoundTripper` for test stability without hitting network | ❌ Hits live server (unstable) |

---

## 🚀 Installation

### As a CLI Tool
To install the pre-built CLI tool directly to your `$GOPATH/bin`:
```bash
go install github.com/Nosvemos/forexfactory-go/cmd/forexfactory-go@latest
```

### As a Go Library
To import and use the Go SDK inside your own algorithms:
```bash
go get github.com/Nosvemos/forexfactory-go
```

---

## 📖 CLI Usage

The CLI executable provides three modular commands: `download`, `dbload`, and `live`.

### 1. Download Historical Calendar Data
Download calendar events concurrently to JSON, CSV, or Apache Parquet files:
```bash
# Concurrently scrape historical range to Apache Parquet (optimal for Pandas/Polars)
forexfactory-go download --start 2026-05-01 --end 2026-05-15 --format parquet --output calendar.parquet

# Scrape a month of data concurrently to CSV converting event times to local zone
forexfactory-go download -s 2026-05-01 -e 2026-05-31 --concurrency 4 -f csv -o calendar.csv --timezone "Local"
```

### 2. Download and Load directly into SQLite
```bash
# Downloads and loads range directly inside a WAL-optimized, local SQLite database
forexfactory-go dbload --start 2026-05-01 --end 2026-05-31 --db forexfactory.db
```

### 3. Stream Live Economic Feed
```bash
# Stream live weekly events inside a colorized ASCII terminal table, polling every 60 seconds
forexfactory-go live --interval 60s --timezone "America/New_York"
```

### 4. Background Webhook Alarm Daemon (`ff-notifier`)
Launch the background notification alerting daemon that watches the feed and pushes Alert Cards to Discord webhooks or Telegram chats exactly 15 minutes before news releases:
```bash
# Run notifier binary
go install github.com/Nosvemos/forexfactory-go/cmd/ff-notifier@latest

ff-notifier --discord-webhook "https://discord.com/api/webhooks/..." \
            --telegram-token "123456:ABC..." \
            --telegram-chat "@my_alerts" \
            --min-impact "Medium"
```

---

## 🐍 Python Pandas SDK Wrapper

`forexfactory-go` exposes a highly-efficient **C-Shared Library Bindings** and a Python ctypes wrapper, allowing Python quantitative analysts to scrape events directly into standard **Pandas DataFrames** with zero dependencies.

### 🪄 Compile Bindings
Compile the Go core to a shared library using the included Makefile:
- **Windows**: `make build-dll` or `go build -buildmode=c-shared -o libforexfactory.dll pkg/bindings/bindings.go`
- **Linux/Mac**: `make build-so` or `go build -buildmode=c-shared -o libforexfactory.so pkg/bindings/bindings.go`

### 💻 Python Integration Example
Run this inside the `python-sdk/` directory (where the wrapper is located):
```python
from datetime import datetime
from forexfactory import ForexFactoryClient

# Initialize Go client through Python ctypes (autodetects libforexfactory.dll/.so)
client = ForexFactoryClient(
    rate_limit=2,
    concurrency=4,
    timezone="UTC",
    impacts=["High", "Medium"]
)

start = datetime(2026, 5, 1)
end = datetime(2026, 5, 10)

# Scrape concurrently and return directly as a structured Pandas DataFrame!
df = client.fetch_range(start, end, as_dataframe=True)

print(df[["date", "currency", "impact", "title", "forecast", "actual"]].head(10))

# Safely close browser allocations
client.close()
```

---

## 🗄️ Database Storage SDK Drivers

The SDK is equipped with a decoupled persistence layer supporting SQLite, PostgreSQL, ClickHouse, and InfluxDB out-of-the-box.

### 1. ClickHouse (High-Performance Columnar OLAP)
Designed with a `ReplacingMergeTree` schema to handle atomic deduplication and high-speed columnar query batching:
```go
import "github.com/Nosvemos/forexfactory-go/pkg/storage"

// Initialize analytical ClickHouse driver
store := storage.NewClickHouseStorage("localhost:9000", "default", "username", "password")
err := store.Init(ctx)

// Bulk-saves scraped events using transaction batching
err = store.SaveEvents(ctx, events)
```

### 2. PostgreSQL (Relational Metadata Store)
Implements an optimized `ON CONFLICT (id) DO UPDATE` bulk-upsert routine:
```go
store := storage.NewPostgresStorage("postgres://user:pass@localhost:5432/dbname?sslmode=disable")
err := store.Init(ctx)
err = store.SaveEvents(ctx, events)
```

### 3. InfluxDB (Time-Series DB)
Writes events as measurement metric points with indexed tags. Queries use native Flux `pivot` routines to reconstruct flat chronological event list:
```go
store := storage.NewInfluxDBStorage("http://localhost:8086", "auth-token", "organization", "bucket-name")
err := store.Init(ctx)
err = store.SaveEvents(ctx, events)
```

---

## 🛡️ Premium Anti-Bot Protections

Forex Factory utilizes Cloudflare bot protection. `forexfactory-go` bypasses this transparently and reliably:
1. **Long-Lived Session Cache**: Automatically saves resolved Cloudflare session cookies and their matching User-Agent inside `session.json` in the user's standard OS cache directory (`os.UserCacheDir()`). This completely avoids launching the browser on subsequent executions.
2. **Dual User-Agent Identity Alignment**: The browser is executed naturally without overriding the user-agent string to prevent platform-mismatch block triggers. The resolved natural browser identity is dynamically captured and bound to the asynchronus TLS fingerprint spoofer (`imroc/req/v3`) for subsequent requests.
3. **Automatic Headed Fallback**: Cloudflare Turnstile blocks standard headless execution. If the background headless attempt fails or times out, the client automatically falls back to headed mode (opening a brief browser window to pass the challenge automatically) and automatically closes once solved, achieving 100% bypass reliability.
4. **Serialization Lock & Deadlock Prevention**: Browser operations are globally serialized via thread-safe mutex locks to prevent concurrency storms and protect memory/CPU, with optimized direct HTTP retries to eliminate deadlock risks.

---

## 🧪 Offline Testing

Developers can test their backtest systems offline using the SDK's built-in `mockRoundTripper`, allowing full parser execution without internet connectivity:
```go
import (
    "net/http"
    "github.com/Nosvemos/forexfactory-go/pkg/forexfactory"
)

// Setup mock client returning predefined weekly and live economic feeds
mockClient := &http.Client{Transport: &mockRoundTripper{}}
client := forexfactory.NewClient(forexfactory.WithHTTPClient(mockClient))
```

---

## 📄 License

This project is licensed under the MIT License - see the [LICENSE](LICENSE) file for details.
