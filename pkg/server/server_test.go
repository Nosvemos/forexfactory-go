package server

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/Nosvemos/tradingview-calendar-go/pkg/tvcalendar"
	"github.com/gorilla/websocket"
)

type roundTripFunc func(req *http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return f(req)
}

func TestServerHealthAndStatsAndCors(t *testing.T) {
	srv := NewServer(Config{
		Addr:        ":0",
		RateLimit:   10,
		Concurrency: 2,
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

	// 2. Test CORS and Security headers
	if origin := recHealth.Header().Get("Access-Control-Allow-Origin"); origin != "*" {
		t.Errorf("Expected CORS header '*', got %q", origin)
	}
	if xcto := recHealth.Header().Get("X-Content-Type-Options"); xcto != "nosniff" {
		t.Errorf("Expected X-Content-Type-Options: nosniff, got %q", xcto)
	}

	// 3. Test OPTIONS preflight
	reqOptions := httptest.NewRequest("OPTIONS", "/health", nil)
	recOptions := httptest.NewRecorder()
	handler.ServeHTTP(recOptions, reqOptions)
	if recOptions.Code != http.StatusOK {
		t.Errorf("Expected status 200 for OPTIONS, got %d", recOptions.Code)
	}

	// 4. Test /metrics (Prometheus)
	reqMetrics := httptest.NewRequest("GET", "/metrics", nil)
	recMetrics := httptest.NewRecorder()
	handler.ServeHTTP(recMetrics, reqMetrics)

	if recMetrics.Code != http.StatusOK {
		t.Errorf("Expected status 200 for /metrics, got %d", recMetrics.Code)
	}
	metricsBody := recMetrics.Body.String()
	if !strings.Contains(metricsBody, "tvcalendar_requests_total") {
		t.Errorf("Metrics output missing tvcalendar_requests_total: %s", metricsBody)
	}

	// 5. Test /api/v1/stats
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

	// 6. Test /api/v1/calendar invalid requests
	reqMissingParams := httptest.NewRequest("GET", "/api/v1/calendar", nil)
	recMissingParams := httptest.NewRecorder()
	handler.ServeHTTP(recMissingParams, reqMissingParams)
	if recMissingParams.Code != http.StatusBadRequest {
		t.Errorf("Expected status 400 for missing params, got %d", recMissingParams.Code)
	}

	reqInvalidDate := httptest.NewRequest("GET", "/api/v1/calendar?start=invalid&end=2025-01-01", nil)
	recInvalidDate := httptest.NewRecorder()
	handler.ServeHTTP(recInvalidDate, reqInvalidDate)
	if recInvalidDate.Code != http.StatusBadRequest {
		t.Errorf("Expected status 400 for invalid start date, got %d", recInvalidDate.Code)
	}

	reqInvalidEndDate := httptest.NewRequest("GET", "/api/v1/calendar?start=2025-01-01&end=invalid", nil)
	recInvalidEndDate := httptest.NewRecorder()
	handler.ServeHTTP(recInvalidEndDate, reqInvalidEndDate)
	if recInvalidEndDate.Code != http.StatusBadRequest {
		t.Errorf("Expected status 400 for invalid end date, got %d", recInvalidEndDate.Code)
	}

	reqEndBeforeStart := httptest.NewRequest("GET", "/api/v1/calendar?start=2025-02-01&end=2025-01-01", nil)
	recEndBeforeStart := httptest.NewRecorder()
	handler.ServeHTTP(recEndBeforeStart, reqEndBeforeStart)
	if recEndBeforeStart.Code != http.StatusBadRequest {
		t.Errorf("Expected status 400 for end before start, got %d", recEndBeforeStart.Code)
	}
}

func TestServerMockEndpointsAndErrors(t *testing.T) {
	mockJSON := `{
		"status": "ok",
		"result": [
			{"id": "1", "title": "CPI YoY", "country": "US", "currency": "USD", "importance": 1, "date": "2025-01-10T13:30:00Z"},
			{"id": "2", "title": "German GDP", "country": "DE", "currency": "EUR", "importance": 0, "date": "2025-01-10T14:30:00Z"}
		]
	}`

	mockClient := &http.Client{
		Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
			return &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(bytes.NewBufferString(mockJSON)),
				Header:     make(http.Header),
			}, nil
		}),
	}

	srv := &Server{
		client:     tvcalendar.NewClient(tvcalendar.WithHTTPClient(mockClient)),
		startTime:  time.Now(),
		sseClients: make(map[chan []byte]bool),
		wsClients:  make(map[*websocket.Conn]bool),
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/api/v1/calendar", srv.handleCalendar)
	mux.HandleFunc("/api/v1/events", srv.handleCalendar)
	mux.HandleFunc("/api/v1/live", srv.handleLive)
	handler := srv.middleware(mux)

	// 1. Test /api/v1/calendar with currency, country, impact filters and pagination
	reqCal := httptest.NewRequest("GET", "/api/v1/calendar?start=2025-01-01&end=2025-01-31&currency=USD&country=US&impact=high&limit=10&offset=0", nil)
	recCal := httptest.NewRecorder()
	handler.ServeHTTP(recCal, reqCal)

	if recCal.Code != http.StatusOK {
		t.Fatalf("Expected status 200 for /api/v1/calendar, got %d", recCal.Code)
	}

	var calRes map[string]interface{}
	if err := json.NewDecoder(recCal.Body).Decode(&calRes); err != nil {
		t.Fatalf("Failed to decode calendar response: %v", err)
	}
	if calRes["count"].(float64) != 1 {
		t.Errorf("Expected count 1 for filtered USD/High event, got %v", calRes["count"])
	}

	// 2. Test /api/v1/events alias
	reqAlias := httptest.NewRequest("GET", "/api/v1/events?start=2025-01-01&end=2025-01-31", nil)
	recAlias := httptest.NewRecorder()
	handler.ServeHTTP(recAlias, reqAlias)
	if recAlias.Code != http.StatusOK {
		t.Errorf("Expected status 200 for /api/v1/events alias, got %d", recAlias.Code)
	}

	// 3. Test /api/v1/live
	reqLive := httptest.NewRequest("GET", "/api/v1/live", nil)
	recLive := httptest.NewRecorder()
	handler.ServeHTTP(recLive, reqLive)
	if recLive.Code != http.StatusOK {
		t.Errorf("Expected status 200 for /api/v1/live, got %d", recLive.Code)
	}

	// 4. Test error branches with failing mock client
	errMockClient := &http.Client{
		Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
			return nil, fmt.Errorf("simulated network error")
		}),
	}
	errSrv := &Server{
		client:    tvcalendar.NewClient(tvcalendar.WithHTTPClient(errMockClient)),
		startTime: time.Now(),
	}
	errMux := http.NewServeMux()
	errMux.HandleFunc("/api/v1/calendar", errSrv.handleCalendar)
	errMux.HandleFunc("/api/v1/live", errSrv.handleLive)
	errHandler := errSrv.middleware(errMux)

	reqCalErr := httptest.NewRequest("GET", "/api/v1/calendar?start=2025-01-01&end=2025-01-05", nil)
	recCalErr := httptest.NewRecorder()
	errHandler.ServeHTTP(recCalErr, reqCalErr)
	if recCalErr.Code != http.StatusInternalServerError {
		t.Errorf("Expected 500 error on failed calendar fetch, got %d", recCalErr.Code)
	}

	reqLiveErr := httptest.NewRequest("GET", "/api/v1/live", nil)
	recLiveErr := httptest.NewRecorder()
	errHandler.ServeHTTP(recLiveErr, reqLiveErr)
	if recLiveErr.Code != http.StatusInternalServerError {
		t.Errorf("Expected 500 error on failed live fetch, got %d", recLiveErr.Code)
	}
}

