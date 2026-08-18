package server

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/Nosvemos/tradingview-calendar-go/pkg/tvcalendar"
	"github.com/gorilla/websocket"
)

// Config holds configuration for the HTTP API server.
type Config struct {
	Addr        string
	RateLimit   int
	Concurrency int
}

var wsUpgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	CheckOrigin: func(r *http.Request) bool {
		return true // Allow all origins for API consumers
	},
}

// Server provides a high-performance REST API, SSE stream, and WebSocket microservice for global economic calendar data.
type Server struct {
	client     *tvcalendar.Client
	server     *http.Server
	startTime  time.Time
	reqCount   uint64
	eventCount uint64

	// SSE clients registry
	sseMu      sync.Mutex
	sseClients map[chan []byte]bool

	// WebSocket clients registry
	wsMu      sync.Mutex
	wsClients map[*websocket.Conn]bool
}

// NewServer creates a new API server instance.
func NewServer(cfg Config) *Server {
	if cfg.Addr == "" {
		cfg.Addr = ":8080"
	}

	var clientOpts []tvcalendar.Option
	if cfg.RateLimit > 0 {
		clientOpts = append(clientOpts, tvcalendar.WithRateLimit(cfg.RateLimit))
	}
	if cfg.Concurrency > 0 {
		clientOpts = append(clientOpts, tvcalendar.WithConcurrency(cfg.Concurrency))
	}

	s := &Server{
		client:     tvcalendar.NewClient(clientOpts...),
		startTime:  time.Now(),
		sseClients: make(map[chan []byte]bool),
		wsClients:  make(map[*websocket.Conn]bool),
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/health", s.handleHealth)
	mux.HandleFunc("/metrics", s.handleMetrics)
	mux.HandleFunc("/api/v1/calendar", s.handleCalendar)
	mux.HandleFunc("/api/v1/events", s.handleCalendar) // Route alias
	mux.HandleFunc("/api/v1/live", s.handleLive)
	mux.HandleFunc("/api/v1/stream", s.handleStream)
	mux.HandleFunc("/api/v1/ws", s.handleWebSocket)
	mux.HandleFunc("/api/v1/stats", s.handleStats)

	handler := s.middleware(mux)

	s.server = &http.Server{
		Addr:              cfg.Addr,
		Handler:           handler,
		ReadHeaderTimeout: 10 * time.Second,
		ReadTimeout:       30 * time.Second,
		WriteTimeout:      60 * time.Second,
		IdleTimeout:       120 * time.Second,
	}

	return s
}

// Start runs the HTTP server.
func (s *Server) Start() error {
	return s.server.ListenAndServe()
}

// Shutdown gracefully shuts down the server.
func (s *Server) Shutdown(ctx context.Context) error {
	_ = s.client.Close()

	// Close active WebSocket connections
	s.wsMu.Lock()
	for conn := range s.wsClients {
		_ = conn.Close()
	}
	s.wsClients = make(map[*websocket.Conn]bool)
	s.wsMu.Unlock()

	return s.server.Shutdown(ctx)
}

// Handler returns the http.Handler for testing purposes.
func (s *Server) Handler() http.Handler {
	return s.server.Handler
}

func (s *Server) middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddUint64(&s.reqCount, 1)

		// CORS headers
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")

		// Security headers
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("X-Frame-Options", "DENY")
		w.Header().Set("X-XSS-Protection", "1; mode=block")

		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusOK)
			return
		}

		next.ServeHTTP(w, r)
	})
}

func (s *Server) writeJSON(w http.ResponseWriter, statusCode int, data interface{}) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(statusCode)
	_ = json.NewEncoder(w).Encode(data)
}

func (s *Server) writeError(w http.ResponseWriter, statusCode int, message string) {
	s.writeJSON(w, statusCode, map[string]interface{}{
		"error":   true,
		"message": message,
	})
}

func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	s.writeJSON(w, http.StatusOK, map[string]interface{}{
		"status":  "ok",
		"uptime":  time.Since(s.startTime).String(),
		"version": "3.0.0",
		"time":    time.Now().UTC().Format(time.RFC3339),
	})
}

