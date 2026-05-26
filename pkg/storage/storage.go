package storage

import (
	"context"

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
	
	// Close safely closes the database connection.
	Close() error
}
