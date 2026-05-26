package storage

import (
	"context"
	"database/sql"
	"os"
	"testing"
	"time"

	"github.com/Nosvemos/forexfactory-go/pkg/forexfactory"
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
	testEvents := []forexfactory.Event{
		{
			ID:          "998877",
			Title:       "FOMC Press Conference",
			Country:     "USD",
			Date:        time.Date(2026, time.May, 26, 14, 30, 0, 0, time.UTC),
			Impact:      forexfactory.ImpactHigh,
			Forecast:    "",
			Previous:    "",
			Actual:      "",
			IsAllDay:    false,
			IsTentative: false,
		},
		{
			ID:          "", // Testing auto-generated ID
			Title:       "French CPI m/m",
			Country:     "EUR",
			Date:        time.Date(2026, time.May, 26, 8, 45, 0, 0, time.UTC),
			Impact:      forexfactory.ImpactMedium,
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

	// 4. Test GetEventsByCountry
	usdEvents, err := store.GetEventsByCountry(ctx, "usd")
	if err != nil {
		t.Fatalf("GetEventsByCountry failed: %v", err)
	}
	if len(usdEvents) != 1 || usdEvents[0].Title != "FOMC Press Conference" {
		t.Errorf("GetEventsByCountry failed to return USD events correctly")
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

	var title, country, date, impact string
	var allDay, tentative int
	err = db.QueryRow("SELECT title, country, date, impact, all_day, tentative FROM events WHERE id = '998877'").
		Scan(&title, &country, &date, &impact, &allDay, &tentative)
	if err != nil {
		t.Fatalf("Failed to query inserted event: %v", err)
	}

	if title != "FOMC Press Conference" || country != "USD" || impact != "High" {
		t.Errorf("Ingested data mismatch: %s / %s / %s", title, country, impact)
	}

	// Verify auto-generated key row exists
	var count int
	err = db.QueryRow("SELECT COUNT(*) FROM events WHERE country = 'EUR'").Scan(&count)
	if err != nil {
		t.Fatalf("Failed to query auto-generated row: %v", err)
	}
	if count != 1 {
		t.Errorf("Expected 1 EUR row from auto-generation, got %d", count)
	}
}
