package main

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/Nosvemos/forexcalendar-go/pkg/forexcalendar"
)

func main() {
	client := forexcalendar.NewClient(
		forexcalendar.WithTimeLocation(time.UTC),
	)

	fmt.Println("Starting real-time economic calendar tracker (Press Ctrl+C to exit)...")
	fmt.Println("--------------------------------------------------------------------------------")

	updateLiveFeed(client)

	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	for range ticker.C {
		fmt.Printf("\n[%s] Checking for economic feed updates...\n", time.Now().Format("15:04:05"))
		updateLiveFeed(client)
	}
}

func updateLiveFeed(client *forexcalendar.Client) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	events, err := client.FetchLiveFeed(ctx)
	if err != nil {
		log.Printf("Error: failed to fetch live feed: %v", err)
		return
	}

	fmt.Printf("Fetched %d current week events from live feed:\n", len(events))
	count := 0
	for _, e := range events {
		if e.Impact == forexcalendar.ImpactHigh {
			timeStr := e.Date.Format("2006-01-02 15:04 UTC")
			fmt.Printf("🔴 [%s] %-2s | %-3s | %s\n", timeStr, e.Country, e.Currency, e.Title)
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
