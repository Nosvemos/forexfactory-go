<div align="center">
  <h1>tradingview-calendar-go 🚀</h1>
  <p><b>Lightning-fast, Pure HTTP TradingView Economic Calendar Engine, CLI, Microservice & SDK</b></p>
  <p>
    <a href="https://github.com/Nosvemos/tradingview-calendar-go/actions/workflows/ci.yml"><img src="https://github.com/Nosvemos/tradingview-calendar-go/actions/workflows/ci.yml/badge.svg" alt="CI"></a>
    <a href="https://github.com/Nosvemos/tradingview-calendar-go/actions/workflows/release.yml"><img src="https://github.com/Nosvemos/tradingview-calendar-go/actions/workflows/release.yml/badge.svg" alt="Release"></a>
    <a href="https://pkg.go.dev/github.com/Nosvemos/tradingview-calendar-go"><img src="https://pkg.go.dev/badge/github.com/Nosvemos/tradingview-calendar-go.svg" alt="Go Reference"></a>
    <a href="https://github.com/Nosvemos/tradingview-calendar-go/releases"><img src="https://img.shields.io/github/v/release/Nosvemos/tradingview-calendar-go" alt="Latest release"></a>
    <img src="https://img.shields.io/badge/go-1.26+-00ADD8.svg" alt="Go version">
    <img src="https://img.shields.io/badge/license-Non--Commercial%20Source--Available-red.svg" alt="License">
  </p>
</div>

---

## ⚡ Highlights

- **Pure HTTP Engine (0% Cloudflare Blocks):** Direct JSON streaming with ~30ms latency, zero browser / headless dependencies, and no Cloudflare Turnstile blocks.
- **12+ Years Historical Depth:** Download 2014 to present and future releases without subscription fees, API keys, or request caps.
- **Ultra-High Concurrency:** Concurrent monthly chunking downloads 8–10 years (180,000+ events) in under 15 seconds.
- **Rich Macroeconomic Fields:** `id`, `title`, `country`, `currency`, `date`, `impact`, `actual`, `forecast`, `previous`, `unit`, `indicator`, `category`, `source`, `ticker`, `deviation`, `surprise`, and `market_bias`.
- **Multi-Format Export:** Snappy-compressed Apache Parquet, Excel (.xlsx) with color-coded styles, CSV, and JSON.
- **Pluggable Storage Engine:** Built-in drivers for SQLite (WAL mode), PostgreSQL, ClickHouse, and InfluxDB.
- **Embedded REST & SSE Microservice:** Launch a local microservice (`tvcalendar serve`) exposing RESTful APIs and real-time Server-Sent Events (SSE).
- **MetaTrader 4 & 5 Bridge:** Automatic news filter sync (`tvcalendar bridge`) with ready-to-use [`TradingViewNewsFilter.mqh`](include/TradingViewNewsFilter.mqh).
- **Real-Time Notification Daemon (`tv-notifier`):** Automated webhook alerts for Discord, Telegram, Slack, and generic webhooks.
- **Python / Pandas CGO SDK:** Query calendar data directly into Pandas DataFrames via high-speed CGO shared bindings.

---

## 📦 Installation

