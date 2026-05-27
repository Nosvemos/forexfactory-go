package main

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/Nosvemos/forexfactory-go/pkg/forexfactory"
)

func main() {
	// 1. Initialize Client
	// Set to use UTC time location for standardized comparisons
	client := forexfactory.NewClient(
		forexfactory.WithTimeLocation(time.UTC),
	)

	fmt.Println("Starting real-time economic calendar tracker (Press Ctrl+C to exit)...")
	fmt.Println("--------------------------------------------------------------------------------")

	// 2. Fetch and print feed initially
	updateLiveFeed(client)

	// 3. Setup a tick loop (e.g. check every 30 seconds)
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	for range ticker.C {
		fmt.Printf("\n[%s] Checking for economic feed updates...\n", time.Now().Format("15:04:05"))
		updateLiveFeed(client)
	}
}

func updateLiveFeed(client *forexfactory.Client) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	events, err := client.FetchLiveFeed(ctx)
	if err != nil {
		log.Printf("Error: failed to fetch live feed: %v", err)
		return
	}

	fmt.Printf("Fetched %d current week events from live XML feed:\n", len(events))
	count := 0
	for _, e := range events {
		// Only display high-impact events for preview
		if e.Impact == forexfactory.ImpactHigh {
			timeStr := e.Date.Format("2006-01-02 15:04 UTC")
			fmt.Printf("🔴 [%s] %-3s | %s\n", timeStr, e.Currency, e.Title)
			if e.Actual != "" || e.Forecast != "" {
				fmt.Printf("   └─ Actual: %-8s | Forecast: %-8s | Previous: %s\n", e.Actual, e.Forecast, e.Previous)
			}
			count++
		}
	}

	if count == 0 {
		fmt.Println("No high-impact events found in this week's live feed.")
	}
}
