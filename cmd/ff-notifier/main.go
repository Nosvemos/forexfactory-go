package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"os"
	"os/signal"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/Nosvemos/forexfactory-go/pkg/forexfactory"
	"github.com/spf13/cobra"
)

var (
	discordWebhookFlag string
	telegramTokenFlag   string
	telegramChatFlag    string
	pollingIntervalFlag time.Duration
	leadTimeFlag        time.Duration
	minImpactFlag       string

	// InMemory notified events cache to prevent double-alerts
	notifiedCache = make(map[string]time.Time)
	cacheMu       sync.Mutex
)

var rootCmd = &cobra.Command{
	Use:   "ff-notifier",
	Short: "ff-notifier: Real-time Discord/Telegram Economic News Alert Daemon",
	Long: `ff-notifier is an enterprise-grade economic calendar alert daemon that polls 
the live Forex Factory XML feed and dispatches rich card alerts to Discord and Telegram channels 
exactly 15 minutes (or customizable lead time) before high-impact economic news releases.`,
	Run: func(cmd *cobra.Command, args []string) {
		executeNotifier()
	},
}

func init() {
	rootCmd.Flags().StringVar(&discordWebhookFlag, "discord-webhook", "", "Discord Webhook URL for rich channel alerts")
	rootCmd.Flags().StringVar(&telegramTokenFlag, "telegram-token", "", "Telegram Bot API Token (e.g. 123456789:ABCDef...)")
	rootCmd.Flags().StringVar(&telegramChatFlag, "telegram-chat", "", "Telegram Channel or Group Chat ID to send alerts to")
	rootCmd.Flags().DurationVarP(&pollingIntervalFlag, "interval", "i", 60*time.Second, "Polling interval to check for calendar updates")
	rootCmd.Flags().DurationVarP(&leadTimeFlag, "lead-time", "l", 15*time.Minute, "Lead time to send alert before the event release")
	rootCmd.Flags().StringVar(&minImpactFlag, "min-impact", "High", "Minimum impact level to alert (High, Medium, Low)")
}

func main() {
	log.SetFlags(log.LstdFlags | log.Lshortfile)
	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}

func executeNotifier() {
	// Validate endpoints
	if discordWebhookFlag == "" && (telegramTokenFlag == "" || telegramChatFlag == "") {
		log.Fatalf("Error: You must configure at least one notification endpoint (--discord-webhook or --telegram-token & --telegram-chat)")
	}

	// Map minimum impact
	minImpact := forexfactory.ImpactHigh
	switch strings.ToLower(minImpactFlag) {
	case "medium":
		minImpact = forexfactory.ImpactMedium
	case "low":
		minImpact = forexfactory.ImpactLow
	}

	fmt.Println("\033[1;36m================================================================================\033[0m")
	fmt.Println("\033[1;35m  FOREX FACTORY REAL-TIME NEWS ALERTS DAEMON STARTED  \033[0m")
	fmt.Printf("  • Polling Interval : %v\n", pollingIntervalFlag)
	fmt.Printf("  • Notification Lead: %v\n", leadTimeFlag)
	fmt.Printf("  • Minimum Impact   : %s\n", minImpactFlag)
	if discordWebhookFlag != "" {
		fmt.Println("  • Discord Webhook  : Configured ✅")
	}
	if telegramTokenFlag != "" {
		fmt.Println("  • Telegram Bot     : Configured ✅")
	}
	fmt.Println("\033[1;36m================================================================================\033[0m")
	fmt.Println("Listening for upcoming high-volatility events... Press Ctrl+C to terminate.")

	// Instantiate standard client forcing UTC alignment
	client := forexfactory.NewClient(
		forexfactory.WithTimeLocation(time.UTC),
	)

	// Clean cache worker to prevent memory bloat over days
	go runCacheCleaner()

	// Capture interrupt signals
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	ticker := time.NewTicker(pollingIntervalFlag)
	defer ticker.Stop()

	// Run initial check
	checkAndAlert(client, minImpact)

	for {
		select {
		case <-sigChan:
			fmt.Println("\nGracefully shutting down alert daemon...")
			return
		case <-ticker.C:
			checkAndAlert(client, minImpact)
		}
	}
}

func checkAndAlert(client *forexfactory.Client, minImpact forexfactory.Impact) {
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()

	events, err := client.FetchLiveFeed(ctx)
	if err != nil {
		log.Printf("Error pulling live feed: %v\n", err)
		return
	}

	now := time.Now().UTC()
	alertThreshold := now.Add(leadTimeFlag)

	for _, e := range events {
		// Filter out based on impact importance
		if !isImpactAllowed(e.Impact, minImpact) {
			continue
		}

		// Skip all day or tentative events since they do not have a concrete hourly release
		if e.IsAllDay || e.IsTentative {
			continue
		}

		// Target release time
		releaseTime := e.Date.UTC()

		// Generate a unique cache key based on timestamp, country, and title
		cacheKey := fmt.Sprintf("%d-%s-%s", releaseTime.Unix(), e.Country, strings.ToLower(e.Title))

		cacheMu.Lock()
		_, alreadyNotified := notifiedCache[cacheKey]
		cacheMu.Unlock()

		if alreadyNotified {
			continue
		}

		// Trigger alert if event is in the upcoming window: now <= releaseTime <= alertThreshold
		if (releaseTime.After(now) || releaseTime.Equal(now)) && (releaseTime.Before(alertThreshold) || releaseTime.Equal(alertThreshold)) {
			// Alert upcoming event
			minutesRemaining := int(releaseTime.Sub(now).Minutes())
			fmt.Printf("[%s] Upcoming Event Alert in %d minutes: %s | %s | %s\n", 
				time.Now().Format("15:04:05"), minutesRemaining, e.Country, string(e.Impact), e.Title)

			var wg sync.WaitGroup
			if discordWebhookFlag != "" {
				wg.Add(1)
				go func(event forexfactory.Event, rem int) {
					defer wg.Done()
					if err := sendDiscordAlert(event, rem); err != nil {
						log.Printf("Discord Alert failed: %v", err)
					}
				}(e, minutesRemaining)
			}

			if telegramTokenFlag != "" && telegramChatFlag != "" {
				wg.Add(1)
				go func(event forexfactory.Event, rem int) {
					defer wg.Done()
					if err := sendTelegramAlert(event, rem); err != nil {
						log.Printf("Telegram Alert failed: %v", err)
					}
				}(e, minutesRemaining)
			}

			wg.Wait()

			// Add to cache to prevent duplicate fires
			cacheMu.Lock()
			notifiedCache[cacheKey] = releaseTime
			cacheMu.Unlock()
		}
	}
}

