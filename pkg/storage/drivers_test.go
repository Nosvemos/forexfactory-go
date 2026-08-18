package storage

import (
	"context"
	"strings"
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

func TestClickHouseQueryBuilder(t *testing.T) {
	ch := NewClickHouseStorage("localhost:9000", "default", "user", "pass")
	defer ch.Close()

	now := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)
	end := now.Add(30 * 24 * time.Hour)

	// 1. All filters
	filter := QueryFilter{
		StartDate:  &now,
		EndDate:    &end,
		Currencies: []string{"USD", "EUR"},
		Impacts:    []tvcalendar.Impact{tvcalendar.ImpactHigh, tvcalendar.ImpactMedium},
	}

	query, args := ch.buildQuery(filter)
	if !strings.Contains(query, "WHERE 1=1 AND date >= ? AND date <= ? AND currency IN (?,?) AND impact IN (?,?)") {
		t.Errorf("Unexpected ClickHouse query: %s", query)
	}
	if len(args) != 6 {
		t.Errorf("Expected 6 args, got %d", len(args))
	}

	// 2. Empty filter
	queryEmpty, argsEmpty := ch.buildQuery(QueryFilter{})
	if !strings.Contains(queryEmpty, "WHERE 1=1 ORDER BY date ASC") {
		t.Errorf("Unexpected empty ClickHouse query: %s", queryEmpty)
	}
	if len(argsEmpty) != 0 {
		t.Errorf("Expected 0 args for empty filter, got %d", len(argsEmpty))
	}
}

func TestInfluxDBFluxQueryBuilder(t *testing.T) {
	inf := NewInfluxDBStorage("http://localhost:8086", "test-token", "my-org", "macro_events")
	defer inf.Close()

	now := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)
	end := now.Add(30 * 24 * time.Hour)

	filter := QueryFilter{
		StartDate:  &now,
		EndDate:    &end,
		Currencies: []string{"USD"},
		Impacts:    []tvcalendar.Impact{tvcalendar.ImpactHigh},
	}

	flux := inf.buildFluxQuery(filter)
	if !strings.Contains(flux, `from(bucket: "macro_events")`) {
		t.Errorf("Flux query missing bucket: %s", flux)
	}
	if !strings.Contains(flux, `r["currency"] == "USD"`) {
		t.Errorf("Flux query missing currency filter: %s", flux)
	}
	if !strings.Contains(flux, `r["impact"] == "High"`) {
		t.Errorf("Flux query missing impact filter: %s", flux)
	}

	// Empty filter
	fluxEmpty := inf.buildFluxQuery(QueryFilter{})
	if !strings.Contains(fluxEmpty, "range(start: -30d, stop: now())") {
		t.Errorf("Flux empty query unexpected: %s", fluxEmpty)
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
