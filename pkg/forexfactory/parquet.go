package forexfactory

import (
	"fmt"
	"time"

	"github.com/xitongsys/parquet-go-source/local"
	"github.com/xitongsys/parquet-go/parquet"
	"github.com/xitongsys/parquet-go/writer"
)

// ParquetEvent represents the flat structural schema designed to map
// Forex Factory Event structures to Apache Parquet columnar storage.
type ParquetEvent struct {
	ID          string `parquet:"name=id, type=BYTE_ARRAY, convertedtype=UTF8, encoding=PLAIN_DICTIONARY"`
	Title       string `parquet:"name=title, type=BYTE_ARRAY, convertedtype=UTF8, encoding=PLAIN_DICTIONARY"`
	Country     string `parquet:"name=country, type=BYTE_ARRAY, convertedtype=UTF8, encoding=PLAIN_DICTIONARY"`
	Date        string `parquet:"name=date, type=BYTE_ARRAY, convertedtype=UTF8, encoding=PLAIN_DICTIONARY"`
	Impact      string `parquet:"name=impact, type=BYTE_ARRAY, convertedtype=UTF8, encoding=PLAIN_DICTIONARY"`
	Forecast    string `parquet:"name=forecast, type=BYTE_ARRAY, convertedtype=UTF8, encoding=PLAIN_DICTIONARY"`
	Previous    string `parquet:"name=previous, type=BYTE_ARRAY, convertedtype=UTF8, encoding=PLAIN_DICTIONARY"`
	Actual      string `parquet:"name=actual, type=BYTE_ARRAY, convertedtype=UTF8, encoding=PLAIN_DICTIONARY"`
	IsAllDay    bool   `parquet:"name=is_all_day, type=BOOLEAN"`
	IsTentative bool   `parquet:"name=is_tentative, type=BOOLEAN"`
}

// WriteParquet saves events to a highly-compressed, Snappy-encoded Apache Parquet file at the specified file path.
func WriteParquet(filePath string, events []Event) error {
	fw, err := local.NewLocalFileWriter(filePath)
	if err != nil {
		return fmt.Errorf("failed to create local file writer for path %q: %w", filePath, err)
	}
	defer fw.Close()

	pw, err := writer.NewParquetWriter(fw, new(ParquetEvent), 4)
	if err != nil {
		return fmt.Errorf("failed to create parquet writer: %w", err)
	}
	defer pw.WriteStop()

	// Configure Snappy compression for maximum compression and read speed
	pw.CompressionType = parquet.CompressionCodec_SNAPPY
	pw.RowGroupSize = 128 * 1024 * 1024 // 128MB
	pw.PageSize = 8 * 1024             // 8KB

	for _, e := range events {
		pe := ParquetEvent{
			ID:          e.ID,
			Title:       e.Title,
			Country:     e.Country,
			Date:        e.Date.Format(time.RFC3339),
			Impact:      string(e.Impact),
			Forecast:    e.Forecast,
			Previous:    e.Previous,
			Actual:      e.Actual,
			IsAllDay:    e.IsAllDay,
			IsTentative: e.IsTentative,
		}
		if err := pw.Write(pe); err != nil {
			return fmt.Errorf("failed to write record for event %q: %w", e.Title, err)
		}
	}

	return nil
}
