package exporter

import (
	"encoding/json"
	"fmt"
	"io"
	"time"
)

// WriteJSON streams the sheet as a JSON array of objects, encoding one row at a
// time so large result sets never materialize a second full copy in memory.
func WriteJSON(w io.Writer, sheet Sheet) (int64, error) {
	enc := json.NewEncoder(w)
	enc.SetEscapeHTML(false)

	if _, err := io.WriteString(w, "[\n"); err != nil {
		return 0, fmt.Errorf("failed to open json array: %w", err)
	}

	var count int64
	for _, row := range sheet.Rows {
		ordered := make(map[string]any, len(sheet.Columns))
		for _, col := range sheet.Columns {
			ordered[col.Name] = normalizeJSONValue(row[col.Name])
		}

		if count > 0 {
			if _, err := io.WriteString(w, ",\n"); err != nil {
				return count, fmt.Errorf("failed to separate json row: %w", err)
			}
		}
		if err := enc.Encode(ordered); err != nil {
			return count, fmt.Errorf("failed to encode json row: %w", err)
		}
		count++
	}

	if _, err := io.WriteString(w, "]\n"); err != nil {
		return count, fmt.Errorf("failed to close json array: %w", err)
	}
	return count, nil
}

// normalizeJSONValue keeps binary and temporal cells representable in JSON.
func normalizeJSONValue(v any) any {
	switch val := v.(type) {
	case []byte:
		return string(val)
	case time.Time:
		return val.Format(time.RFC3339)
	default:
		return val
	}
}
