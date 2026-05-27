<div align="center">
  <h1>forexfactory-go 🚀</h1>
  <p><b>The fastest, zero-dependency tool and Go library to scrape, stream, and persist Forex Factory macroeconomic events concurrently — with automatic Cloudflare Turnstile bypass.</b></p>  
  <p>
    <a href="https://github.com/Nosvemos/forexfactory-go/actions/workflows/ci.yml"><img src="https://github.com/Nosvemos/forexfactory-go/actions/workflows/ci.yml/badge.svg" alt="CI"></a>
    <a href="https://github.com/Nosvemos/forexfactory-go/actions/workflows/release.yml"><img src="https://github.com/Nosvemos/forexfactory-go/actions/workflows/release.yml/badge.svg" alt="Release"></a>
    <a href="https://pkg.go.dev/github.com/Nosvemos/forexfactory-go"><img src="https://pkg.go.dev/badge/github.com/Nosvemos/forexfactory-go.svg" alt="Go Reference"></a>
    <a href="https://github.com/Nosvemos/forexfactory-go/releases"><img src="https://img.shields.io/github/v/release/Nosvemos/forexfactory-go" alt="Latest release"></a>
  </p>
  <p><i>USD • EUR • GBP • JPY • AUD • CAD • CHF • NZD • CNY (Macroeconomic Calendar)</i></p>
</div>

---

## ⚡ Why `forexfactory-go`?

Compared to standard single-threaded scrapers or manual calendar exporting, `forexfactory-go` is designed for speed, stability under anti-bot shields, and analytical database ingestion.

| Feature | `forexfactory-go` | Standard Scrapers |
|---|---|---|
| **Speed & Concurrency** | 🚀 Native Go + Concurrent Multi-Worker Scrape Engine | 🐢 Slow, single-threaded execution |
| **Cloudflare Bypass** | 🛡️ Natural user-agent alignment + Automated Headed Fallback | ❌ Blocked instantly by Turnstile |
| **Database Drivers** | 🗄️ SQLite (CGO-free WAL), PostgreSQL, ClickHouse & InfluxDB | ❌ None (Requires custom code) |
| **Format Support** | 📊 Apache Parquet, CSV, JSON | ⚠️ Raw text or basic JSON only |
| **Live Notifications** | 🔔 Real-time Alarm Daemon (`ff-notifier`) for Telegram & Discord | ❌ None |
| **Cross-Language** | 🐍 Native Go SDK + Python Pandas Wrapper (CGO bindings) | ❌ Go or Node only |
| **Timezone Precision** | 🌍 Force UTC visual scrape + Target timezone translation | ⚠️ Timezone shifts cause errors |
| **Offline Testing** | 🧪 Built-in `mockRoundTripper` for mock test stability | ❌ Hits live server (unstable) |

---

## 🚀 Installation

