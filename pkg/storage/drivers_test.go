package storage

import (
	"context"
	"testing"
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