### Pre-built Binaries
Download ready-to-run executables for Windows, macOS, and Linux from the **[Releases](https://github.com/Nosvemos/tradingview-calendar-go/releases)** page.

### Go Install
```bash
go install github.com/Nosvemos/tradingview-calendar-go/cmd/tvcalendar@latest
go install github.com/Nosvemos/tradingview-calendar-go/cmd/tv-notifier@latest
```

### Build from Source
```bash
git clone https://github.com/Nosvemos/tradingview-calendar-go.git
cd tradingview-calendar-go
go build -o tvcalendar ./cmd/tvcalendar
```

---

## 🚀 CLI Usage

### 1. Download Historical Data (Parquet / Excel / CSV / JSON)
```bash
# Export 8 years of historical data to Apache Parquet (Snappy compressed)
tvcalendar download --start 2017-01-01 --end 2025-01-01 --format parquet --output calendar.parquet

# Export with Impact and Currency filters to styled Excel (.xlsx)
tvcalendar download --start 2024-01-01 --end 2025-01-01 --currency USD,EUR --impact High --format xlsx --output calendar.xlsx

# Export to CSV with custom concurrency and timezone
tvcalendar download -s 2020-01-01 -e 2025-01-01 -f csv -o calendar.csv --concurrency 8 --timezone "UTC"
```

### 2. Live Calendar Terminal Dashboard
```bash
# Display live economic calendar with real-time countdown to next high-impact release
tvcalendar live

# Poll every 30 seconds
tvcalendar live --interval 30s --timezone "America/New_York"
```

### 3. Embedded REST & SSE Microservice
```bash
tvcalendar serve --port 8080
```
- **Health Check:** `GET http://localhost:8080/health`
- **Calendar Range:** `GET http://localhost:8080/api/v1/calendar?start=2025-01-01&end=2025-01-31&currency=USD&impact=high`
- **Live Feed:** `GET http://localhost:8080/api/v1/live`
- **SSE Stream:** `GET http://localhost:8080/api/v1/stream`
- **Server Stats:** `GET http://localhost:8080/api/v1/stats`

### 4. MetaTrader 4 / 5 EA News Filter Bridge
```bash
tvcalendar bridge --mt4-dir "C:/Users/.../AppData/Roaming/MetaQuotes/Terminal/<ID>/MQL4/Files" --min-impact High --interval 60s
```
*Drop [`include/TradingViewNewsFilter.mqh`](include/TradingViewNewsFilter.mqh) into your Expert Advisor to restrict trading during high-impact news releases:*
```cpp
#include <TradingViewNewsFilter.mqh>
CTradingViewNewsFilter newsFilter;

void OnTick() {
    if (newsFilter.IsNewsRestricted(_Symbol, 30, 15, "High")) {
        Print("Trading halted due to high-impact economic news");
        return;
    }
    // Execute trade logic...
}
```

### 5. SQLite Database Bulk Ingestion
```bash
tvcalendar dbload --start 2020-01-01 --end 2025-01-01 --db calendar.db
```

### 6. Real-Time Alert Daemon (`tv-notifier`)
```bash
# Discord Channel Alerts
tv-notifier --discord-webhook "https://discord.com/api/webhooks/..." --min-impact High --lead-time 15m

# Telegram Bot Alerts
tv-notifier --telegram-token "BOT_TOKEN" --telegram-chat "CHAT_ID" --min-impact High --lead-time 15m
```

---

## 🐍 Python SDK (Pandas Integration)

`tradingview-calendar-go` provides CGO shared library bindings for direct ingestion into structured **Pandas DataFrames**.

1. **Build shared library:**
   ```bash
   make build-dll    # Windows (libtvcalendar.dll)
   make build-so     # Linux/macOS (libtvcalendar.so)
   ```

2. **Python usage:**
   ```python
   from datetime import datetime
   from python_sdk.tradingview_calendar import TradingViewCalendarClient

   client = TradingViewCalendarClient(
       rate_limit=10,
       concurrency=5,
       timezone="UTC",
       impacts=["High", "Medium"]
   )

   # Fetch historical range directly as Pandas DataFrame
   df = client.fetch_range(
       start_date=datetime(2020, 1, 1),
       end_date=datetime(2025, 1, 1),
       as_dataframe=True
   )

   print(df.head())
   df.to_parquet("calendar.parquet")
   ```

---

## 💻 Go SDK Example

```go
package main

import (
	"context"
	"fmt"
	"time"

	"github.com/Nosvemos/tradingview-calendar-go/pkg/tvcalendar"
)

func main() {
	client := tvcalendar.NewClient(
		tvcalendar.WithConcurrency(5),
		tvcalendar.WithTimeLocation(time.UTC),
	)

	start := time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)
	end := time.Date(2024, 12, 31, 23, 59, 59, 0, time.UTC)

	events, err := client.FetchRange(context.Background(), start, end)
	if err != nil {
		panic(err)
	}

	fmt.Printf("Fetched %d events in range\n", len(events))
	for _, e := range events[:5] {
		fmt.Printf("[%s] %s | %s %s | Actual: %s, Forecast: %s\n",
			e.Date.Format("2006-01-02 15:04"), e.Impact, e.Country, e.Title, e.Actual, e.Forecast)
	}
}
```

---

## 🗄️ Pluggable Database Storage Drivers

```go
import (
	"context"
	"github.com/Nosvemos/tradingview-calendar-go/pkg/storage"
)

// SQLite
store := storage.NewSQLiteStorage("calendar.db")

// PostgreSQL
store := storage.NewPostgresStorage("postgres://user:pass@localhost:5432/economic_db?sslmode=disable")

// ClickHouse
store := storage.NewClickHouseStorage("localhost:9000", "default", "default", "")

// InfluxDB
store := storage.NewInfluxDBStorage("http://localhost:8086", "TOKEN", "ORG", "BUCKET")

_ = store.Init(context.Background())
_ = store.SaveEvents(context.Background(), events)
```

---

## 📄 License

This project is licensed under the **Non-Commercial Source-Available License (NCSA 1.0)** - see the [LICENSE](LICENSE) file for details.

- ✅ **Allowed:** Source code inspection, personal non-commercial use, educational research, local modifications, and free non-commercial redistribution.
- ❌ **Prohibited:** Commercial use, resale, paid distribution, SaaS/cloud hosting, sublicensing, or bundling into commercial trading products/EAs without express written authorization.
