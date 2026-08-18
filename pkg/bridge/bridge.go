package bridge

import (
	"context"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/Nosvemos/tradingview-calendar-go/pkg/tvcalendar"
)

// BridgeConfig holds parameters for syncing Forex Factory news with MT4/MT5 terminals.
type BridgeConfig struct {
	OutputDir   string
	MinImpact   tvcalendar.Impact
	Interval    time.Duration
	Timezone    *time.Location
	Currencies  []string
	Cookie      string
	Headless    bool
}

// NewsFilterPayload represents the JSON contract ingested by MetaTrader Expert Advisors.
type NewsFilterPayload struct {
	LastUpdatedUTC string          `json:"last_updated_utc"`
	NextEvent      *UpcomingEvent  `json:"next_event,omitempty"`
	Events         []UpcomingEvent `json:"events"`
}

// UpcomingEvent represents a lightweight event item formatted for MT4/MT5 EA parsing.
type UpcomingEvent struct {
	ID          string `json:"id"`
	Title       string `json:"title"`
	Currency    string `json:"currency"`
	DateUTC     string `json:"date_utc"`
	Timestamp   int64  `json:"timestamp"`
	MinutesLeft int    `json:"minutes_left"`
	Impact      string `json:"impact"`
	Forecast    string `json:"forecast"`
	Previous    string `json:"previous"`
}

// Bridge periodically fetches live economic events and publishes atomic news filter files for MT4/MT5.
type Bridge struct {
	cfg    BridgeConfig
	client *tvcalendar.Client
}

// NewBridge creates a new MetaTrader news filter bridge instance.
func NewBridge(cfg BridgeConfig) *Bridge {
	if cfg.Interval <= 0 {
		cfg.Interval = 1 * time.Minute
	}
	if cfg.MinImpact == "" {
		cfg.MinImpact = tvcalendar.ImpactHigh
	}
	if cfg.Timezone == nil {
		cfg.Timezone = time.UTC
	}

	var clientOpts []tvcalendar.Option
	clientOpts = append(clientOpts, tvcalendar.WithTimeLocation(cfg.Timezone))

	return &Bridge{
		cfg:    cfg,
		client: tvcalendar.NewClient(clientOpts...),
	}
}

// SyncOnce fetches live feed events, filters them, and atomically writes json and csv files.
func (b *Bridge) SyncOnce(ctx context.Context) error {
	events, err := b.client.FetchLiveFeed(ctx)
	if err != nil {
		return fmt.Errorf("failed to fetch live feed for MT4/MT5 bridge: %w", err)
	}

	now := time.Now().UTC()
	var upcoming []UpcomingEvent
	var nextEvt *UpcomingEvent

	currFilter := make(map[string]bool)
	for _, c := range b.cfg.Currencies {
		currFilter[strings.ToUpper(strings.TrimSpace(c))] = true
	}

	for _, e := range events {
		// Filter currency
		if len(currFilter) > 0 && !currFilter[strings.ToUpper(e.Currency)] {
			continue
		}

		// Filter impact
		if !isImpactEligible(e.Impact, b.cfg.MinImpact) {
			continue
		}

		evtTime := e.Date.UTC()
		diff := evtTime.Sub(now)
		mins := int(diff.Minutes())

		// Only include events occurring between -60 minutes (recent) to +7 days ahead
		if mins < -60 || mins > 7*24*60 {
			continue
		}

		ue := UpcomingEvent{
			ID:          e.ID,
			Title:       e.Title,
			Currency:    e.Currency,
			DateUTC:     evtTime.Format("2006-01-02 15:04:05"),
			Timestamp:   evtTime.Unix(),
			MinutesLeft: mins,
			Impact:      string(e.Impact),
			Forecast:    e.Forecast,
			Previous:    e.Previous,
		}

		upcoming = append(upcoming, ue)

		if mins >= 0 {
			if nextEvt == nil || ue.Timestamp < nextEvt.Timestamp {
				copyNext := ue
				nextEvt = &copyNext
			}
		}
	}

	payload := NewsFilterPayload{
		LastUpdatedUTC: now.Format("2006-01-02 15:04:05"),
		NextEvent:      nextEvt,
		Events:         upcoming,
	}

	if err := b.writeAtomicFiles(payload); err != nil {
		return fmt.Errorf("failed to write MT4/MT5 news files: %w", err)
	}

	return nil
}

// Start runs the periodic synchronization loop until the context is canceled.
func (b *Bridge) Start(ctx context.Context) error {
	if err := b.SyncOnce(ctx); err != nil {
		fmt.Fprintf(os.Stderr, "[Bridge Warning] Initial sync failed: %v\n", err)
	}

	ticker := time.NewTicker(b.cfg.Interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			_ = b.client.Close()
			return ctx.Err()
		case <-ticker.C:
			if err := b.SyncOnce(ctx); err != nil {
				fmt.Fprintf(os.Stderr, "[Bridge Warning] Periodic sync failed: %v\n", err)
			}
		}
	}
}

func (b *Bridge) writeAtomicFiles(payload NewsFilterPayload) error {
	if b.cfg.OutputDir == "" {
		b.cfg.OutputDir = "."
	}
	if err := os.MkdirAll(b.cfg.OutputDir, 0755); err != nil {
		return err
	}

	// 1. Write JSON file atomically
	jsonPath := filepath.Join(b.cfg.OutputDir, "ff_news_filter.json")
	tmpJSONPath := jsonPath + ".tmp"

	jsonData, err := json.MarshalIndent(payload, "", "  ")
	if err != nil {
		return err
	}

	if err := os.WriteFile(tmpJSONPath, jsonData, 0644); err != nil {
		return err
	}
	_ = os.Remove(jsonPath) // Ensure Windows rename succeeds
	if err := os.Rename(tmpJSONPath, jsonPath); err != nil {
		return err
	}

	// 2. Write CSV file atomically (for legacy MT4 EAs that lack native JSON parsers)
	csvPath := filepath.Join(b.cfg.OutputDir, "ff_news_filter.csv")
	tmpCSVPath := csvPath + ".tmp"

	f, err := os.Create(tmpCSVPath)
	if err != nil {
		return err
	}

	w := csv.NewWriter(f)
	_ = w.Write([]string{"Currency", "MinutesLeft", "Impact", "Timestamp", "Title", "Forecast", "Previous"})

	for _, e := range payload.Events {
		_ = w.Write([]string{
			e.Currency,
			strconv.Itoa(e.MinutesLeft),
			e.Impact,
			strconv.FormatInt(e.Timestamp, 10),
			e.Title,
			e.Forecast,
			e.Previous,
		})
	}
	w.Flush()
	_ = f.Close()

	_ = os.Remove(csvPath)
	return os.Rename(tmpCSVPath, csvPath)
}

func isImpactEligible(actual, target tvcalendar.Impact) bool {
	weight := map[tvcalendar.Impact]int{
		tvcalendar.ImpactHigh:   3,
		tvcalendar.ImpactMedium: 2,
		tvcalendar.ImpactLow:    1,
		tvcalendar.ImpactNone:   0,
	}
	return weight[actual] >= weight[target]
}