func TestServerStartAndShutdown(t *testing.T) {
	srv := NewServer(Config{
		Addr: "127.0.0.1:0",
	})

	go func() {
		_ = srv.Start()
	}()

	time.Sleep(50 * time.Millisecond)

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	if err := srv.Shutdown(ctx); err != nil {
		t.Errorf("Shutdown failed: %v", err)
	}
}

func TestServerWebSocketAndStream(t *testing.T) {
	srv := NewServer(Config{
		Addr: ":0",
	})

	ts := httptest.NewServer(srv.Handler())
	defer ts.Close()

	// 1. Test WebSocket connection
	wsURL := "ws" + strings.TrimPrefix(ts.URL, "http") + "/api/v1/ws"
	wsConn, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
	if err != nil {
		t.Fatalf("WebSocket connection failed: %v", err)
	}
	defer wsConn.Close()

	var wsMsg map[string]interface{}
	if err := wsConn.ReadJSON(&wsMsg); err != nil {
		t.Fatalf("Failed to read WebSocket welcome message: %v", err)
	}
	if wsMsg["event"] != "connected" {
		t.Errorf("Expected event 'connected', got %v", wsMsg["event"])
	}

	// 2. Test SSE Stream
	ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
	defer cancel()

	req, _ := http.NewRequestWithContext(ctx, "GET", ts.URL+"/api/v1/stream", nil)
	resp, err := http.DefaultClient.Do(req)
	if err == nil {
		resp.Body.Close()
	}

	// 3. Test Shutdown
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer shutdownCancel()
	if err := srv.Shutdown(shutdownCtx); err != nil {
		t.Errorf("Server Shutdown failed: %v", err)
	}
}
