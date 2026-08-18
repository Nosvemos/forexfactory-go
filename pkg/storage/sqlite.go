package storage

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"fmt"
	"strings"
	"time"

	"github.com/Nosvemos/tradingview-calendar-go/pkg/tvcalendar"
	_ "modernc.org/sqlite"
)

// SQLiteStorage is an implementation of the Storage interface using pure Go SQLite.
type SQLiteStorage struct {
	dbPath string
	db     *sql.DB
}

// NewSQLiteStorage instantiates a new SQLiteStorage driver.
func NewSQLiteStorage(dbPath string) *SQLiteStorage {
	return &SQLiteStorage{dbPath: dbPath}
}

// Init opens the database and configures the table schemas, indices, and performance pragmas.
func (s *SQLiteStorage) Init(ctx context.Context) error {
	db, err := sql.Open("sqlite", s.dbPath)
	if err != nil {
		return fmt.Errorf("failed to open sqlite database %q: %w", s.dbPath, err)
	}
	s.db = db

	// Configure connection pool limits to strictly serialize writes and prevent locked states
	s.db.SetMaxOpenConns(1)
	s.db.SetMaxIdleConns(1)
	s.db.SetConnMaxLifetime(time.Hour)

	// High-performance PRAGMA configurations for SQLite
	pragmas := []string{
		"PRAGMA journal_mode=WAL;",
		"PRAGMA busy_timeout=10000;",
		"PRAGMA synchronous=NORMAL;",
		"PRAGMA cache_size=-64000;", // 64MB cache
		"PRAGMA temp_store=MEMORY;",
	}
	for _, p := range pragmas {
		if _, err := s.db.ExecContext(ctx, p); err != nil {
			return fmt.Errorf("failed to apply SQLite pragma %q: %w", p, err)
		}
	}

	schema := `
	CREATE TABLE IF NOT EXISTS events (
		id TEXT PRIMARY KEY,
		title TEXT NOT NULL,
		country TEXT,
		currency TEXT NOT NULL,
		date TEXT NOT NULL,
		impact TEXT NOT NULL,
		forecast TEXT,
		previous TEXT,
		actual TEXT,
		unit TEXT,
		category TEXT,
		indicator TEXT,
		comment TEXT,
		source TEXT,
		source_url TEXT,
		ticker TEXT,
		all_day INTEGER DEFAULT 0,
		tentative INTEGER DEFAULT 0
	);
	CREATE INDEX IF NOT EXISTS idx_events_date ON events(date);
	CREATE INDEX IF NOT EXISTS idx_events_currency ON events(currency);
	CREATE INDEX IF NOT EXISTS idx_events_date_impact ON events(date, impact);
	CREATE INDEX IF NOT EXISTS idx_events_currency_date ON events(currency, date);
	`

	_, err = s.db.ExecContext(ctx, schema)
	if err != nil {
		return fmt.Errorf("failed to run SQLite schema migrations: %w", err)
	}

	return nil
}

// SaveEvents bulk saves or updates calendar events inside SQLite using high-speed transactions.
func (s *SQLiteStorage) SaveEvents(ctx context.Context, events []tvcalendar.Event) error {
	if s.db == nil {
		return fmt.Errorf("database not initialized, call Init() first")
	}

	if len(events) == 0 {
		return nil
	}

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}

	stmt, err := tx.PrepareContext(ctx, `
		INSERT OR REPLACE INTO events (
			id, title, country, currency, date, impact, forecast, previous, actual, 
			unit, category, indicator, comment, source, source_url, ticker, all_day, tentative
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`)
	if err != nil {
		_ = tx.Rollback()
		return fmt.Errorf("failed to prepare database statement: %w", err)
	}
	defer stmt.Close()

	for _, e := range events {
		allDayVal := 0
		if e.IsAllDay {
			allDayVal = 1
		}
		tentativeVal := 0
		if e.IsTentative {
			tentativeVal = 1
		}

		eventID := e.ID
		if eventID == "" {
			hashInput := fmt.Sprintf("%d-%s-%s-%s-%s-%s", e.Date.Unix(), e.Currency, strings.ReplaceAll(strings.ToLower(e.Title), " ", "-"), e.Impact, e.Forecast, e.Previous)
			h := sha256.Sum256([]byte(hashInput))
			eventID = fmt.Sprintf("fallback-%x", h[:8])
		}

		_, err = stmt.ExecContext(ctx,
			eventID,
			e.Title,
			e.Country,
			e.Currency,
			e.Date.Format(time.RFC3339),
			string(e.Impact),
			e.Forecast,
			e.Previous,
			e.Actual,
			e.Unit,
			e.Category,
			e.Indicator,
			e.Comment,
			e.Source,
			e.SourceURL,
			e.Ticker,
			allDayVal,
			tentativeVal,
		)
		if err != nil {
			_ = tx.Rollback()
			return fmt.Errorf("failed to insert event %q into database: %w", e.Title, err)
		}
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("failed to commit database transaction: %w", err)
	}

	return nil
}

