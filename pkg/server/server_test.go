package server

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestServerHealthAndStatsAndCors(t *testing.T) {
	srv := NewServer(Config{
		Addr:      ":0",
		RateLimit: 10,
	})

	handler := srv.Handler()

	// 1. Test /health
	reqHealth := httptest.NewRequest("GET", "/health", nil)
	recHealth := httptest.NewRecorder()
	handler.ServeHTTP(recHealth, reqHealth)

	if recHealth.Code != http.StatusOK {
		t.Errorf("Expected status 200 for /health, got %d", recHealth.Code)
	}

	var healthRes map[string]interface{}
	if err := json.NewDecoder(recHealth.Body).Decode(&healthRes); err != nil {
		t.Fatalf("Failed to decode /health JSON: %v", err)
	}

	if healthRes["status"] != "ok" {
		t.Errorf("Expected status 'ok', got %v", healthRes["status"])
	}

	// 2. Test CORS headers
	if origin := recHealth.Header().Get("Access-Control-Allow-Origin"); origin != "*" {
		t.Errorf("Expected CORS header '*', got %q", origin)
	}

	// 3. Test /api/v1/stats
	reqStats := httptest.NewRequest("GET", "/api/v1/stats", nil)
	recStats := httptest.NewRecorder()
	handler.ServeHTTP(recStats, reqStats)

	if recStats.Code != http.StatusOK {
		t.Errorf("Expected status 200 for /api/v1/stats, got %d", recStats.Code)
	}

	var statsRes map[string]interface{}
	if err := json.NewDecoder(recStats.Body).Decode(&statsRes); err != nil {
		t.Fatalf("Failed to decode /api/v1/stats JSON: %v", err)
	}

	if statsRes["total_requests"] == nil {
		t.Errorf("Expected total_requests field in stats")
	}

	// 4. Test /api/v1/calendar missing params validation
	reqCalMissing := httptest.NewRequest("GET", "/api/v1/calendar", nil)
	recCalMissing := httptest.NewRecorder()
	handler.ServeHTTP(recCalMissing, reqCalMissing)

	if recCalMissing.Code != http.StatusBadRequest {
		t.Errorf("Expected status 400 for missing calendar params, got %d", recCalMissing.Code)
	}
}
