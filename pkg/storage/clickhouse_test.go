package storage

import (
	"fmt"
	"testing"
	"time"

	chDriver "github.com/ClickHouse/clickhouse-go/v2/lib/driver"
	"github.com/Nosvemos/tradingview-calendar-go/pkg/tvcalendar"
)

type mockCHRows struct {
	data [][]interface{}
	idx  int
}

func (m *mockCHRows) Columns() []string {
	return []string{"id", "title", "currency", "date", "impact", "forecast", "previous", "actual", "all_day", "tentative"}
}
func (m *mockCHRows) ColumnTypes() []chDriver.ColumnType { return nil }
func (m *mockCHRows) Next() bool {
	return m.idx < len(m.data)
}
func (m *mockCHRows) Scan(dest ...any) error {
	if m.idx >= len(m.data) {
		return fmt.Errorf("out of range")
	}
	row := m.data[m.idx]
	m.idx++
	for i, val := range row {
		switch d := dest[i].(type) {
		case *string:
			*d = val.(string)
		case *time.Time:
			*d = val.(time.Time)
		case *uint8:
			*d = val.(uint8)
		}
	}
	return nil
}
func (m *mockCHRows) ScanRow(dest ...any) error { return m.Scan(dest...) }
func (m *mockCHRows) Close() error              { return nil }
func (m *mockCHRows) Err() error                { return nil }
func (m *mockCHRows) HasData() bool             { return m.idx < len(m.data) }
func (m *mockCHRows) Totals(dest ...any) error  { return nil }
func (m *mockCHRows) ScanStruct(dest any) error { return nil }

func TestClickHouseScanEvents(t *testing.T) {
	ch := NewClickHouseStorage("localhost:9000", "default", "user", "pass")
	now := time.Date(2025, 1, 15, 12, 0, 0, 0, time.UTC)

	mockRows := &mockCHRows{
		data: [][]interface{}{
			{
				"1001", "FOMC Rate", "USD", now, "High", "5.5%", "5.25%", "5.5%", uint8(0), uint8(0),
			},
			{
				"1002", "Holiday", "JPY", now, "None", "", "", "", uint8(1), uint8(1),
			},
		},
	}

	events, err := ch.scanEvents(mockRows)
	if err != nil {
		t.Fatalf("scanEvents failed: %v", err)
	}
	if len(events) != 2 {
		t.Fatalf("Expected 2 events, got %d", len(events))
	}
	if events[0].Title != "FOMC Rate" || events[0].Impact != tvcalendar.ImpactHigh || events[0].IsAllDay {
		t.Errorf("Unexpected event 0: %+v", events[0])
	}
	if events[1].Title != "Holiday" || !events[1].IsAllDay || !events[1].IsTentative {
		t.Errorf("Unexpected event 1: %+v", events[1])
	}
}
