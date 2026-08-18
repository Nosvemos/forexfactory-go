package server

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"runtime"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/Nosvemos/tradingview-calendar-go/pkg/tvcalendar"
)

// Config holds configuration for the HTTP API server.
type Config struct {
	Addr        string
	RateLimit   int
	Concurrency int
}

// Server provides a high-performance REST API and SSE streaming microservice for global economic calendar data.
type Server struct {
	client     *tvcalendar.Client
	server     *http.Server
	startTime  time.Time
	reqCount   uint64
	eventCount uint64

	// SSE clients registry
	sseMu      sync.Mutex
	sseClients map[chan []byte]bool
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
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/health", s.handleHealth)
	mux.HandleFunc("/api/v1/calendar", s.handleCalendar)
	mux.HandleFunc("/api/v1/live", s.handleLive)
	mux.HandleFunc("/api/v1/stream", s.handleStream)
	mux.HandleFunc("/api/v1/stats", s.handleStats)

	// Wrap mux with CORS and Logging
	handler := s.corsMiddleware(mux)

	s.server = &http.Server{
		Addr:         cfg.Addr,
		Handler:      handler,
		ReadTimeout:  30 * time.Second,
		WriteTimeout: 60 * time.Second,
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
	return s.server.Shutdown(ctx)
}

// Handler returns the http.Handler for testing purposes.
func (s *Server) Handler() http.Handler {
	return s.server.Handler
}

func (s *Server) corsMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddUint64(&s.reqCount, 1)

		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")

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

	// Apply currency filter if specified
	currenciesParam := q.Get("currency")
	var targetCurrencies map[string]bool
	if currenciesParam != "" {
		targetCurrencies = make(map[string]bool)
		for _, c := range strings.Split(currenciesParam, ",") {
			targetCurrencies[strings.ToUpper(strings.TrimSpace(c))] = true
		}
	}

	// Apply impact filter if specified
	impactsParam := q.Get("impact")
	var targetImpacts map[string]bool
	if impactsParam != "" {
		targetImpacts = make(map[string]bool)
		for _, imp := range strings.Split(impactsParam, ",") {
			targetImpacts[strings.ToLower(strings.TrimSpace(imp))] = true
		}
	}

	// Apply country filter if specified
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

	atomic.AddUint64(&s.eventCount, uint64(len(filtered)))

	s.writeJSON(w, http.StatusOK, map[string]interface{}{
		"count":  len(filtered),
		"start":  startStr,
		"end":    endStr,
		"events": filtered,
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

	// Send initial greeting
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
			// Heartbeat
			fmt.Fprintf(w, ": heartbeat\n\n")
			flusher.Flush()
		}
	}
}

func (s *Server) handleStats(w http.ResponseWriter, r *http.Request) {
	var mem runtime.MemStats
	runtime.ReadMemStats(&mem)

	s.writeJSON(w, http.StatusOK, map[string]interface{}{
		"uptime":             time.Since(s.startTime).String(),
		"total_requests":     atomic.LoadUint64(&s.reqCount),
		"total_events_query": atomic.LoadUint64(&s.eventCount),
		"active_sse_clients": len(s.sseClients),
		"goroutines":         runtime.NumGoroutine(),
		"alloc_memory_mb":    fmt.Sprintf("%.2f MB", float64(mem.Alloc)/1024/1024),
	})
}
