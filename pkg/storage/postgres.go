package storage

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"fmt"
	"strings"
	"time"

	"github.com/Nosvemos/forexfactory-go/pkg/forexfactory"
	_ "github.com/lib/pq"
)

// PostgresStorage implements the Storage SDK interface for PostgreSQL relational databases.
type PostgresStorage struct {
	connStr string
	db      *sql.DB
}

// NewPostgresStorage instantiates a new PostgresStorage driver with the given connection string.
func NewPostgresStorage(connStr string) *PostgresStorage {
	return &PostgresStorage{connStr: connStr}
}

// Init opens the database and configures the table schemas and indices.
func (p *PostgresStorage) Init(ctx context.Context) error {
	db, err := sql.Open("postgres", p.connStr)
	if err != nil {
		return fmt.Errorf("failed to open postgres database: %w", err)
	}
	p.db = db

	// Set sane connection pool limits
	p.db.SetMaxOpenConns(10)
	p.db.SetMaxIdleConns(5)
	p.db.SetConnMaxLifetime(30 * time.Minute)

	// Verify database connection is alive
	if err := p.db.PingContext(ctx); err != nil {
		return fmt.Errorf("failed to ping postgres database: %w", err)
	}

	schema := `
	CREATE TABLE IF NOT EXISTS events (
		id VARCHAR(128) PRIMARY KEY,
		title VARCHAR(256),
		currency VARCHAR(32),
		date TIMESTAMPTZ,
		impact VARCHAR(64),
		forecast VARCHAR(128),
		previous VARCHAR(128),
		actual VARCHAR(128),
		all_day BOOLEAN,
		tentative BOOLEAN
	);
	CREATE INDEX IF NOT EXISTS idx_events_date ON events(date);
	CREATE INDEX IF NOT EXISTS idx_events_currency ON events(currency);
	`

	_, err = p.db.ExecContext(ctx, schema)
	if err != nil {
		return fmt.Errorf("failed to initialize postgres tables: %w", err)
	}

	return nil
}

// SaveEvents bulk saves or updates calendar events inside PostgreSQL using a fast single transaction.
func (p *PostgresStorage) SaveEvents(ctx context.Context, events []forexfactory.Event) error {
	if p.db == nil {
		return fmt.Errorf("database not initialized, call Init() first")
	}

	tx, err := p.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}

	stmt, err := tx.PrepareContext(ctx, `
		INSERT INTO events (
			id, title, currency, date, impact, forecast, previous, actual, all_day, tentative
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
		ON CONFLICT (id) DO UPDATE SET
			title = EXCLUDED.title,
			currency = EXCLUDED.currency,
			date = EXCLUDED.date,
			impact = EXCLUDED.impact,
			forecast = EXCLUDED.forecast,
			previous = EXCLUDED.previous,
			actual = EXCLUDED.actual,
			all_day = EXCLUDED.all_day,
			tentative = EXCLUDED.tentative
	`)
	if err != nil {
		_ = tx.Rollback()
		return fmt.Errorf("failed to prepare postgres statement: %w", err)
	}
	defer stmt.Close()

	for _, e := range events {
		eventID := e.ID
		if eventID == "" {
			hashInput := fmt.Sprintf("%d-%s-%s-%s-%s-%s", e.Date.Unix(), e.Currency, strings.ReplaceAll(strings.ToLower(e.Title), " ", "-"), e.Impact, e.Forecast, e.Previous)
			h := sha256.Sum256([]byte(hashInput))
			eventID = fmt.Sprintf("fallback-%x", h[:8])
		}

		_, err = stmt.ExecContext(ctx,
			eventID,
			e.Title,
			e.Currency,
			e.Date,
			string(e.Impact),
			e.Forecast,
			e.Previous,
			e.Actual,
			e.IsAllDay,
			e.IsTentative,
		)
		if err != nil {
			_ = tx.Rollback()
			return fmt.Errorf("failed to upsert event %q into postgres: %w", e.Title, err)
		}
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("failed to commit transaction: %w", err)
	}

	return nil
}

