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
