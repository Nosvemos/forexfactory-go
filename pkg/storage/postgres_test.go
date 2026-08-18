package storage

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/Nosvemos/tradingview-calendar-go/pkg/tvcalendar"
)

func TestPostgresStorageWithMock(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("Failed to open sqlmock: %v", err)
	}
	defer db.Close()

	store := &PostgresStorage{
		connStr: "mock_postgres",
		db:      db,
	}

	ctx := context.Background()
	now := time.Date(2025, 1, 15, 10, 0, 0, 0, time.UTC)

	// 1. Test SaveEvents
	events := []tvcalendar.Event{
		{
			ID:          "101",
			Title:       "US CPI",
			Currency:    "USD",
			Date:        now,
			Impact:      tvcalendar.ImpactHigh,
			Forecast:    "3.0%",
			Previous:    "2.9%",
			Actual:      "3.1%",
			IsAllDay:    false,
			IsTentative: false,
		},
		{
			ID:          "", // Testing auto-generated ID
			Title:       "EU Inflation",
			Currency:    "EUR",
			Date:        now,
			Impact:      tvcalendar.ImpactMedium,
			Forecast:    "2.0%",
			Previous:    "1.9%",
			Actual:      "2.1%",
			IsAllDay:    true,
			IsTentative: true,
		},
	}

	mock.ExpectBegin()
	prep := mock.ExpectPrepare("INSERT INTO events")
	prep.ExpectExec().
		WithArgs("101", "US CPI", "USD", now, "High", "3.0%", "2.9%", "3.1%", false, false).
		WillReturnResult(sqlmock.NewResult(1, 1))
	prep.ExpectExec().
		WithArgs(sqlmock.AnyArg(), "EU Inflation", "EUR", now, "Medium", "2.0%", "1.9%", "2.1%", true, true).
		WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectCommit()

	if err := store.SaveEvents(ctx, events); err != nil {
		t.Fatalf("SaveEvents failed: %v", err)
	}

	// 2. Test GetEvents
	rows := sqlmock.NewRows([]string{"id", "title", "currency", "date", "impact", "forecast", "previous", "actual", "all_day", "tentative"}).
		AddRow("101", "US CPI", "USD", now, "High", "3.0%", "2.9%", "3.1%", false, false)

	mock.ExpectQuery("SELECT id, title, currency, date, impact, forecast, previous, actual, all_day, tentative FROM events WHERE date >= \\$1 AND date <= \\$2 ORDER BY date ASC").
		WithArgs(now, now).
		WillReturnRows(rows)

	res, err := store.GetEvents(ctx, now, now)
	if err != nil || len(res) != 1 {
		t.Fatalf("GetEvents failed: %v, count=%d", err, len(res))
	}

	// 3. Test GetEventsByCurrency
	rowsCur := sqlmock.NewRows([]string{"id", "title", "currency", "date", "impact", "forecast", "previous", "actual", "all_day", "tentative"}).
		AddRow("101", "US CPI", "USD", now, "High", "3.0%", "2.9%", "3.1%", false, false)

	mock.ExpectQuery("SELECT id, title, currency, date, impact, forecast, previous, actual, all_day, tentative FROM events WHERE UPPER\\(currency\\) = \\$1 ORDER BY date ASC").
		WithArgs("USD").
		WillReturnRows(rowsCur)

	resCur, err := store.GetEventsByCurrency(ctx, "USD")
	if err != nil || len(resCur) != 1 {
		t.Fatalf("GetEventsByCurrency failed: %v, count=%d", err, len(resCur))
	}

	// 4. Test QueryEvents with filters
	start := now
	end := now.Add(24 * time.Hour)
	filter := QueryFilter{
		StartDate:  &start,
		EndDate:    &end,
		Currencies: []string{"USD", "EUR"},
		Impacts:    []tvcalendar.Impact{tvcalendar.ImpactHigh},
	}

	rowsQuery := sqlmock.NewRows([]string{"id", "title", "currency", "date", "impact", "forecast", "previous", "actual", "all_day", "tentative"}).
		AddRow("101", "US CPI", "USD", now, "High", "3.0%", "2.9%", "3.1%", false, false)

	mock.ExpectQuery("SELECT id, title, currency, date, impact, forecast, previous, actual, all_day, tentative FROM events WHERE 1=1 AND date >= \\$1 AND date <= \\$2 AND currency IN \\(\\$3,\\$4\\) AND impact IN \\(\\$5\\) ORDER BY date ASC").
		WithArgs(start, end, "USD", "EUR", "High").
		WillReturnRows(rowsQuery)

	resQuery, err := store.QueryEvents(ctx, filter)
	if err != nil || len(resQuery) != 1 {
		t.Fatalf("QueryEvents failed: %v, count=%d", err, len(resQuery))
	}

	// 5. Test Close
	mock.ExpectClose()
	if err := store.Close(); err != nil {
		t.Fatalf("Close failed: %v", err)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("Unfulfilled mock expectations: %v", err)
	}
}

func TestPostgresStorageErrorRollback(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("Failed to open sqlmock: %v", err)
	}
	defer db.Close()

	store := &PostgresStorage{
		connStr: "mock_postgres",
		db:      db,
	}

	ctx := context.Background()
	now := time.Now()

	events := []tvcalendar.Event{
		{
			ID:       "err_event",
			Title:    "Failing event",
			Currency: "USD",
			Date:     now,
			Impact:   tvcalendar.ImpactHigh,
		},
	}

	mock.ExpectBegin()
	prep := mock.ExpectPrepare("INSERT INTO events")
	prep.ExpectExec().
		WithArgs(sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg()).
		WillReturnError(fmt.Errorf("db constraint error"))
	mock.ExpectRollback()

	if err := store.SaveEvents(ctx, events); err == nil {
		t.Errorf("Expected SaveEvents to fail on execution error, got nil")
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("Unfulfilled mock expectations: %v", err)
	}
}
