package exporter

import (
	"encoding/csv"
	"fmt"
	"io"
	"time"
)

// WriteCSV streams a header row followed by every data row.
func WriteCSV(w io.Writer, sheet Sheet) (int64, error) {
	writer := csv.NewWriter(w)

	header := make([]string, len(sheet.Columns))
	for i, col := range sheet.Columns {
		header[i] = col.Name
	}
	if err := writer.Write(header); err != nil {
		return 0, fmt.Errorf("failed to write csv header: %w", err)
	}

	record := make([]string, len(sheet.Columns))
	var count int64
	for _, row := range sheet.Rows {
		for i, col := range sheet.Columns {
			record[i] = FormatValue(row[col.Name])
		}
		if err := writer.Write(record); err != nil {
			return count, fmt.Errorf("failed to write csv row: %w", err)
		}
		count++
	}

	writer.Flush()
	if err := writer.Error(); err != nil {
		return count, fmt.Errorf("failed to flush csv writer: %w", err)
	}
	return count, nil
}

// FormatValue renders a single cell as its textual representation.
func FormatValue(v any) string {
	switch val := v.(type) {
	case nil:
		return ""
	case string:
		return val
	case []byte:
		return string(val)
	case time.Time:
		return val.Format(time.RFC3339)
	case bool:
		if val {
			return "true"
		}
		return "false"
	default:
		return fmt.Sprintf("%v", val)
	}
}
