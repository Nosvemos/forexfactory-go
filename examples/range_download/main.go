package main

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/Nosvemos/forexfactory-go/pkg/forexfactory"
	"github.com/Nosvemos/forexfactory-go/pkg/storage"
)

func main() {
	fmt.Println("=== Forex Factory Go SDK: Programmatic Range Download Example ===")

	// 1. Setup client options
	// We configure a rate limit of 2 requests/sec, 4 concurrent worker threads,
	// local time conversions, and a custom programmatic progress callback hook.
	client := forexfactory.NewClient(
		forexfactory.WithRateLimit(2),
		forexfactory.WithConcurrency(4),
		forexfactory.WithTimeLocation(time.Local),
		forexfactory.WithProgressCallback(func(current, total int) {
			pct := (current * 100) / total
			fmt.Printf("[SDK Progress Update] Downloaded week %d/%d (%d%%)\n", current, total, pct)
		}),
	)

	// 2. Define range time boundaries
	start := time.Date(2026, time.May, 1, 0, 0, 0, 0, time.UTC)
	end := time.Date(2026, time.May, 20, 23, 59, 59, 0, time.UTC)

	fmt.Printf("Fetching economic events between %s and %s...\n\n", start.Format("2006-01-02"), end.Format("2006-01-02"))

	ctx := context.Background()

	// 3. Download the range programmatically via the core SDK
	events, err := client.FetchRange(ctx, start, end)
	if err != nil {
		log.Fatalf("Error downloading range: %v", err)
	}

	fmt.Printf("\nSuccessfully downloaded %d events via parallel SDK workers!\n\n", len(events))

	// 4. Initialize the Storage SDK connection
	dbPath := "demo_sdk.db"
	store := storage.NewSQLiteStorage(dbPath)

	if err := store.Init(ctx); err != nil {
		log.Fatalf("Failed to initialize database: %v", err)
	}
	defer store.Close()

	// 5. Save the events to our storage backend
	fmt.Printf("Ingesting %d events into %s database...\n", len(events), dbPath)
	if err := store.SaveEvents(ctx, events); err != nil {
		log.Fatalf("Failed to save events into database: %v", err)
	}

	// 6. Query the database using the new Storage SDK query APIs
	fmt.Println("\n--- Querying DB: High Impact USD Events ---")
	dbEvents, err := store.GetEventsByCurrency(ctx, "USD")
	if err != nil {
		log.Fatalf("Failed to query events by currency: %v", err)
	}

	count := 0
	for _, e := range dbEvents {
		if e.Impact == forexfactory.ImpactHigh {
			fmt.Printf("🔴 [%s] %s (Actual: %s, Forecast: %s)\n", e.Date.Format("2006-01-02 15:04"), e.Title, e.Actual, e.Forecast)
			count++
		}
	}
	if count == 0 {
		fmt.Println("No high impact USD events found in the database for this range.")
	}

	fmt.Println("\n=== SDK Execution Finished Successfully ===")
}
