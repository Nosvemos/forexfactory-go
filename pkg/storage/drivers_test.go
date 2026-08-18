package storage

import (
	"context"
	"testing"
	"time"

	"github.com/Nosvemos/tradingview-calendar-go/pkg/tvcalendar"
)

func TestDriversImplementsStorageInterface(t *testing.T) {
	// Verify PostgreSQL driver compiles and implements Storage interface
	var pgVar interface{} = NewPostgresStorage("postgres://user:pass@localhost:5432/dbname?sslmode=disable")
	if _, ok := pgVar.(Storage); !ok {
		t.Errorf("PostgresStorage does not implement Storage interface")
	}

	// Verify ClickHouse driver compiles and implements Storage interface
	var chVar interface{} = NewClickHouseStorage("localhost:9000", "default", "default", "")
	if _, ok := chVar.(Storage); !ok {
		t.Errorf("ClickHouseStorage does not implement Storage interface")
	}

	// Verify InfluxDB driver compiles and implements Storage interface
	var infVar interface{} = NewInfluxDBStorage("http://localhost:8086", "token", "org", "bucket")
	if _, ok := infVar.(Storage); !ok {
		t.Errorf("InfluxDBStorage does not implement Storage interface")
	}
}

func TestDriversUninitializedErrors(t *testing.T) {
	ctx := context.Background()
	now := time.Now()

	// 1. PostgreSQL Uninitialized
	pgStore := NewPostgresStorage("")
	if err := pgStore.SaveEvents(ctx, []tvcalendar.Event{}); err == nil {
		t.Errorf("Expected error when saving on uninitialized Postgres, got nil")
	}
	if _, err := pgStore.GetEvents(ctx, now, now); err == nil {
		t.Errorf("Expected error on uninitialized Postgres GetEvents, got nil")
	}
	if _, err := pgStore.GetEventsByCurrency(ctx, "USD"); err == nil {
		t.Errorf("Expected error on uninitialized Postgres GetEventsByCurrency, got nil")
	}
	if _, err := pgStore.QueryEvents(ctx, QueryFilter{}); err == nil {
		t.Errorf("Expected error on uninitialized Postgres QueryEvents, got nil")
	}

	// 2. ClickHouse Uninitialized
	chStore := NewClickHouseStorage("", "", "", "")
	if err := chStore.SaveEvents(ctx, []tvcalendar.Event{}); err == nil {
		t.Errorf("Expected error when saving on uninitialized ClickHouse, got nil")
	}
	if _, err := chStore.GetEvents(ctx, now, now); err == nil {
		t.Errorf("Expected error on uninitialized ClickHouse GetEvents, got nil")
	}
	if _, err := chStore.GetEventsByCurrency(ctx, "USD"); err == nil {
		t.Errorf("Expected error on uninitialized ClickHouse GetEventsByCurrency, got nil")
	}
	if _, err := chStore.QueryEvents(ctx, QueryFilter{}); err == nil {
		t.Errorf("Expected error on uninitialized ClickHouse QueryEvents, got nil")
	}

	// 3. InfluxDB Uninitialized
	infStore := NewInfluxDBStorage("", "", "", "")
	if err := infStore.SaveEvents(ctx, []tvcalendar.Event{}); err == nil {
		t.Errorf("Expected error when saving on uninitialized InfluxDB, got nil")
	}
	if _, err := infStore.GetEvents(ctx, now, now); err == nil {
		t.Errorf("Expected error on uninitialized InfluxDB GetEvents, got nil")
	}
	if _, err := infStore.GetEventsByCurrency(ctx, "USD"); err == nil {
		t.Errorf("Expected error on uninitialized InfluxDB GetEventsByCurrency, got nil")
	}
	if _, err := infStore.QueryEvents(ctx, QueryFilter{}); err == nil {
		t.Errorf("Expected error on uninitialized InfluxDB QueryEvents, got nil")
	}
}

func TestDriversOfflineGracefulError(t *testing.T) {
	ctx := context.Background()

	// 1. PostgreSQL offline initialization check
	pgStore := NewPostgresStorage("postgres://invalid_user:invalid_pass@127.0.0.1:9999/invalid_db?sslmode=disable")
	err := pgStore.Init(ctx)
	if err == nil {
		t.Errorf("Expected PostgresStorage Init to fail with offline credentials, got nil")
	}
	_ = pgStore.Close()

	// 2. ClickHouse offline connection check
	chStore := NewClickHouseStorage("127.0.0.1:9999", "default", "default", "")
	err = chStore.Init(ctx)
	if err == nil {
		t.Errorf("Expected ClickHouseStorage Init to fail with offline port, got nil")
	}
	_ = chStore.Close()

	// 3. InfluxDB offline connection check
	infStore := NewInfluxDBStorage("http://127.0.0.1:9999", "token", "org", "bucket")
	err = infStore.Init(ctx)
	if err == nil {
		t.Errorf("Expected InfluxDBStorage Init to fail with offline port, got nil")
	}
	_ = infStore.Close()
}
