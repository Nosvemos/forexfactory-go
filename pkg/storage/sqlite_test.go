package storage

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/Nosvemos/forexcalendar-go/pkg/forexcalendar"
)

func TestSQLiteStorage(t *testing.T) {
	dbPath := "test_sandbox.db"
	// Clean up after test
	defer os.Remove(dbPath)

	store := NewSQLiteStorage(dbPath)
	ctx := context.Background()

	// 1. Test Initialization
	if err := store.Init(ctx); err != nil {
		t.Fatalf("Database Init failed: %v", err)
	}

	// 2. Test Ingestion / Saving Events
	testEvents := []forexcalendar.Event{
		{
			ID:          "998877",
			Title:       "FOMC Press Conference",
			Currency:    "USD",
			Date:        time.Date(2026, time.May, 26, 14, 30, 0, 0, time.UTC),
			Impact:      forexcalendar.ImpactHigh,
			Forecast:    "",
			Previous:    "",
			Actual:      "",
			IsAllDay:    false,
			IsTentative: false,
		},
		{
			ID:          "", // Testing auto-generated ID
			Title:       "French CPI m/m",
			Currency:    "EUR",
			Date:        time.Date(2026, time.May, 26, 8, 45, 0, 0, time.UTC),
			Impact:      forexcalendar.ImpactMedium,
			Forecast:    "0.2%",
			Previous:    "0.1%",
			Actual:      "0.3%",
			IsAllDay:    false,
			IsTentative: false,
		},
	}

	if err := store.SaveEvents(ctx, testEvents); err != nil {
		t.Fatalf("SaveEvents failed: %v", err)
	}

	// Test new SDK query functions
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

	// 4.5. Test QueryEvents with dynamic QueryFilter
	filter := QueryFilter{
		StartDate:  &startQuery,
		EndDate:    &endQuery,
		Currencies: []string{"USD"},
		Impacts:    []forexcalendar.Impact{forexcalendar.ImpactHigh},
	}
	highUsdEvents, err := store.QueryEvents(ctx, filter)
	if err != nil {
		t.Fatalf("QueryEvents failed: %v", err)
	}
	if len(highUsdEvents) != 1 || highUsdEvents[0].Title != "FOMC Press Conference" {
		t.Errorf("QueryEvents failed to filter correctly: expected 1 FOMC event, got %d", len(highUsdEvents))
	}

	// 5. Close connection
	if err := store.Close(); err != nil {
		t.Fatalf("Close failed: %v", err)
	}

	// 4. Verify data in sandbox using raw SQL query
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

func TestSQLiteStorageEmptyEvents(t *testing.T) {
	dbPath := "test_empty.db"
	defer os.Remove(dbPath)

	store := NewSQLiteStorage(dbPath)
	ctx := context.Background()

	if err := store.Init(ctx); err != nil {
		t.Fatalf("Init failed: %v", err)
	}
	defer store.Close()

	if err := store.SaveEvents(ctx, []forexcalendar.Event{}); err != nil {
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

	var events []forexcalendar.Event
	baseDate := time.Date(2026, time.January, 1, 0, 0, 0, 0, time.UTC)
	for i := 0; i < 250; i++ {
		events = append(events, forexcalendar.Event{
			ID:       fmt.Sprintf("batch-event-%d", i),
			Title:    fmt.Sprintf("Economic Release #%d", i),
			Currency: "USD",
			Date:     baseDate.Add(time.Duration(i) * time.Hour),
			Impact:   forexcalendar.ImpactHigh,
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