func (s *Server) handleMetrics(w http.ResponseWriter, r *http.Request) {
	var mem runtime.MemStats
	runtime.ReadMemStats(&mem)

	uptimeSec := time.Since(s.startTime).Seconds()
	reqs := atomic.LoadUint64(&s.reqCount)
	events := atomic.LoadUint64(&s.eventCount)

	s.sseMu.Lock()
	sseCount := len(s.sseClients)
	s.sseMu.Unlock()

	s.wsMu.Lock()
	wsCount := len(s.wsClients)
	s.wsMu.Unlock()

	w.Header().Set("Content-Type", "text/plain; version=0.0.4; charset=utf-8")
	w.WriteHeader(http.StatusOK)

	fmt.Fprintf(w, "# HELP tvcalendar_uptime_seconds Total uptime in seconds\n")
	fmt.Fprintf(w, "# TYPE tvcalendar_uptime_seconds counter\n")
	fmt.Fprintf(w, "tvcalendar_uptime_seconds %.2f\n\n", uptimeSec)

	fmt.Fprintf(w, "# HELP tvcalendar_requests_total Total number of HTTP requests received\n")
	fmt.Fprintf(w, "# TYPE tvcalendar_requests_total counter\n")
	fmt.Fprintf(w, "tvcalendar_requests_total %d\n\n", reqs)

	fmt.Fprintf(w, "# HELP tvcalendar_events_served_total Total count of events queried and served\n")
	fmt.Fprintf(w, "# TYPE tvcalendar_events_served_total counter\n")
	fmt.Fprintf(w, "tvcalendar_events_served_total %d\n\n", events)

	fmt.Fprintf(w, "# HELP tvcalendar_active_sse_clients Number of active SSE streaming connections\n")
	fmt.Fprintf(w, "# TYPE tvcalendar_active_sse_clients gauge\n")
	fmt.Fprintf(w, "tvcalendar_active_sse_clients %d\n\n", sseCount)

	fmt.Fprintf(w, "# HELP tvcalendar_active_ws_clients Number of active WebSocket streaming connections\n")
	fmt.Fprintf(w, "# TYPE tvcalendar_active_ws_clients gauge\n")
	fmt.Fprintf(w, "tvcalendar_active_ws_clients %d\n\n", wsCount)

	fmt.Fprintf(w, "# HELP go_goroutines Number of goroutines currently executing\n")
	fmt.Fprintf(w, "# TYPE go_goroutines gauge\n")
	fmt.Fprintf(w, "go_goroutines %d\n\n", runtime.NumGoroutine())

	fmt.Fprintf(w, "# HELP go_memstats_alloc_bytes Bytes allocated and still in use\n")
	fmt.Fprintf(w, "# TYPE go_memstats_alloc_bytes gauge\n")
	fmt.Fprintf(w, "go_memstats_alloc_bytes %d\n", mem.Alloc)
}

func (s *Server) handleCalendar(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()

	startStr := q.Get("start")
	endStr := q.Get("end")

	if startStr == "" || endStr == "" {
		s.writeError(w, http.StatusBadRequest, "Missing required query parameters: 'start' and 'end' (YYYY-MM-DD)")
		return
	}

	startDate, err := time.Parse("2006-01-02", startStr)
	if err != nil {
		s.writeError(w, http.StatusBadRequest, fmt.Sprintf("Invalid start date %q: must be YYYY-MM-DD", startStr))
		return
	}

	endDate, err := time.Parse("2006-01-02", endStr)
	if err != nil {
		s.writeError(w, http.StatusBadRequest, fmt.Sprintf("Invalid end date %q: must be YYYY-MM-DD", endStr))
		return
	}

	if startDate.After(endDate) {
		s.writeError(w, http.StatusBadRequest, "start date cannot be after end date")
		return
	}

	events, err := s.client.FetchRange(r.Context(), startDate, endDate)
	if err != nil {
		s.writeError(w, http.StatusInternalServerError, fmt.Sprintf("Failed to fetch events: %v", err))
		return
	}

	currenciesParam := q.Get("currency")
	var targetCurrencies map[string]bool
	if currenciesParam != "" {
		targetCurrencies = make(map[string]bool)
		for _, c := range strings.Split(currenciesParam, ",") {
			targetCurrencies[strings.ToUpper(strings.TrimSpace(c))] = true
		}
	}

	impactsParam := q.Get("impact")
	var targetImpacts map[string]bool
	if impactsParam != "" {
		targetImpacts = make(map[string]bool)
		for _, imp := range strings.Split(impactsParam, ",") {
			targetImpacts[strings.ToLower(strings.TrimSpace(imp))] = true
		}
	}

	countriesParam := q.Get("country")
	var targetCountries map[string]bool
	if countriesParam != "" {
		targetCountries = make(map[string]bool)
		for _, cnt := range strings.Split(countriesParam, ",") {
			targetCountries[strings.ToUpper(strings.TrimSpace(cnt))] = true
		}
	}

	var filtered []tvcalendar.Event
	for _, e := range events {
		if targetCurrencies != nil && !targetCurrencies[strings.ToUpper(e.Currency)] {
			continue
		}
		if targetImpacts != nil && !targetImpacts[strings.ToLower(string(e.Impact))] {
			continue
		}
		if targetCountries != nil && !targetCountries[strings.ToUpper(e.Country)] {
			continue
		}
		filtered = append(filtered, e)
	}

	totalFiltered := len(filtered)
	atomic.AddUint64(&s.eventCount, uint64(totalFiltered))

	offset := 0
	if offsetStr := q.Get("offset"); offsetStr != "" {
		if o, err := strconv.Atoi(offsetStr); err == nil && o >= 0 {
			offset = o
		}
	}

	limit := totalFiltered
	if limitStr := q.Get("limit"); limitStr != "" {
		if l, err := strconv.Atoi(limitStr); err == nil && l > 0 {
			limit = l
		}
	}

	var paginated []tvcalendar.Event
	if offset < totalFiltered {
		endIdx := offset + limit
		if endIdx > totalFiltered {
			endIdx = totalFiltered
		}
		paginated = filtered[offset:endIdx]
	}

	s.writeJSON(w, http.StatusOK, map[string]interface{}{
		"total":  totalFiltered,
		"count":  len(paginated),
		"offset": offset,
		"limit":  limit,
		"start":  startStr,
		"end":    endStr,
		"events": paginated,
	})
}

