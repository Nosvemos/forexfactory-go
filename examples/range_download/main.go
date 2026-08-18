package main

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/Nosvemos/forexcalendar-go/pkg/forexcalendar"
	"github.com/Nosvemos/forexcalendar-go/pkg/storage"
)

func main() {
	fmt.Println("=== ForexCalendar Go SDK: Range Download Example ===")

	client := forexcalendar.NewClient(
		forexcalendar.WithRateLimit(10),
		forexcalendar.WithConcurrency(5),
		forexcalendar.WithTimeLocation(time.Local),
		forexcalendar.WithProgressCallback(func(current, total int) {
			pct := (current * 100) / total
			fmt.Printf("[SDK Progress] Downloaded month %d/%d (%d%%)\n", current, total, pct)
		}),
	)

	start := time.Date(2024, time.January, 1, 0, 0, 0, 0, time.UTC)
	end := time.Date(2024, time.March, 31, 23, 59, 59, 0, time.UTC)

	fmt.Printf("Fetching economic events between %s and %s...\n\n", start.Format("2006-01-02"), end.Format("2006-01-02"))

	ctx := context.Background()
	events, err := client.FetchRange(ctx, start, end)
	if err != nil {
		log.Fatalf("Error downloading range: %v", err)
	}

	fmt.Printf("\nSuccessfully downloaded %d events via parallel SDK workers!\n\n", len(events))

	dbPath := "demo_calendar.db"
	store := storage.NewSQLiteStorage(dbPath)

	if err := store.Init(ctx); err != nil {
		log.Fatalf("Failed to initialize database: %v", err)
	}
	defer store.Close()

	fmt.Printf("Ingesting %d events into %s database...\n", len(events), dbPath)
	if err := store.SaveEvents(ctx, events); err != nil {
		log.Fatalf("Failed to save events into database: %v", err)
	}

	fmt.Println("\n--- Querying DB: High Impact USD Events ---")
	dbEvents, err := store.GetEventsByCurrency(ctx, "USD")
	if err != nil {
		log.Fatalf("Failed to query events by currency: %v", err)
	}

	count := 0
	for _, e := range dbEvents {
		if e.Impact == forexcalendar.ImpactHigh {
			fmt.Printf("🔴 [%s] %s (Actual: %s, Forecast: %s)\n", e.Date.Format("2006-01-02 15:04"), e.Title, e.Actual, e.Forecast)
			count++
		}
	}
	if count == 0 {
		fmt.Println("No high impact USD events found in the database for this range.")
	}

	fmt.Println("\n=== SDK Execution Finished Successfully ===")
}
