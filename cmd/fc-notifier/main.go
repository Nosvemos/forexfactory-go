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
	"path/filepath"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/Nosvemos/forexcalendar-go/pkg/forexcalendar"
	"github.com/spf13/cobra"
)

var (
	discordWebhookFlag  string
	slackWebhookFlag    string
	genericWebhookFlag  string
	telegramTokenFlag   string
	telegramChatFlag    string
	pollingIntervalFlag time.Duration
	leadTimeFlag        time.Duration
	minImpactFlag       string

	notifiedCache = make(map[string]time.Time)
	cacheMu       sync.Mutex
)

var rootCmd = &cobra.Command{
	Use:   "fc-notifier",
	Short: "fc-notifier: Real-time Discord/Slack/Telegram Economic News Alert Daemon",
	Long: `fc-notifier is an economic calendar alert daemon that polls 
the live economic feed and dispatches rich card alerts to Discord, Slack, Telegram, and generic webhooks 
before high-impact macroeconomic news releases.`,
	Run: func(cmd *cobra.Command, args []string) {
		executeNotifier()
	},
}

func init() {
	rootCmd.Flags().StringVar(&discordWebhookFlag, "discord-webhook", "", "Discord Webhook URL for rich channel alerts")
	rootCmd.Flags().StringVar(&slackWebhookFlag, "slack-webhook", "", "Slack Incoming Webhook URL for channel alerts")
	rootCmd.Flags().StringVar(&genericWebhookFlag, "generic-webhook", "", "Generic HTTP POST Webhook URL for custom alert ingestion")
	rootCmd.Flags().StringVar(&telegramTokenFlag, "telegram-token", "", "Telegram Bot API Token")
	rootCmd.Flags().StringVar(&telegramChatFlag, "telegram-chat", "", "Telegram Channel or Group Chat ID")
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
	if discordWebhookFlag == "" && (telegramTokenFlag == "" || telegramChatFlag == "") && slackWebhookFlag == "" && genericWebhookFlag == "" {
		log.Fatalf("Error: You must configure at least one notification endpoint (--discord-webhook, --telegram-token/chat, --slack-webhook, or --generic-webhook)")
	}

	minImpact := forexcalendar.ImpactHigh
	switch strings.ToLower(minImpactFlag) {
	case "medium":
		minImpact = forexcalendar.ImpactMedium
	case "low":
		minImpact = forexcalendar.ImpactLow
	}

	fmt.Println("\033[1;36m================================================================================\033[0m")
	fmt.Println("\033[1;35m  FOREX CALENDAR REAL-TIME NEWS ALERTS DAEMON STARTED  \033[0m")
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

	client := forexcalendar.NewClient(
		forexcalendar.WithTimeLocation(time.UTC),
	)
	defer client.Close()

	loadNotifiedCache()
	go runCacheCleaner()

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	ticker := time.NewTicker(pollingIntervalFlag)
	defer ticker.Stop()

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

func checkAndAlert(client *forexcalendar.Client, minImpact forexcalendar.Impact) {
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	events, err := client.FetchLiveFeed(ctx)
	if err != nil {
		log.Printf("Error pulling live feed: %v\n", err)
		return
	}

	now := time.Now().UTC()
	alertThreshold := now.Add(leadTimeFlag)

	for _, e := range events {
		if !isImpactAllowed(e.Impact, minImpact) {
			continue
		}
		if e.IsAllDay || e.IsTentative {
			continue
		}

		releaseTime := e.Date.UTC()
		cacheKey := fmt.Sprintf("%d-%s-%s", releaseTime.Unix(), e.Currency, strings.ToLower(e.Title))

		cacheMu.Lock()
		_, alreadyNotified := notifiedCache[cacheKey]
		cacheMu.Unlock()

		if alreadyNotified {
			continue
		}

		if (releaseTime.After(now) || releaseTime.Equal(now)) && (releaseTime.Before(alertThreshold) || releaseTime.Equal(alertThreshold)) {
			minutesRemaining := int(releaseTime.Sub(now).Minutes())
			fmt.Printf("[%s] Upcoming Event Alert in %d minutes: %s | %s | %s\n",
				time.Now().Format("15:04:05"), minutesRemaining, e.Currency, string(e.Impact), e.Title)

			var wg sync.WaitGroup
			if discordWebhookFlag != "" {
				wg.Add(1)
				go func(event forexcalendar.Event, rem int) {
					defer wg.Done()
					_ = sendDiscordAlert(event, rem)
				}(e, minutesRemaining)
			}

			if slackWebhookFlag != "" {
				wg.Add(1)
				go func(event forexcalendar.Event, rem int) {
					defer wg.Done()
					_ = sendSlackAlert(event, rem)
				}(e, minutesRemaining)
			}

			if genericWebhookFlag != "" {
				wg.Add(1)
				go func(event forexcalendar.Event, rem int) {
					defer wg.Done()
					_ = sendGenericWebhookAlert(event, rem)
				}(e, minutesRemaining)
			}

			if telegramTokenFlag != "" && telegramChatFlag != "" {
				wg.Add(1)
				go func(event forexcalendar.Event, rem int) {
					defer wg.Done()
					_ = sendTelegramAlert(event, rem)
				}(e, minutesRemaining)
			}

			wg.Wait()

			cacheMu.Lock()
			notifiedCache[cacheKey] = releaseTime
			cacheMu.Unlock()

			saveNotifiedCache()
		}
	}
}

func isImpactAllowed(imp forexcalendar.Impact, min forexcalendar.Impact) bool {
	weight := func(i forexcalendar.Impact) int {
		switch i {
		case forexcalendar.ImpactHigh:
			return 3
		case forexcalendar.ImpactMedium:
			return 2
		case forexcalendar.ImpactLow:
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
		changed := false
		for key, releaseTime := range notifiedCache {
			if now.Sub(releaseTime) > 1*time.Hour {
				delete(notifiedCache, key)
				changed = true
			}
		}
		cacheMu.Unlock()

		if changed {
			saveNotifiedCache()
		}
	}
}

func getNotifierCacheFilePath() string {
	cacheDir, err := os.UserCacheDir()
	if err != nil {
		cacheDir = os.TempDir()
	}
	dir := filepath.Join(cacheDir, "forexcalendar-go")
	_ = os.MkdirAll(dir, 0700)
	return filepath.Join(dir, "notified_cache.json")
}

func loadNotifiedCache() {
	path := getNotifierCacheFilePath()
	data, err := os.ReadFile(path)
	if err != nil {
		return
	}

	cacheMu.Lock()
	defer cacheMu.Unlock()

	var tempMap map[string]string
	if err := json.Unmarshal(data, &tempMap); err != nil {
		return
	}

	for k, v := range tempMap {
		if t, err := time.Parse(time.RFC3339, v); err == nil {
			notifiedCache[k] = t
		}
	}
}

func saveNotifiedCache() {
	cacheMu.Lock()
	defer cacheMu.Unlock()

	tempMap := make(map[string]string)
	for k, v := range notifiedCache {
		tempMap[k] = v.Format(time.RFC3339)
	}

	data, err := json.Marshal(tempMap)
	if err != nil {
		return
	}

	path := getNotifierCacheFilePath()
	_ = os.WriteFile(path, data, 0600)
}

var webhookHTTPClient = &http.Client{
	Timeout: 10 * time.Second,
}

func sendDiscordAlert(e forexcalendar.Event, minutesLeft int) error {
	color := 9807270 // Gray
	emoji := "⚪"
	switch e.Impact {
	case forexcalendar.ImpactHigh:
		color = 15158332 // Red
		emoji = "🔴"
	case forexcalendar.ImpactMedium:
		color = 15105570 // Orange
		emoji = "🟡"
	case forexcalendar.ImpactLow:
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
					{"name": "Currency", "value": e.Currency, "inline": true},
					{"name": "Volatility threat", "value": string(e.Impact), "inline": true},
					{"name": "Scheduled Time", "value": timeFormatted, "inline": true},
					{"name": "Consensus forecast", "value": getValOrDash(e.Forecast), "inline": true},
					{"name": "Previous value", "value": getValOrDash(e.Previous), "inline": true},
				},
				"footer": map[string]string{
					"text": "ForexCalendar Go SDK • Alerts",
				},
				"timestamp": time.Now().UTC().Format(time.RFC3339),
			},
		},
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return err
	}

	resp, err := webhookHTTPClient.Post(discordWebhookFlag, "application/json", bytes.NewBuffer(body))
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusNoContent {
		return fmt.Errorf("bad status response from discord: %s", resp.Status)
	}

	return nil
}