func isImpactAllowed(imp forexfactory.Impact, min forexfactory.Impact) bool {
	weight := func(i forexfactory.Impact) int {
		switch i {
		case forexfactory.ImpactHigh:
			return 3
		case forexfactory.ImpactMedium:
			return 2
		case forexfactory.ImpactLow:
			return 1
		default:
			return 0
		}
	}
	return weight(imp) >= weight(min)
}

func runCacheCleaner() {
	ticker := time.NewTicker(2 * time.Hour)
	for range ticker.C {
		now := time.Now().UTC()
		cacheMu.Lock()
		for key, releaseTime := range notifiedCache {
			// Evict events that passed more than 1 hour ago
			if now.Sub(releaseTime) > 1*time.Hour {
				delete(notifiedCache, key)
			}
		}
		cacheMu.Unlock()
	}
}

func sendDiscordAlert(e forexfactory.Event, minutesLeft int) error {
	color := 9807270 // Gray
	emoji := "⚪"
	switch e.Impact {
	case forexfactory.ImpactHigh:
		color = 15158332 // Red
		emoji = "🔴"
	case forexfactory.ImpactMedium:
		color = 15105570 // Orange
		emoji = "🟡"
	case forexfactory.ImpactLow:
		color = 3066993 // Green
		emoji = "🟢"
	}

	timeFormatted := e.Date.UTC().Format("15:04 UTC")

	payload := map[string]interface{}{
		"embeds": []map[string]interface{}{
			{
				"title":       fmt.Sprintf("%s Economic Event Warning", emoji),
				"description": fmt.Sprintf("**%s** is releasing in **%d minutes**!", e.Title, minutesLeft),
				"color":       color,
				"fields": []map[string]interface{}{
					{"name": "Currency", "value": e.Country, "inline": true},
					{"name": "Volatility threat", "value": string(e.Impact), "inline": true},
					{"name": "Scheduled Time", "value": timeFormatted, "inline": true},
					{"name": "Consensus forecast", "value": getValOrDash(e.Forecast), "inline": true},
					{"name": "Previous value", "value": getValOrDash(e.Previous), "inline": true},
				},
				"footer": map[string]string{
					"text": "ForexFactory Go SDK • Alerts",
				},
				"timestamp": time.Now().UTC().Format(time.RFC3339),
			},
		},
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return err
	}

	resp, err := http.Post(discordWebhookFlag, "application/json", bytes.NewBuffer(body))
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusNoContent {
		return fmt.Errorf("bad status response from discord: %s", resp.Status)
	}

	return nil
}

func sendTelegramAlert(e forexfactory.Event, minutesLeft int) error {
	emoji := "⚪"
	switch e.Impact {
	case forexfactory.ImpactHigh:
		emoji = "🔴"
	case forexfactory.ImpactMedium:
		emoji = "🟡"
	case forexfactory.ImpactLow:
		emoji = "🟢"
	}

	message := fmt.Sprintf(
		"%s *ECONOMIC EVENT WARNING*\n\n"+
			"*Event:* %s\n"+
			"*Releasing in:* %d minutes\n\n"+
			"• *Currency:* %s\n"+
			"• *Scheduled Time:* %s\n"+
			"• *Impact Level:* %s\n"+
			"• *Analyst Forecast:* %s\n"+
			"• *Previous Value:* %s\n\n"+
			"_Powered by forexfactory-go_",
		emoji,
		escapeMarkdown(e.Title),
		minutesLeft,
		escapeMarkdown(e.Country),
		escapeMarkdown(e.Date.UTC().Format("15:04 UTC")),
		escapeMarkdown(string(e.Impact)),
		escapeMarkdown(getValOrDash(e.Forecast)),
		escapeMarkdown(getValOrDash(e.Previous)),
	)

	apiURL := fmt.Sprintf("https://api.telegram.org/bot%s/sendMessage", telegramTokenFlag)
	formData := url.Values{
		"chat_id":    {telegramChatFlag},
		"text":       {message},
		"parse_mode": {"Markdown"},
	}

	resp, err := http.PostForm(apiURL, formData)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("bad status response from telegram: %s", resp.Status)
	}

	return nil
}

func getValOrDash(v string) string {
	if v == "" {
		return "-"
	}
	return v
}

func escapeMarkdown(text string) string {
	replacer := strings.NewReplacer(
		"_", "\\_",
		"*", "\\*",
		"[", "\\[",
		"`", "\\`",
	)
	return replacer.Replace(text)
}