func (s *Server) handleLive(w http.ResponseWriter, r *http.Request) {
	events, err := s.client.FetchLiveFeed(r.Context())
	if err != nil {
		s.writeError(w, http.StatusInternalServerError, fmt.Sprintf("Failed to fetch live feed: %v", err))
		return
	}

	s.writeJSON(w, http.StatusOK, map[string]interface{}{
		"count":  len(events),
		"events": events,
	})
}

func (s *Server) handleStream(w http.ResponseWriter, r *http.Request) {
	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "Streaming unsupported", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")

	clientChan := make(chan []byte, 10)
	s.sseMu.Lock()
	s.sseClients[clientChan] = true
	s.sseMu.Unlock()

	defer func() {
		s.sseMu.Lock()
		delete(s.sseClients, clientChan)
		s.sseMu.Unlock()
		close(clientChan)
	}()

	fmt.Fprintf(w, "event: connected\ndata: {\"status\":\"connected\",\"time\":\"%s\"}\n\n", time.Now().UTC().Format(time.RFC3339))
	flusher.Flush()

	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-r.Context().Done():
			return
		case msg := <-clientChan:
			fmt.Fprintf(w, "event: update\ndata: %s\n\n", string(msg))
			flusher.Flush()
		case <-ticker.C:
			fmt.Fprintf(w, ": heartbeat\n\n")
			flusher.Flush()
		}
	}
}

func (s *Server) handleWebSocket(w http.ResponseWriter, r *http.Request) {
	conn, err := wsUpgrader.Upgrade(w, r, nil)
	if err != nil {
		return
	}

	s.wsMu.Lock()
	s.wsClients[conn] = true
	s.wsMu.Unlock()

	defer func() {
		s.wsMu.Lock()
		delete(s.wsClients, conn)
		s.wsMu.Unlock()
		_ = conn.Close()
	}()

	_ = conn.WriteJSON(map[string]interface{}{
		"event": "connected",
		"time":  time.Now().UTC().Format(time.RFC3339),
	})

	for {
		// Keep connection open and drain incoming ping/pong/messages
		_, _, err := conn.ReadMessage()
		if err != nil {
			break
		}
	}
}

func (s *Server) handleStats(w http.ResponseWriter, r *http.Request) {
	var mem runtime.MemStats
	runtime.ReadMemStats(&mem)

	s.sseMu.Lock()
	sseCount := len(s.sseClients)
	s.sseMu.Unlock()

	s.wsMu.Lock()
	wsCount := len(s.wsClients)
	s.wsMu.Unlock()

	s.writeJSON(w, http.StatusOK, map[string]interface{}{
		"uptime":             time.Since(s.startTime).String(),
		"total_requests":     atomic.LoadUint64(&s.reqCount),
		"total_events_query": atomic.LoadUint64(&s.eventCount),
		"active_sse_clients": sseCount,
		"active_ws_clients":  wsCount,
		"goroutines":         runtime.NumGoroutine(),
		"alloc_memory_mb":    fmt.Sprintf("%.2f MB", float64(mem.Alloc)/1024/1024),
	})
}