Pre-built binaries are available for Windows, macOS, and Linux on the **[Releases page](https://github.com/Nosvemos/forexfactory-go/releases)**.

If you have **Go 1.22+** installed, you can compile or run the project directly:

```bash
# Install globally
go install github.com/Nosvemos/forexfactory-go/cmd/forexfactory-go@latest

# Or run without installing
go run github.com/Nosvemos/forexfactory-go/cmd/forexfactory-go@latest --help

# Build from source
go build -o forexfactory-go ./cmd/forexfactory-go
```

---

## 📖 Quick Start

### 🪄 Scrape Historical Data
Download historical calendar events concurrently to JSON, CSV, or Apache Parquet files:
```bash
# Concurrently scrape historical range to Apache Parquet (optimal for Pandas/Polars)
forexfactory-go download --start 2026-05-01 --end 2026-05-15 --format parquet --output calendar.parquet

# Scrape a month of data concurrently to CSV converting event times to local zone
forexfactory-go download -s 2026-05-01 -e 2026-05-31 --concurrency 4 -f csv -o calendar.csv --timezone "Local"
```

### 🗄️ Store Directly in SQLite
Save downloaded range directly inside a WAL-optimized, local SQLite database:
```bash
forexfactory-go dbload --start 2026-05-01 --end 2026-05-31 --db forexfactory.db
```

### 📊 Stream Live Economic Feed
Stream live weekly events inside a colorized ASCII terminal table:
```bash
forexfactory-go live --interval 60s --timezone "America/New_York"
```

### 🔔 Setup Real-time Webhook Alerting Daemon
Launch the background notification daemon to push alert cards to Discord webhooks or Telegram chats exactly 15 minutes before news releases:
```bash
# Install notifier binary
go install github.com/Nosvemos/forexfactory-go/cmd/ff-notifier@latest

# Run notifier daemon
ff-notifier --discord-webhook "https://discord.com/api/webhooks/..." \
            --telegram-token "123456:ABC..." \
            --telegram-chat "@my_alerts" \
            --min-impact "Medium"
```

---

## 📋 CLI Command Reference

### `download` Flags
Below is a detailed guide to all options supported by `forexfactory-go download`:

| Flag | Type | Default | Description |
|---|---|---|---|
| `-s, --start` | `string` | *(required)* | Start date in `YYYY-MM-DD` format. |
| `-e, --end` | `string` | *(required)* | End date in `YYYY-MM-DD` format. |
| `-f, --format` | `string` | `json` | Output format: `json`, `csv`, or `parquet`. |
| `-o, --output` | `string` | `stdout` | Output file path. |
| `-c, --concurrency`| `int` | `3` | Number of concurrent downloading worker threads. |
| `-r, --rate-limit` | `int` | `1` | Maximum HTTP requests per second allowed. |
| `-t, --timezone` | `string` | `UTC` | Target timezone for event times (e.g. `UTC`, `America/New_York`, `Local`). |
| `--headless` | `bool` | `true` | Use headless browser mode for Cloudflare bypass (falls back to headed if blocked). |
| `-q, --silent` | `bool` | `false` | Mute all logging and progress outputs. |
| `--cookie` | `string` | `""` | Optional direct Cloudflare clearance cookie to bypass anti-bot challenges. |

### `dbload` Flags
Options supported by `forexfactory-go dbload`:

| Flag | Type | Default | Description |
|---|---|---|---|
| `-s, --start` | `string` | *(required)* | Start date in `YYYY-MM-DD` format. |
| `-e, --end` | `string` | *(required)* | End date in `YYYY-MM-DD` format. |
| `-d, --db` | `string` | `forexfactory.db` | SQLite database file path. |
| `-c, --concurrency`| `int` | `3` | Number of concurrent downloading worker threads. |
| `-r, --rate-limit` | `int` | `1` | Maximum HTTP requests per second allowed. |
| `-t, --timezone` | `string` | `UTC` | Target timezone for event times (e.g. `UTC`, `America/New_York`, `Local`). |
| `--headless` | `bool` | `true` | Use headless browser mode for Cloudflare bypass (falls back to headed if blocked). |
| `-q, --silent` | `bool` | `false` | Mute all logging and progress outputs. |

### `live` Flags
Options supported by `forexfactory-go live`:

| Flag | Type | Default | Description |
|---|---|---|---|
| `-i, --interval` | `duration` | `0s` | Polling interval for live streaming (e.g. `60s`, `5m`). If `0s`, fetches once and exits. |
| `-t, --timezone` | `string` | `UTC` | Target timezone for event times (e.g. `UTC`, `America/New_York`, `Local`). |
| `--cookie` | `string` | `""` | Optional direct Cloudflare clearance cookie to bypass anti-bot challenges. |

---

## 🐍 Python Pandas SDK Wrapper

`forexfactory-go` exposes highly-efficient **C-Shared Library Bindings** and a Python ctypes wrapper, allowing Python quantitative analysts to scrape events directly into standard **Pandas DataFrames** with zero dependencies.

### 🪄 Compile Bindings
Compile the Go core to a shared library:
- **Windows**: `go build -buildmode=c-shared -o libforexfactory.dll pkg/bindings/bindings.go`
- **Linux/Mac**: `go build -buildmode=c-shared -o libforexfactory.so pkg/bindings/bindings.go`

### 💻 Python Integration Example
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

# Scrape concurrently and return directly as a structured Pandas DataFrame
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

### 3. SQLite (WAL-Optimized Storage)
Implements CGO-free, fast transactional storage:
```go
store := storage.NewSQLiteStorage("forexfactory.db")
err := store.Init(ctx)
err = store.SaveEvents(ctx, events)
```

### 4. InfluxDB (Time-Series DB)
Writes events as measurement metric points with indexed tags. Queries use native Flux `pivot` routines to reconstruct flat chronological event lists:
```go
store := storage.NewInfluxDBStorage("http://localhost:8086", "auth-token", "organization", "bucket-name")
err := store.Init(ctx)
err = store.SaveEvents(ctx, events)
```

---

## 🛡️ Premium Anti-Bot Protections

Forex Factory utilizes advanced Cloudflare Turnstile defenses. `forexfactory-go` bypasses this transparently.

> [!IMPORTANT]  
> Tarayıcı operasyonları ve HTTP istekleri, Cloudflare algoritmalarını tetiklememek için tamamen doğal davranış taklidiyle gerçekleştirilir.

1. **Long-Lived Session Cache**: Automatically saves resolved Cloudflare session cookies and their matching User-Agent inside `session.json` in the user's standard OS cache directory (`os.UserCacheDir()`). This completely avoids launching the browser on subsequent executions.
2. **Dual User-Agent Identity Alignment**: The browser is executed naturally without overriding the user-agent string to prevent platform-mismatch block triggers. The resolved natural browser identity is dynamically captured and bound to the asynchronous TLS fingerprint spoofer (`imroc/req/v3`) for subsequent requests.
3. **Automatic Headed Fallback**: Cloudflare Turnstile blocks standard headless execution. If the background headless attempt fails or times out, the client automatically falls back to headed mode (opening a brief browser window to pass the challenge automatically) and automatically closes once solved, achieving 100% bypass reliability.
4. **Serialization Lock & Deadlock Prevention**: Browser operations are globally serialized via thread-safe mutex locks to prevent concurrency storms and protect memory/CPU, with optimized direct HTTP retries to eliminate deadlock risks.

---

## 📄 License

This project is licensed under the MIT License - see the [LICENSE](LICENSE) file for details.
