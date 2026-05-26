package main

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/Nosvemos/forexfactory-go/pkg/forexfactory"
)

func main() {
	// 1. Initialize a Client
	// Configured with rate-limiting of 1 request/second and target timezone set to local
	client := forexfactory.NewClient(
		forexfactory.WithRateLimit(1),
		forexfactory.WithTimeLocation(time.Local),
	)

	// 2. Specify the target date (e.g., May 26, 2026)
	// FetchWeek will fetch the whole week containing this day (running Sunday to Saturday)
	targetDate := time.Date(2026, time.May, 26, 0, 0, 0, 0, time.UTC)
	fmt.Printf("Fetching week containing: %s...\n\n", targetDate.Format("2006-01-02"))

	// 3. Retrieve events from Forex Factory
	events, err := client.FetchWeek(context.Background(), targetDate)
	if err != nil {
		log.Fatalf("Error fetching calendar: %v", err)
	}

	// 4. Print events
	fmt.Printf("Successfully fetched %d events:\n", len(events))
	fmt.Println("--------------------------------------------------------------------------------")
	for i, e := range events {
		timeStr := e.Date.Format("2006-01-02 15:04")
		if e.IsAllDay {
			timeStr = e.Date.Format("2006-01-02") + " [All Day]"
		} else if e.IsTentative {
			timeStr = e.Date.Format("2006-01-02") + " [Tentative]"
		}

		fmt.Printf("[%02d] [%s] %-3s | %-6s | %s\n", i+1, timeStr, e.Country, e.Impact, e.Title)
		if e.Actual != "" || e.Forecast != "" || e.Previous != "" {
			fmt.Printf("     └─ Actual: %-8s | Forecast: %-8s | Previous: %s\n", e.Actual, e.Forecast, e.Previous)
		}
	}
	fmt.Println("--------------------------------------------------------------------------------")
}
