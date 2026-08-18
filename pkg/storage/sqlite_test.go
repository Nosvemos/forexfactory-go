package storage

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/Nosvemos/tradingview-calendar-go/pkg/tvcalendar"
)

func TestSQLiteStorage(t *testing.T) {
	dbPath := "test_sandbox.db"
	defer os.Remove(dbPath)

	store := NewSQLiteStorage(dbPath)
	ctx := context.Background()

	// 1. Test Initialization
	if err := store.Init(ctx); err != nil {
		t.Fatalf("Database Init failed: %v", err)
	}

	// 2. Test Ingestion / Saving Events
	testEvents := []tvcalendar.Event{
		{
			ID:          "998877",
			Title:       "FOMC Press Conference",
			Country:     "US",
			Currency:    "USD",
			Date:        time.Date(2026, time.May, 26, 14, 30, 0, 0, time.UTC),
			Impact:      tvcalendar.ImpactHigh,
			Forecast:    "5.25%",
			Previous:    "5.50%",
			Actual:      "5.25%",
			Unit:        "%",
			Category:    "mny",
			Indicator:   "Interest Rate",
			Comment:     "Federal Reserve interest rate statement",
			Source:      "Federal Reserve",
			SourceURL:   "https://www.federalreserve.gov",
			Ticker:      "ECONOMICS:USINTR",
			IsAllDay:    false,
			IsTentative: false,
		},
		{
			ID:          "", // Testing auto-generated ID
			Title:       "French CPI m/m",
			Country:     "FR",
			Currency:    "EUR",
			Date:        time.Date(2026, time.May, 26, 8, 45, 0, 0, time.UTC),
			Impact:      tvcalendar.ImpactMedium,
			Forecast:    "0.2%",
			Previous:    "0.1%",
			Actual:      "0.3%",
			Unit:        "%",
			Category:    "prce",
			Indicator:   "Inflation",
			IsAllDay:    true,
			IsTentative: true,
		},
		{
			ID:          "low-gbp-1",
			Title:       "UK BRC Shop Price Index",
			Country:     "GB",
			Currency:    "GBP",
			Date:        time.Date(2026, time.May, 27, 0, 0, 0, 0, time.UTC),
			Impact:      tvcalendar.ImpactLow,
			Forecast:    "0.5%",
			Previous:    "0.6%",
			Actual:      "0.4%",
			IsAllDay:    true,
			IsTentative: false,
		},
	}

	if err := store.SaveEvents(ctx, testEvents); err != nil {
		t.Fatalf("SaveEvents failed: %v", err)
	}

	// 3. Test GetEvents
	startQuery := time.Date(2026, time.May, 26, 0, 0, 0, 0, time.UTC)
	endQuery := time.Date(2026, time.May, 26, 23, 59, 59, 0, time.UTC)
	events, err := store.GetEvents(ctx, startQuery, endQuery)
	if err != nil {
		t.Fatalf("GetEvents failed: %v", err)
	}
	if len(events) != 2 {
		t.Errorf("Expected 2 events from GetEvents, got %d", len(events))
	}

	// 4. Test GetEventsByCurrency
	usdEvents, err := store.GetEventsByCurrency(ctx, "usd")
	if err != nil {
		t.Fatalf("GetEventsByCurrency failed: %v", err)
	}
	if len(usdEvents) != 1 || usdEvents[0].Title != "FOMC Press Conference" {
		t.Errorf("GetEventsByCurrency failed to return USD events correctly")
	}

	// 5. Test QueryEvents with various filter permutations
	// 5a. Filter by StartDate only
	qStartOnly, err := store.QueryEvents(ctx, QueryFilter{StartDate: &startQuery})
	if err != nil || len(qStartOnly) != 3 {
		t.Errorf("QueryEvents StartDate only failed: count=%d, err=%v", len(qStartOnly), err)
	}

	// 5b. Filter by EndDate only
	qEndOnly, err := store.QueryEvents(ctx, QueryFilter{EndDate: &endQuery})
	if err != nil || len(qEndOnly) != 2 {
		t.Errorf("QueryEvents EndDate only failed: count=%d, err=%v", len(qEndOnly), err)
	}

	// 5c. Filter by Currencies only (multiple currencies)
	qCurrs, err := store.QueryEvents(ctx, QueryFilter{Currencies: []string{"EUR", "GBP"}})
	if err != nil || len(qCurrs) != 2 {
		t.Errorf("QueryEvents Currencies only failed: count=%d, err=%v", len(qCurrs), err)
	}

	// 5d. Filter by Impacts only (multiple impacts)
	qImpacts, err := store.QueryEvents(ctx, QueryFilter{Impacts: []tvcalendar.Impact{tvcalendar.ImpactHigh, tvcalendar.ImpactMedium}})
	if err != nil || len(qImpacts) != 2 {
		t.Errorf("QueryEvents Impacts only failed: count=%d, err=%v", len(qImpacts), err)
	}

	// 5e. Combined filter
	filter := QueryFilter{
		StartDate:  &startQuery,
		EndDate:    &endQuery,
		Currencies: []string{"USD"},
		Impacts:    []tvcalendar.Impact{tvcalendar.ImpactHigh},
	}
	highUsdEvents, err := store.QueryEvents(ctx, filter)
	if err != nil {
		t.Fatalf("QueryEvents failed: %v", err)
	}
	if len(highUsdEvents) != 1 || highUsdEvents[0].Title != "FOMC Press Conference" {
		t.Errorf("QueryEvents failed to filter correctly: expected 1 FOMC event, got %d", len(highUsdEvents))
	}

	// 6. Close connection
	if err := store.Close(); err != nil {
		t.Fatalf("Close failed: %v", err)
	}

	// 7. Verify data in sandbox using raw SQL query
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		t.Fatalf("Failed to open verified database: %v", err)
	}
	defer db.Close()

	var title, currency, date, impact string
	var allDay, tentative int
	err = db.QueryRow("SELECT title, currency, date, impact, all_day, tentative FROM events WHERE id = '998877'").
		Scan(&title, &currency, &date, &impact, &allDay, &tentative)
	if err != nil {
		t.Fatalf("Failed to query inserted event: %v", err)
	}

	if title != "FOMC Press Conference" || currency != "USD" || impact != "High" {
		t.Errorf("Ingested data mismatch: %s / %s / %s", title, currency, impact)
	}

	// Verify auto-generated key row exists
	var count int
	err = db.QueryRow("SELECT COUNT(*) FROM events WHERE currency = 'EUR'").Scan(&count)
	if err != nil {
		t.Fatalf("Failed to query auto-generated row: %v", err)
	}
	if count != 1 {
		t.Errorf("Expected 1 EUR row from auto-generation, got %d", count)
	}
}

