package main

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/Nosvemos/tradingview-calendar-go/pkg/tvcalendar"
)

func main() {
	// 1. Initialize Client
	client := tvcalendar.NewClient(
		tvcalendar.WithRateLimit(10),
		tvcalendar.WithTimeLocation(time.Local),
	)

	// 2. Specify target date
	targetDate := time.Date(2025, time.January, 15, 0, 0, 0, 0, time.UTC)
	fmt.Printf("Fetching economic calendar for week of: %s...\n\n", targetDate.Format("2006-01-02"))

	// 3. Retrieve events
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
		}

		fmt.Printf("[%02d] [%s] %-2s | %-3s | %-6s | %s\n", i+1, timeStr, e.Country, e.Currency, e.Impact, e.Title)
		if e.Actual != "" || e.Forecast != "" || e.Previous != "" {
			fmt.Printf("     └─ Actual: %-8s | Forecast: %-8s | Previous: %s\n", e.Actual, e.Forecast, e.Previous)
		}
	}
	fmt.Println("--------------------------------------------------------------------------------")
}