// GetEvents retrieves events falling within the specified date range.
func (p *PostgresStorage) GetEvents(ctx context.Context, start, end time.Time) ([]forexfactory.Event, error) {
	if p.db == nil {
		return nil, fmt.Errorf("database not initialized, call Init() first")
	}

	rows, err := p.db.QueryContext(ctx, `
		SELECT id, title, currency, date, impact, forecast, previous, actual, all_day, tentative 
		FROM events 
		WHERE date >= $1 AND date <= $2
		ORDER BY date ASC
	`, start, end)
	if err != nil {
		return nil, fmt.Errorf("failed to query events by range: %w", err)
	}
	defer rows.Close()

	return p.scanEvents(rows)
}

// GetEventsByCurrency retrieves events matching a specific currency code.
func (p *PostgresStorage) GetEventsByCurrency(ctx context.Context, currency string) ([]forexfactory.Event, error) {
	if p.db == nil {
		return nil, fmt.Errorf("database not initialized, call Init() first")
	}

	rows, err := p.db.QueryContext(ctx, `
		SELECT id, title, currency, date, impact, forecast, previous, actual, all_day, tentative 
		FROM events 
		WHERE UPPER(currency) = $1
		ORDER BY date ASC
	`, strings.ToUpper(strings.TrimSpace(currency)))
	if err != nil {
		return nil, fmt.Errorf("failed to query events by currency: %w", err)
	}
	defer rows.Close()

	return p.scanEvents(rows)
}

// QueryEvents retrieves events matching a dynamic filter payload (date range, countries, and impacts).
func (p *PostgresStorage) QueryEvents(ctx context.Context, filter QueryFilter) ([]forexfactory.Event, error) {
	if p.db == nil {
		return nil, fmt.Errorf("database not initialized, call Init() first")
	}

	var queryParts []string
	var args []interface{}
	paramIndex := 1

	queryParts = append(queryParts, "SELECT id, title, currency, date, impact, forecast, previous, actual, all_day, tentative FROM events WHERE 1=1")

	if filter.StartDate != nil {
		queryParts = append(queryParts, fmt.Sprintf("AND date >= $%d", paramIndex))
		args = append(args, *filter.StartDate)
		paramIndex++
	}
	if filter.EndDate != nil {
		queryParts = append(queryParts, fmt.Sprintf("AND date <= $%d", paramIndex))
		args = append(args, *filter.EndDate)
		paramIndex++
	}

	if len(filter.Currencies) > 0 {
		var placeholders []string
		for _, c := range filter.Currencies {
			placeholders = append(placeholders, fmt.Sprintf("$%d", paramIndex))
			args = append(args, strings.ToUpper(strings.TrimSpace(c)))
			paramIndex++
		}
		queryParts = append(queryParts, fmt.Sprintf("AND currency IN (%s)", strings.Join(placeholders, ",")))
	}

	if len(filter.Impacts) > 0 {
		var placeholders []string
		for _, imp := range filter.Impacts {
			placeholders = append(placeholders, fmt.Sprintf("$%d", paramIndex))
			args = append(args, string(imp))
			paramIndex++
		}
		queryParts = append(queryParts, fmt.Sprintf("AND impact IN (%s)", strings.Join(placeholders, ",")))
	}

	queryParts = append(queryParts, "ORDER BY date ASC")
	queryStr := strings.Join(queryParts, " ")

	rows, err := p.db.QueryContext(ctx, queryStr, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to dynamically query postgres: %w", err)
	}
	defer rows.Close()

	return p.scanEvents(rows)
}

// Close safely closes the database connection.
func (p *PostgresStorage) Close() error {
	if p.db != nil {
		return p.db.Close()
	}
	return nil
}

// scanEvents is a helper function that reads postgres rows into Event structs.
func (p *PostgresStorage) scanEvents(rows *sql.Rows) ([]forexfactory.Event, error) {
	var events []forexfactory.Event

	for rows.Next() {
		var e forexfactory.Event
		var impactStr string

		err := rows.Scan(
			&e.ID,
			&e.Title,
			&e.Currency,
			&e.Date,
			&impactStr,
			&e.Forecast,
			&e.Previous,
			&e.Actual,
			&e.IsAllDay,
			&e.IsTentative,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan postgres row: %w", err)
		}

		e.Impact = forexfactory.Impact(impactStr)
		events = append(events, e)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error during postgres rows iteration: %w", err)
	}

	return events, nil
}