func sendTelegramAlert(e forexcalendar.Event, minutesLeft int) error {
	emoji := "⚪"
	switch e.Impact {
	case forexcalendar.ImpactHigh:
		emoji = "🔴"
	case forexcalendar.ImpactMedium:
		emoji = "🟡"
	case forexcalendar.ImpactLow:
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
			"_Powered by forexcalendar-go_",
		emoji,
		escapeMarkdown(e.Title),
		minutesLeft,
		escapeMarkdown(e.Currency),
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

	resp, err := webhookHTTPClient.PostForm(apiURL, formData)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("bad status response from telegram: %s", resp.Status)
	}

	return nil
}

func sendSlackAlert(e forexcalendar.Event, minutesLeft int) error {
	emoji := ":white_circle:"
	switch e.Impact {
	case forexcalendar.ImpactHigh:
		emoji = ":red_circle:"
	case forexcalendar.ImpactMedium:
		emoji = ":large_orange_circle:"
	case forexcalendar.ImpactLow:
		emoji = ":large_green_circle:"
	}

	payload := map[string]interface{}{
		"blocks": []map[string]interface{}{
			{
				"type": "header",
				"text": map[string]string{
					"type": "plain_text",
					"text": fmt.Sprintf("%s Economic Event Warning", emoji),
				},
			},
			{
				"type": "section",
				"text": map[string]string{
					"type": "mrkdwn",
					"text": fmt.Sprintf("*%s* is releasing in *%d minutes*!", e.Title, minutesLeft),
				},
			},
			{
				"type": "section",
				"fields": []map[string]string{
					{"type": "mrkdwn", "text": fmt.Sprintf("*Currency:*\n%s", e.Currency)},
					{"type": "mrkdwn", "text": fmt.Sprintf("*Impact:*\n%s", string(e.Impact))},
					{"type": "mrkdwn", "text": fmt.Sprintf("*Scheduled Time:*\n%s", e.Date.UTC().Format("15:04 UTC"))},
					{"type": "mrkdwn", "text": fmt.Sprintf("*Forecast:*\n%s", getValOrDash(e.Forecast))},
					{"type": "mrkdwn", "text": fmt.Sprintf("*Previous:*\n%s", getValOrDash(e.Previous))},
				},
			},
		},
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return err
	}

	resp, err := webhookHTTPClient.Post(slackWebhookFlag, "application/json", bytes.NewBuffer(body))
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusNoContent {
		return fmt.Errorf("bad status response from slack: %s", resp.Status)
	}

	return nil
}

func sendGenericWebhookAlert(e forexcalendar.Event, minutesLeft int) error {
	payload := map[string]interface{}{
		"event":        e,
		"minutes_left": minutesLeft,
		"timestamp":    time.Now().UTC().Unix(),
		"source":       "forexcalendar-go",
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return err
	}

	resp, err := webhookHTTPClient.Post(genericWebhookFlag, "application/json", bytes.NewBuffer(body))
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("bad status response from generic webhook: %s", resp.Status)
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
