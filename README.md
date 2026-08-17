<div align="center">
  <h1>forexfactory-go 🚀</h1>
  <p><b>High-performance Go CLI, SDK, and microservice to scrape, stream, analyze, and persist Forex Factory macroeconomic events concurrently — featuring automated Cloudflare Turnstile stealth bypass.</b></p>  
  <p>
    <a href="https://github.com/Nosvemos/forexfactory-go/actions/workflows/ci.yml"><img src="https://github.com/Nosvemos/forexfactory-go/actions/workflows/ci.yml/badge.svg" alt="CI"></a>
    <a href="https://github.com/Nosvemos/forexfactory-go/actions/workflows/release.yml"><img src="https://github.com/Nosvemos/forexfactory-go/actions/workflows/release.yml/badge.svg" alt="Release"></a>
    <a href="https://pkg.go.dev/github.com/Nosvemos/forexfactory-go"><img src="https://pkg.go.dev/badge/github.com/Nosvemos/forexfactory-go.svg" alt="Go Reference"></a>
    <a href="https://github.com/Nosvemos/forexfactory-go/releases"><img src="https://img.shields.io/github/v/release/Nosvemos/forexfactory-go" alt="Latest release"></a>
  </p>
  <p><i>USD • EUR • GBP • JPY • AUD • CAD • CHF • NZD • CNY (Global Macroeconomic Calendar)</i></p>
</div>

---

## ⚡ Key Highlights

- **High-Speed Concurrency:** Scrape 1+ years of historical economic events across multi-worker threads in seconds.
- **Automated Cloudflare Turnstile Stealth Bypass:** Chrome TLS JA3/JA4 fingerprint matching, CDP stealth script injection (`navigator.webdriver` masking), and persistent cookie session caching.
- **Network Resilience:** 3-tier exponential backoff retries, proxy pool rotation (`WithProxyPool`), and User-Agent rotation (`WithUserAgentPool`).
- **Deep Event Analytics:** Event specifications scraper (`FetchEventDetail`) with 10-year historical release tables, `Deviation`, `Surprise %`, and `MarketBias` (Bullish/Bearish) calculations.
- **Versatile Data Formats:** Export to **CSV**, **JSON**, **Apache Parquet**, and color-formatted **Excel (`.xlsx`)**.
- **Pluggable Database SDK Drivers:** Ready-to-use persistence for **SQLite (CGO-free WAL)**, **PostgreSQL**, **ClickHouse**, and **InfluxDB**.
- **Embedded REST & SSE Microservice:** Launch a local HTTP server (`forexfactory-go serve`) exposing RESTful JSON APIs and Server-Sent Events (SSE) live streams.
- **MetaTrader 4 & 5 EA Bridge:** Atomic news filter synchronization (`forexfactory-go bridge`) with ready-to-use `ForexFactoryNewsFilter.mqh` C++ MQL class.
- **Multi-Channel Alert Daemon (`ff-notifier`):** Automated notifications for **Discord**, **Slack**, **Telegram**, and custom generic HTTP webhooks.
- **Cross-Language Python SDK:** Native CGO shared library bindings with direct **Pandas DataFrame** integration.

---

## 🚀 Installation

