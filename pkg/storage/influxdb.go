package storage

import (
	"context"
	"crypto/sha256"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/Nosvemos/tradingview-calendar-go/pkg/tvcalendar"
	influxdb2 "github.com/influxdata/influxdb-client-go/v2"
	"github.com/influxdata/influxdb-client-go/v2/api"
)

// InfluxDBStorage implements the Storage SDK interface for InfluxDB time-series databases.
type InfluxDBStorage struct {
	serverURL string
	token     string
	org       string
	bucket    string
	client    influxdb2.Client
	writeAPI  api.WriteAPIBlocking
	queryAPI  api.QueryAPI
}

// NewInfluxDBStorage instantiates a new InfluxDBStorage persistence driver.
func NewInfluxDBStorage(serverURL, token, org, bucket string) *InfluxDBStorage {
	return &InfluxDBStorage{
		serverURL: serverURL,
		token:     token,
		org:       org,
		bucket:    bucket,
	}
}

// Init creates the InfluxDB client and initializes the read/write APIs.
func (i *InfluxDBStorage) Init(ctx context.Context) error {
	client := influxdb2.NewClient(i.serverURL, i.token)

	// Perform server health check
	ok, err := client.Ping(ctx)
	if err != nil || !ok {
		client.Close()
		return fmt.Errorf("failed to ping InfluxDB server: %w", err)
	}

	i.client = client
	i.writeAPI = i.client.WriteAPIBlocking(i.org, i.bucket)
	i.queryAPI = i.client.QueryAPI(i.org)

	return nil
}

// SaveEvents writes calendar events as time-series metrics points into InfluxDB.
func (i *InfluxDBStorage) SaveEvents(ctx context.Context, events []tvcalendar.Event) error {
	if i.client == nil {
		return fmt.Errorf("influxdb client not initialized, call Init() first")
	}
	if len(events) == 0 {
		return nil
	}

	for _, e := range events {
		eventID := e.ID
		if eventID == "" {
			hashInput := fmt.Sprintf("%d-%s-%s-%s-%s-%s", e.Date.Unix(), e.Currency, strings.ReplaceAll(strings.ToLower(e.Title), " ", "-"), e.Impact, e.Forecast, e.Previous)
			h := sha256.Sum256([]byte(hashInput))
			eventID = fmt.Sprintf("fallback-%x", h[:8])
		}

		// Tags: dimensions we want to group or query on
		tags := map[string]string{
			"currency":     strings.ToUpper(e.Currency),
			"impact":       string(e.Impact),
			"is_all_day":   strconv.FormatBool(e.IsAllDay),
			"is_tentative": strconv.FormatBool(e.IsTentative),
		}

		// Fields: values associated with the timestamp
		fields := map[string]interface{}{
			"id":       eventID,
			"title":    e.Title,
			"forecast": e.Forecast,
			"previous": e.Previous,
			"actual":   e.Actual,
		}

		// Points must have non-empty time tags for correct time series plotting
		eventTime := e.Date
		if eventTime.IsZero() {
			eventTime = time.Now()
		}

		p := influxdb2.NewPoint("economic_events", tags, fields, eventTime)
		if err := i.writeAPI.WritePoint(ctx, p); err != nil {
			return fmt.Errorf("failed to write influxdb point for %q: %w", e.Title, err)
		}
	}

	return nil
}

// GetEvents retrieves events falling within the specified date range.
func (i *InfluxDBStorage) GetEvents(ctx context.Context, start, end time.Time) ([]tvcalendar.Event, error) {
	fluxQuery := fmt.Sprintf(`
		from(bucket: "%s")
			|> range(start: %s, stop: %s)
			|> filter(fn: (r) => r["_measurement"] == "economic_events")
			|> pivot(rowKey:["_time", "currency", "impact", "is_all_day", "is_tentative"], columnKey: ["_field"], valueColumn: "_value")
			|> sort(columns: ["_time"])
	`, i.bucket, start.Format(time.RFC3339), end.Format(time.RFC3339))

	return i.executeFluxQuery(ctx, fluxQuery)
}

// GetEventsByCurrency retrieves events matching a specific currency code.
func (i *InfluxDBStorage) GetEventsByCurrency(ctx context.Context, currency string) ([]tvcalendar.Event, error) {
	cleanCurrency := strings.ToUpper(strings.TrimSpace(strings.ReplaceAll(currency, `"`, "")))

	// Query historical data (last 5 years to ensure full historical coverage of currency events)
	fluxQuery := fmt.Sprintf(`
		from(bucket: "%s")
			|> range(start: -5y)
			|> filter(fn: (r) => r["_measurement"] == "economic_events" and r["currency"] == "%s")
			|> pivot(rowKey:["_time", "currency", "impact", "is_all_day", "is_tentative"], columnKey: ["_field"], valueColumn: "_value")
			|> sort(columns: ["_time"])
	`, i.bucket, cleanCurrency)

	return i.executeFluxQuery(ctx, fluxQuery)
}