// Close safely shuts down the database connection pool.
func (s *SQLiteStorage) Close() error {
	if s.db != nil {
		return s.db.Close()
	}
	return nil
}

// GetEvents retrieves events falling within the specified date range (inclusive of start/end days).
func (s *SQLiteStorage) GetEvents(ctx context.Context, start, end time.Time) ([]tvcalendar.Event, error) {
	if s.db == nil {
		return nil, fmt.Errorf("database not initialized, call Init() first")
	}

	startStr := start.Format(time.RFC3339)
	endStr := end.Format(time.RFC3339)

	rows, err := s.db.QueryContext(ctx, `
		SELECT id, title, country, currency, date, impact, forecast, previous, actual,
		       unit, category, indicator, comment, source, source_url, ticker, all_day, tentative 
		FROM events 
		WHERE date >= ? AND date <= ?
		ORDER BY date ASC
	`, startStr, endStr)
	if err != nil {
		return nil, fmt.Errorf("failed to query events by range: %w", err)
	}
	defer rows.Close()

	return scanEvents(rows)
}

// GetEventsByCurrency retrieves events matching a specific currency code.
func (s *SQLiteStorage) GetEventsByCurrency(ctx context.Context, currency string) ([]tvcalendar.Event, error) {
	if s.db == nil {
		return nil, fmt.Errorf("database not initialized, call Init() first")
	}

	rows, err := s.db.QueryContext(ctx, `
		SELECT id, title, country, currency, date, impact, forecast, previous, actual,
		       unit, category, indicator, comment, source, source_url, ticker, all_day, tentative 
		FROM events 
		WHERE currency = ?
		ORDER BY date ASC
	`, strings.ToUpper(strings.TrimSpace(currency)))
	if err != nil {
		return nil, fmt.Errorf("failed to query events by currency: %w", err)
	}
	defer rows.Close()

	return scanEvents(rows)
}

// QueryEvents retrieves events matching a dynamic filter payload (date range, currencies, and impacts).
func (s *SQLiteStorage) QueryEvents(ctx context.Context, filter QueryFilter) ([]tvcalendar.Event, error) {
	if s.db == nil {
		return nil, fmt.Errorf("database not initialized, call Init() first")
	}

	var queryParts []string
	var args []interface{}

	queryParts = append(queryParts, `
		SELECT id, title, country, currency, date, impact, forecast, previous, actual,
		       unit, category, indicator, comment, source, source_url, ticker, all_day, tentative 
		FROM events 
		WHERE 1=1
	`)

	if filter.StartDate != nil {
		queryParts = append(queryParts, "AND date >= ?")
		args = append(args, filter.StartDate.Format(time.RFC3339))
	}
	if filter.EndDate != nil {
		queryParts = append(queryParts, "AND date <= ?")
		args = append(args, filter.EndDate.Format(time.RFC3339))
	}

	if len(filter.Currencies) > 0 {
		var placeholders []string
		for _, c := range filter.Currencies {
			placeholders = append(placeholders, "?")
			args = append(args, strings.ToUpper(strings.TrimSpace(c)))
		}
		queryParts = append(queryParts, fmt.Sprintf("AND currency IN (%s)", strings.Join(placeholders, ",")))
	}

	if len(filter.Impacts) > 0 {
		var placeholders []string
		for _, imp := range filter.Impacts {
			placeholders = append(placeholders, "?")
			args = append(args, string(imp))
		}
		queryParts = append(queryParts, fmt.Sprintf("AND impact IN (%s)", strings.Join(placeholders, ",")))
	}

	queryParts = append(queryParts, "ORDER BY date ASC")
	queryStr := strings.Join(queryParts, " ")

	rows, err := s.db.QueryContext(ctx, queryStr, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to query events dynamically: %w", err)
	}
	defer rows.Close()

	return scanEvents(rows)
}

func scanEvents(rows *sql.Rows) ([]tvcalendar.Event, error) {
	var events []tvcalendar.Event

	for rows.Next() {
		var e tvcalendar.Event
		var dateStr string
		var impactStr string
		var allDayVal, tentativeVal int

		err := rows.Scan(
			&e.ID,
			&e.Title,
			&e.Country,
			&e.Currency,
			&dateStr,
			&impactStr,
			&e.Forecast,
			&e.Previous,
			&e.Actual,
			&e.Unit,
			&e.Category,
			&e.Indicator,
			&e.Comment,
			&e.Source,
			&e.SourceURL,
			&e.Ticker,
			&allDayVal,
			&tentativeVal,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan event row: %w", err)
		}

		parsedTime, err := time.Parse(time.RFC3339, dateStr)
		if err != nil {
			parsedTime, _ = time.Parse("2006-01-02 15:04:05", dateStr)
		}
		e.Date = parsedTime
		e.Impact = tvcalendar.Impact(impactStr)
		e.IsAllDay = allDayVal == 1
		e.IsTentative = tentativeVal == 1

		events = append(events, e)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error during rows iteration: %w", err)
	}

	return events, nil
}