### Pre-built Binaries
Download ready-to-run executables for Windows, macOS, and Linux from the **[Releases page](https://github.com/Nosvemos/forexfactory-go/releases)**.

### From Source (Go 1.22+)
```bash
# Install CLI tools globally
go install github.com/Nosvemos/forexfactory-go/cmd/forexfactory-go@latest
go install github.com/Nosvemos/forexfactory-go/cmd/ff-notifier@latest

# Or clone and build locally
git clone https://github.com/Nosvemos/forexfactory-go.git
cd forexfactory-go
go build -o forexfactory-go ./cmd/forexfactory-go
go build -o ff-notifier ./cmd/ff-notifier
```

---

## 📖 CLI Usage & Quick Start

### 1. Download Historical Calendar Data
Concurrently download events across any date range into Parquet, Excel, CSV, or JSON:
```bash
# Export to Apache Parquet (optimized for Pandas/Polars/DuckDB)
forexfactory-go download --start 2026-05-01 --end 2026-05-31 --format parquet --output calendar.parquet

# Export to styled & color-coded Excel (.xlsx) file
forexfactory-go download --start 2026-05-01 --end 2026-05-31 --format xlsx --output calendar.xlsx

# Export to CSV with local timezone conversion
forexfactory-go download -s 2026-01-01 -e 2026-05-31 -f csv -o calendar.csv --timezone "Local" --concurrency 4
```

### 2. Deep Dive Event Specs & 10-Year History
Scrape detailed economic specifications (Source, Measures, Usual Effect, Frequency, Next Release) and historical release records for any Forex Factory event ID:
```bash
forexfactory-go event 123456
```

### 3. Launch REST API & SSE Microservice Server
Start an embedded HTTP server exposing RESTful endpoints and real-time Server-Sent Events (SSE):
```bash
forexfactory-go serve --port 8080
```
**Available Endpoints:**
* `GET /health` — Server health, uptime, and system status.
* `GET /api/v1/calendar?start=2026-05-01&end=2026-05-31&currency=USD,EUR&impact=High` — Filtered calendar events.
* `GET /api/v1/event?id=123456` — Detailed event specs and past historical releases.
* `GET /api/v1/live` — Current live weekly XML feed.
* `GET /api/v1/stream` — Real-time Server-Sent Events (SSE) event stream.
* `GET /api/v1/stats` — Microservice request volume and memory statistics.

### 4. MetaTrader 4 / 5 News Filter Bridge
Synchronize atomic `ff_news_filter.json` and `ff_news_filter.csv` files into your MT4/MT5 `MQL/Files` directory for algorithmic EA news filtering:
```bash
forexfactory-go bridge --mt4-dir "C:/Users/.../AppData/Roaming/MetaQuotes/Terminal/<ID>/MQL4/Files" --min-impact High --interval 60s
```
*Traders can drop [`include/ForexFactoryNewsFilter.mqh`](include/ForexFactoryNewsFilter.mqh) straight into their Expert Advisor to call `IsNewsRestricted(symbol, minutesBefore, minutesAfter, minImpact)`.*

### 5. Ingest Directly into Local SQLite Database
```bash
forexfactory-go dbload --start 2026-01-01 --end 2026-05-31 --db forexfactory.db
```

### 6. Live Terminal Stream & Countdown Banner
Stream weekly events inside an ASCII terminal table with a live countdown timer to the next upcoming high-impact release:
```bash
forexfactory-go live --interval 60s --timezone "America/New_York"
```

### 7. Real-Time Multi-Channel Alert Daemon (`ff-notifier`)
Dispatch rich notification cards to Discord, Slack, Telegram, or custom webhooks prior to high-volatility news releases:
```bash
ff-notifier --discord-webhook "https://discord.com/api/webhooks/..." \
            --slack-webhook "https://hooks.slack.com/services/..." \
            --telegram-token "123456789:ABCDef..." \
            --telegram-chat "@my_alerts_channel" \
            --generic-webhook "https://my-trading-bot.com/api/news-alert" \
            --lead-time 15m \
            --min-impact High
```

---

## 🐍 Python Pandas SDK Wrapper

`forexfactory-go` provides CGO shared library bindings and a lightweight Python ctypes wrapper to fetch events directly into structured **Pandas DataFrames**.

### Compile Shared Library
- **Windows:** `go build -buildmode=c-shared -o libforexfactory.dll pkg/bindings/bindings.go`
- **Linux/macOS:** `go build -buildmode=c-shared -o libforexfactory.so pkg/bindings/bindings.go`

### Python Example
```python
from datetime import datetime
from forexfactory import ForexFactoryClient

# Initialize Go client via C-bindings
client = ForexFactoryClient(
    rate_limit=2,
    concurrency=4,
    timezone="UTC",
    impacts=["High", "Medium"]
)

# Fetch historical date range directly as a Pandas DataFrame
df = client.fetch_range(datetime(2026, 5, 1), datetime(2026, 5, 15), as_dataframe=True)
print(df[["date", "currency", "impact", "title", "forecast", "actual"]].head(10))

# Fetch live weekly feed
live_df = client.fetch_live_feed(as_dataframe=True)

# Free browser resources
client.close()
```

---

## 🗄️ Database Storage SDK Drivers

The modular persistence layer in `pkg/storage` provides unified interfaces for enterprise analytics:

```go
import "github.com/Nosvemos/forexfactory-go/pkg/storage"

// 1. ClickHouse (ReplacingMergeTree Columnar Batching)
store := storage.NewClickHouseStorage("localhost:9000", "default", "user", "password")

// 2. PostgreSQL (Relational Store with Upsert Conflicts)
store := storage.NewPostgresStorage("postgres://user:pass@localhost:5432/db?sslmode=disable")

// 3. SQLite (Fast CGO-Free WAL Engine)
store := storage.NewSQLiteStorage("forexfactory.db")

// 4. InfluxDB (Time-Series Metrics with Flux Pivot Queries)
store := storage.NewInfluxDBStorage("http://localhost:8086", "token", "org", "bucket")

// Common Lifecycle
_ = store.Init(ctx)
_ = store.SaveEvents(ctx, events)
defer store.Close()
```

---

## 🛡️ Anti-Bot & Network Architecture

1. **CDP Stealth Script Injection:** Injects JavaScript before document evaluation via Chrome DevTools Protocol to override `navigator.webdriver`, mock `window.chrome`, and normalize plugins/languages.
2. **TLS ClientHello Spoofing:** Mimics official Chrome JA3/JA4 TLS fingerprints and HTTP/2 settings using `imroc/req/v3`.
3. **Session Cache Persistence:** Saves resolved `cf_clearance` and timezone cookies into `session.json` (`os.UserCacheDir()`) to avoid launching browser processes on repeat runs.
4. **Exponential Backoff & Network Recovery:** 3-tier automatic retry loop with exponential delay for connection drops, timeouts, or 5xx server errors.
5. **Proxy & User-Agent Pools:** Thread-safe round-robin rotation via `WithProxyPool` and `WithUserAgentPool`.
6. **Automatic Headed Fallback:** If background headless execution is challenged, the client briefly falls back to headed mode to solve Turnstile automatically and returns to headless mode.

---

## 📄 License

This project is licensed under the MIT License - see the [LICENSE](LICENSE) file for details.

