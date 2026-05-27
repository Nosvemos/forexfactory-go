package storage

import (
	"context"
	"crypto/sha256"
	"fmt"
	"strings"
	"time"

	"github.com/ClickHouse/clickhouse-go/v2"
	"github.com/ClickHouse/clickhouse-go/v2/lib/driver"
	"github.com/Nosvemos/forexfactory-go/pkg/forexfactory"
)

// ClickHouseStorage implements the Storage SDK interface for ClickHouse columnar databases.
type ClickHouseStorage struct {
	addr     string
	database string
	username string
	password string
	conn     driver.Conn
}

// NewClickHouseStorage creates a new ClickHouseStorage persistence driver.
func NewClickHouseStorage(addr, database, username, password string) *ClickHouseStorage {
	if database == "" {
		database = "default"
	}
	return &ClickHouseStorage{
		addr:     addr,
		database: database,
		username: username,
		password: password,
	}
}

// Init establishes a connection to ClickHouse and constructs the ReplacingMergeTree schema.
func (c *ClickHouseStorage) Init(ctx context.Context) error {
	opt := clickhouse.Options{
		Addr: []string{c.addr},
		Auth: clickhouse.Auth{
			Database: c.database,
			Username: c.username,
			Password: c.password,
		},
		DialTimeout:     10 * time.Second,
		MaxOpenConns:    10,
		MaxIdleConns:    5,
		ConnMaxLifetime: 10 * time.Minute,
	}

	conn, err := clickhouse.Open(&opt)
	if err != nil {
		return fmt.Errorf("failed to open clickhouse connection: %w", err)
	}
	c.conn = conn

	if err := c.conn.Ping(ctx); err != nil {
		return fmt.Errorf("failed to ping clickhouse server: %w", err)
	}

	// ReplacingMergeTree engine automatically merges duplicate events based on (date, currency, id)
	schema := `
	CREATE TABLE IF NOT EXISTS events (
		id String,
		title String,
		currency String,
		date DateTime64(3, 'UTC'),
		impact LowCardinality(String),
		forecast String,
		previous String,
		actual String,
		all_day UInt8,
		tentative UInt8
	) ENGINE = ReplacingMergeTree()
	ORDER BY (date, currency, id)
	`

	if err := c.conn.Exec(ctx, schema); err != nil {
		return fmt.Errorf("failed to run clickhouse schema migration: %w", err)
	}

	return nil
}

// SaveEvents executes a high-speed columnar batch insertion into ClickHouse.
func (c *ClickHouseStorage) SaveEvents(ctx context.Context, events []forexfactory.Event) error {
	if c.conn == nil {
		return fmt.Errorf("clickhouse connection not initialized, call Init() first")
	}

	batch, err := c.conn.PrepareBatch(ctx, "INSERT INTO events (id, title, currency, date, impact, forecast, previous, actual, all_day, tentative)")
	if err != nil {
		return fmt.Errorf("failed to prepare clickhouse batch: %w", err)
	}

	for _, e := range events {
		eventID := e.ID
		if eventID == "" {
			hashInput := fmt.Sprintf("%d-%s-%s-%s-%s-%s", e.Date.Unix(), e.Currency, strings.ReplaceAll(strings.ToLower(e.Title), " ", "-"), e.Impact, e.Forecast, e.Previous)
			h := sha256.Sum256([]byte(hashInput))
			eventID = fmt.Sprintf("fallback-%x", h[:8])
		}

		allDayVal := uint8(0)
		if e.IsAllDay {
			allDayVal = 1
		}
		tentativeVal := uint8(0)
		if e.IsTentative {
			tentativeVal = 1
		}

		err = batch.Append(
			eventID,
			e.Title,
			e.Currency,
			e.Date.UTC(),
			string(e.Impact),
			e.Forecast,
			e.Previous,
			e.Actual,
			allDayVal,
			tentativeVal,
		)
		if err != nil {
			return fmt.Errorf("failed to append clickhouse batch record: %w", err)
		}
	}

	if err := batch.Send(); err != nil {
		return fmt.Errorf("failed to send clickhouse batch: %w", err)
	}

	return nil
}

