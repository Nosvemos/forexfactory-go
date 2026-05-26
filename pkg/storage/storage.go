package storage

import (
	"context"
	"time"

	"github.com/Nosvemos/forexfactory-go/pkg/forexfactory"
)

// Storage defines the common interface for database persistence.
// This decouples the CLI loading implementation from the SDK library layer,
// allowing external developers to easily implement custom database backends (e.g. Postgres, InfluxDB).
type Storage interface {
	// Init initializes the database connection, table schemas, or migrations.
	Init(ctx context.Context) error
	
	// SaveEvents bulk saves or updates the list of scraped economic events.
	SaveEvents(ctx context.Context, events []forexfactory.Event) error
	
	// GetEvents retrieves events falling within the specified date range.
	GetEvents(ctx context.Context, start, end time.Time) ([]forexfactory.Event, error)
	
	// GetEventsByCountry retrieves events matching a specific currency/country code.
	GetEventsByCountry(ctx context.Context, country string) ([]forexfactory.Event, error)
	
	// Close safely closes the database connection.
	Close() error
}
