package main

import (
	"strings"
	"testing"

	"github.com/Nosvemos/tradingview-calendar-go/pkg/tvcalendar"
)

func TestNotifierImpactWeights(t *testing.T) {
	if !isImpactAllowed(tvcalendar.ImpactHigh, tvcalendar.ImpactMedium) {
		t.Errorf("Expected High impact to be allowed when minimum is Medium")
	}
	if isImpactAllowed(tvcalendar.ImpactLow, tvcalendar.ImpactHigh) {
		t.Errorf("Expected Low impact NOT to be allowed when minimum is High")
	}
	if !isImpactAllowed(tvcalendar.ImpactHigh, tvcalendar.ImpactHigh) {
		t.Errorf("Expected High impact to be allowed when minimum is High")
	}
	if !isImpactAllowed(tvcalendar.ImpactLow, tvcalendar.ImpactNone) {
		t.Errorf("Expected Low impact to be allowed when minimum is None")
	}
}

func TestNotifierCachePath(t *testing.T) {
	path := getNotifierCacheFilePath()
	if !strings.Contains(path, "tradingview-calendar-go") || !strings.HasSuffix(path, "notified_cache.json") {
		t.Errorf("Unexpected notifier cache file path: %s", path)
	}
}

func TestNotifierRootCmd(t *testing.T) {
	if rootCmd.Use != "tv-notifier" {
		t.Errorf("Expected rootCmd use 'tv-notifier', got %s", rootCmd.Use)
	}
}
