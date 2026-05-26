package storage

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"time"

	"github.com/Nosvemos/forexfactory-go/pkg/forexfactory"
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

// Init opens the database and configures the table schemas and indices.
func (s *SQLiteStorage) Init(ctx context.Context) error {
	db, err := sql.Open("sqlite", s.dbPath)
	if err != nil {
		return fmt.Errorf("failed to open sqlite database %q: %w", s.dbPath, err)
	}
	s.db = db

	schema := `
	CREATE TABLE IF NOT EXISTS events (
		id TEXT PRIMARY KEY,
		title TEXT,
		country TEXT,
		date TEXT,
		impact TEXT,
		forecast TEXT,
		previous TEXT,
		actual TEXT,
		all_day INTEGER,
		tentative INTEGER
	);
	CREATE INDEX IF NOT EXISTS idx_events_date ON events(date);
	CREATE INDEX IF NOT EXISTS idx_events_country ON events(country);
	`

	_, err = s.db.ExecContext(ctx, schema)
	if err != nil {
		return fmt.Errorf("failed to run SQLite schema migrations: %w", err)
	}

	return nil
}

// SaveEvents bulk saves or updates calendar events inside SQLite using high-speed transactions.
func (s *SQLiteStorage) SaveEvents(ctx context.Context, events []forexfactory.Event) error {
	if s.db == nil {
		return fmt.Errorf("database not initialized, call Init() first")
	}

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}

	stmt, err := tx.PrepareContext(ctx, `
		INSERT OR REPLACE INTO events (
			id, title, country, date, impact, forecast, previous, actual, all_day, tentative
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
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
			// Generate standard unique hash based on timestamp, country and event name if detail id is missing
			eventID = fmt.Sprintf("%d-%s-%s", e.Date.Unix(), e.Country, strings.ReplaceAll(strings.ToLower(e.Title), " ", "-"))
		}

		_, err = stmt.ExecContext(ctx,
			eventID,
			e.Title,
			e.Country,
			e.Date.Format(time.RFC3339),
			string(e.Impact),
			e.Forecast,
			e.Previous,
			e.Actual,
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
func (s *SQLiteStorage) GetEvents(ctx context.Context, start, end time.Time) ([]forexfactory.Event, error) {
	if s.db == nil {
		return nil, fmt.Errorf("database not initialized, call Init() first")
	}

	// Format dates in RFC3339 for correct string comparison in SQLite
	startStr := start.Format(time.RFC3339)
	endStr := end.Format(time.RFC3339)

	rows, err := s.db.QueryContext(ctx, `
		SELECT id, title, country, date, impact, forecast, previous, actual, all_day, tentative 
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

// GetEventsByCountry retrieves events matching a specific currency/country code.
func (s *SQLiteStorage) GetEventsByCountry(ctx context.Context, country string) ([]forexfactory.Event, error) {
	if s.db == nil {
		return nil, fmt.Errorf("database not initialized, call Init() first")
	}

	rows, err := s.db.QueryContext(ctx, `
		SELECT id, title, country, date, impact, forecast, previous, actual, all_day, tentative 
		FROM events 
		WHERE country = ?
		ORDER BY date ASC
	`, strings.ToUpper(strings.TrimSpace(country)))
	if err != nil {
		return nil, fmt.Errorf("failed to query events by country: %w", err)
	}
	defer rows.Close()

	return scanEvents(rows)
}

// scanEvents is a helper function that reads database rows into Event structs.
func scanEvents(rows *sql.Rows) ([]forexfactory.Event, error) {
	var events []forexfactory.Event

	for rows.Next() {
		var e forexfactory.Event
		var dateStr string
		var impactStr string
		var allDayVal, tentativeVal int

		err := rows.Scan(
			&e.ID,
			&e.Title,
			&e.Country,
			&dateStr,
			&impactStr,
			&e.Forecast,
			&e.Previous,
			&e.Actual,
			&allDayVal,
			&tentativeVal,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan event row: %w", err)
		}

		// Parse date back to time.Time
		parsedTime, err := time.Parse(time.RFC3339, dateStr)
		if err != nil {
			// Fallback to simpler ISO-8601 parsing if needed
			parsedTime, _ = time.Parse("2006-01-02 15:04:05", dateStr)
		}
		e.Date = parsedTime

		// Map Impact
		e.Impact = forexfactory.Impact(impactStr)

		// Map boolean flags
		e.IsAllDay = allDayVal == 1
		e.IsTentative = tentativeVal == 1

		events = append(events, e)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error during rows iteration: %w", err)
	}

	return events, nil
}