// buildFluxQuery constructs the Flux script for dynamic querying.
func (i *InfluxDBStorage) buildFluxQuery(filter QueryFilter) string {
	startStr := "-30d"
	stopStr := "now()"

	if filter.StartDate != nil {
		startStr = filter.StartDate.UTC().Format(time.RFC3339)
	}
	if filter.EndDate != nil {
		stopStr = filter.EndDate.UTC().Format(time.RFC3339)
	}

	filterConditions := []string{`r["_measurement"] == "economic_events"`}

	if len(filter.Currencies) > 0 {
		var sub []string
		for _, c := range filter.Currencies {
			cleanC := strings.ToUpper(strings.TrimSpace(strings.ReplaceAll(c, `"`, "")))
			sub = append(sub, fmt.Sprintf(`r["currency"] == "%s"`, cleanC))
		}
		filterConditions = append(filterConditions, fmt.Sprintf("(%s)", strings.Join(sub, " or ")))
	}

	if len(filter.Impacts) > 0 {
		var sub []string
		for _, imp := range filter.Impacts {
			cleanImp := strings.ReplaceAll(string(imp), `"`, "")
			sub = append(sub, fmt.Sprintf(`r["impact"] == "%s"`, cleanImp))
		}
		filterConditions = append(filterConditions, fmt.Sprintf("(%s)", strings.Join(sub, " or ")))
	}

	fluxQuery := fmt.Sprintf(`
		from(bucket: "%s")
			|> range(start: %s, stop: %s)
			|> filter(fn: (r) => %s)
			|> pivot(rowKey:["_time", "currency", "impact", "is_all_day", "is_tentative"], columnKey: ["_field"], valueColumn: "_value")
			|> sort(columns: ["_time"])
	`, i.bucket, startStr, stopStr, strings.Join(filterConditions, " and "))

	return fluxQuery
}

// QueryEvents retrieves events matching dynamic filter parameters.
func (i *InfluxDBStorage) QueryEvents(ctx context.Context, filter QueryFilter) ([]tvcalendar.Event, error) {
	if i.client == nil {
		return nil, fmt.Errorf("influxdb client not initialized, call Init() first")
	}

	fluxQuery := i.buildFluxQuery(filter)

	return i.executeFluxQuery(ctx, fluxQuery)
}

// Close safely closes the InfluxDB connection.
func (i *InfluxDBStorage) Close() error {
	if i.client != nil {
		i.client.Close()
	}
	return nil
}

// executeFluxQuery executes the Flux statement and parses result points back into standard Event lists.
func (i *InfluxDBStorage) executeFluxQuery(ctx context.Context, fluxQuery string) ([]tvcalendar.Event, error) {
	if i.client == nil {
		return nil, fmt.Errorf("influxdb client not initialized, call Init() first")
	}

	result, err := i.queryAPI.Query(ctx, fluxQuery)
	if err != nil {
		return nil, fmt.Errorf("influxdb query failed: %w", err)
	}
	defer result.Close()

	var events []tvcalendar.Event

	for result.Next() {
		record := result.Record()

		var e tvcalendar.Event

		// Map tags safely
		if curVal, ok := record.ValueByKey("currency").(string); ok {
			e.Currency = curVal
		}
		if impVal, ok := record.ValueByKey("impact").(string); ok {
			e.Impact = tvcalendar.Impact(impVal)
		}

		allDayStr, _ := record.ValueByKey("is_all_day").(string)
		e.IsAllDay = allDayStr == "true"

		tentativeStr, _ := record.ValueByKey("is_tentative").(string)
		e.IsTentative = tentativeStr == "true"

		// Map pivoted field values
		if idVal, ok := record.ValueByKey("id").(string); ok {
			e.ID = idVal
		}
		if titleVal, ok := record.ValueByKey("title").(string); ok {
			e.Title = titleVal
		}
		if forecastVal, ok := record.ValueByKey("forecast").(string); ok {
			e.Forecast = forecastVal
		}
		if previousVal, ok := record.ValueByKey("previous").(string); ok {
			e.Previous = previousVal
		}
		if actualVal, ok := record.ValueByKey("actual").(string); ok {
			e.Actual = actualVal
		}

		e.Date = record.Time()
		events = append(events, e)
	}

	if err := result.Err(); err != nil {
		return nil, fmt.Errorf("error during influxdb query result parsing: %w", err)
	}

	return events, nil
}
