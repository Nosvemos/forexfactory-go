package storage

import (
	"context"
	"time"

	"github.com/Nosvemos/forexcalendar-go/pkg/forexcalendar"
)

// QueryFilter represents a set of dynamic criteria to filter events during a database query.
type QueryFilter struct {
	StartDate  *time.Time
	EndDate    *time.Time
	Currencies []string
	Impacts    []forexcalendar.Impact
}

// Storage defines the common interface for database persistence.
// This decouples the CLI loading implementation from the SDK library layer,
// allowing external developers to easily implement custom database backends (e.g. Postgres, InfluxDB).
type Storage interface {
	// Init initializes the database connection, table schemas, or migrations.
	Init(ctx context.Context) error

	// SaveEvents bulk saves or updates the list of scraped economic events.
	SaveEvents(ctx context.Context, events []forexcalendar.Event) error

	// GetEvents retrieves events falling within the specified date range.
	GetEvents(ctx context.Context, start, end time.Time) ([]forexcalendar.Event, error)

	// GetEventsByCurrency retrieves events matching a specific currency code.
	GetEventsByCurrency(ctx context.Context, currency string) ([]forexcalendar.Event, error)

	// QueryEvents retrieves events matching a set of dynamic filter criteria.
	QueryEvents(ctx context.Context, filter QueryFilter) ([]forexcalendar.Event, error)

	// Close safely closes the database connection.
	Close() error
}