// GetEvents retrieves events falling within the specified date range.
func (c *ClickHouseStorage) GetEvents(ctx context.Context, start, end time.Time) ([]forexfactory.Event, error) {
	if c.conn == nil {
		return nil, fmt.Errorf("clickhouse connection not initialized, call Init() first")
	}

	query := `
		SELECT id, title, currency, date, impact, forecast, previous, actual, all_day, tentative 
		FROM events 
		WHERE date >= ? AND date <= ?
		ORDER BY date ASC
	`

	rows, err := c.conn.Query(ctx, query, start.UTC(), end.UTC())
	if err != nil {
		return nil, fmt.Errorf("failed to query clickhouse by range: %w", err)
	}
	defer rows.Close()

	return c.scanEvents(rows)
}

// GetEventsByCurrency retrieves events matching a specific currency code.
func (c *ClickHouseStorage) GetEventsByCurrency(ctx context.Context, currency string) ([]forexfactory.Event, error) {
	if c.conn == nil {
		return nil, fmt.Errorf("clickhouse connection not initialized, call Init() first")
	}

	query := `
		SELECT id, title, currency, date, impact, forecast, previous, actual, all_day, tentative 
		FROM events 
		WHERE UPPER(currency) = ?
		ORDER BY date ASC
	`

	rows, err := c.conn.Query(ctx, query, strings.ToUpper(strings.TrimSpace(currency)))
	if err != nil {
		return nil, fmt.Errorf("failed to query clickhouse by currency: %w", err)
	}
	defer rows.Close()

	return c.scanEvents(rows)
}

// QueryEvents retrieves events matching a set of dynamic filter criteria.
func (c *ClickHouseStorage) QueryEvents(ctx context.Context, filter QueryFilter) ([]forexfactory.Event, error) {
	if c.conn == nil {
		return nil, fmt.Errorf("clickhouse connection not initialized, call Init() first")
	}

	var queryParts []string
	var args []interface{}

	queryParts = append(queryParts, "SELECT id, title, currency, date, impact, forecast, previous, actual, all_day, tentative FROM events WHERE 1=1")

	if filter.StartDate != nil {
		queryParts = append(queryParts, "AND date >= ?")
		args = append(args, filter.StartDate.UTC())
	}
	if filter.EndDate != nil {
		queryParts = append(queryParts, "AND date <= ?")
		args = append(args, filter.EndDate.UTC())
	}

	if len(filter.Currencies) > 0 {
		var placeholders []string
		for _, currency := range filter.Currencies {
			placeholders = append(placeholders, "?")
			args = append(args, strings.ToUpper(strings.TrimSpace(currency)))
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

	rows, err := c.conn.Query(ctx, queryStr, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to query clickhouse dynamically: %w", err)
	}
	defer rows.Close()

	return c.scanEvents(rows)
}

// Close safely shuts down the ClickHouse connection pool.
func (c *ClickHouseStorage) Close() error {
	if c.conn != nil {
		return c.conn.Close()
	}
	return nil
}

// scanEvents is a helper function that reads ClickHouse rows into Event structs.
func (c *ClickHouseStorage) scanEvents(rows driver.Rows) ([]forexfactory.Event, error) {
	var events []forexfactory.Event

	for rows.Next() {
		var e forexfactory.Event
		var dateVal time.Time
		var impactStr string
		var allDayVal, tentativeVal uint8

		err := rows.Scan(
			&e.ID,
			&e.Title,
			&e.Currency,
			&dateVal,
			&impactStr,
			&e.Forecast,
			&e.Previous,
			&e.Actual,
			&allDayVal,
			&tentativeVal,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan clickhouse row: %w", err)
		}

		e.Date = dateVal
		e.Impact = forexfactory.Impact(impactStr)
		e.IsAllDay = allDayVal == 1
		e.IsTentative = tentativeVal == 1

		events = append(events, e)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating clickhouse rows: %w", err)
	}

	return events, nil
}