func TestSQLiteStorageUninitialized(t *testing.T) {
	store := NewSQLiteStorage("uninit.db")
	ctx := context.Background()
	now := time.Now()

	if err := store.SaveEvents(ctx, []tvcalendar.Event{{Title: "Test"}}); err == nil {
		t.Errorf("Expected error when saving on uninitialized store")
	}
	if _, err := store.GetEvents(ctx, now, now); err == nil {
		t.Errorf("Expected error on uninitialized GetEvents")
	}
	if _, err := store.GetEventsByCurrency(ctx, "USD"); err == nil {
		t.Errorf("Expected error on uninitialized GetEventsByCurrency")
	}
	if _, err := store.QueryEvents(ctx, QueryFilter{}); err == nil {
		t.Errorf("Expected error on uninitialized QueryEvents")
	}
	_ = store.Close()
}

func TestSQLiteStorageEmptyEvents(t *testing.T) {
	dbPath := "test_empty.db"
	defer os.Remove(dbPath)

	store := NewSQLiteStorage(dbPath)
	ctx := context.Background()

	if err := store.Init(ctx); err != nil {
		t.Fatalf("Init failed: %v", err)
	}
	defer store.Close()

	if err := store.SaveEvents(ctx, []tvcalendar.Event{}); err != nil {
		t.Fatalf("Expected saving empty events slice to succeed, got %v", err)
	}
}

func TestSQLiteStorageBatchLarge(t *testing.T) {
	dbPath := "test_large_batch.db"
	defer os.Remove(dbPath)

	store := NewSQLiteStorage(dbPath)
	ctx := context.Background()

	if err := store.Init(ctx); err != nil {
		t.Fatalf("Init failed: %v", err)
	}
	defer store.Close()

	var events []tvcalendar.Event
	baseDate := time.Date(2026, time.January, 1, 0, 0, 0, 0, time.UTC)
	for i := 0; i < 250; i++ {
		events = append(events, tvcalendar.Event{
			ID:       fmt.Sprintf("batch-event-%d", i),
			Title:    fmt.Sprintf("Economic Release #%d", i),
			Currency: "USD",
			Date:     baseDate.Add(time.Duration(i) * time.Hour),
			Impact:   tvcalendar.ImpactHigh,
			Forecast: "1.0%",
			Previous: "0.8%",
			Actual:   "1.2%",
		})
	}

	if err := store.SaveEvents(ctx, events); err != nil {
		t.Fatalf("SaveEvents failed on 250 batch events: %v", err)
	}

	queried, err := store.GetEvents(ctx, baseDate, baseDate.Add(300*time.Hour))
	if err != nil {
		t.Fatalf("GetEvents failed: %v", err)
	}
	if len(queried) != 250 {
		t.Errorf("Expected 250 events, got %d", len(queried))
	}
}
