// Package exporter renders analytical tabular results into downloadable files.
package exporter

import (
	"fmt"
	"io"
	"os"
	"strings"

	"muara.bi/core/internal/duckdb"
)

const (
	FormatCSV  = "CSV"
	FormatJSON = "JSON"
	FormatXLSX = "XLSX"
)

// Sheet is one tabular dataset. XLSX exports map one Sheet to one worksheet;
// CSV and JSON exports only consume the first Sheet.
type Sheet struct {
	Name    string
	Columns []duckdb.ColumnMeta
	Rows    []map[string]any
}

func NewSheet(name string, result *duckdb.QueryResult) Sheet {
	return Sheet{Name: name, Columns: result.Columns, Rows: result.Rows}
}

// NormalizeFormat upper-cases and validates an export format identifier.
func NormalizeFormat(format string) (string, error) {
	f := strings.ToUpper(strings.TrimSpace(format))
	switch f {
	case FormatCSV, FormatJSON, FormatXLSX:
		return f, nil
	default:
		return "", fmt.Errorf("unsupported export format: %s", format)
	}
}

// FileExtension maps a normalized format to its lowercase file extension.
func FileExtension(format string) string {
	switch format {
	case FormatCSV:
		return "csv"
	case FormatJSON:
		return "json"
	case FormatXLSX:
		return "xlsx"
	default:
		return strings.ToLower(format)
	}
}

// WriteFile renders sheets to path using the given format. CSV and JSON use
// the first sheet only.
func WriteFile(path, format string, sheets []Sheet) (int64, error) {
	if len(sheets) == 0 {
		return 0, fmt.Errorf("no sheets to export")
	}

	f, err := os.Create(path)
	if err != nil {
		return 0, fmt.Errorf("failed to create export file: %w", err)
	}
	defer f.Close()

	var rows int64
	switch format {
	case FormatCSV:
		rows, err = WriteCSV(f, sheets[0])
	case FormatJSON:
		rows, err = WriteJSON(f, sheets[0])
	case FormatXLSX:
		rows, err = WriteXLSX(f, sheets)
	default:
		return 0, fmt.Errorf("unsupported export format: %s", format)
	}
	if err != nil {
		return 0, err
	}

	return rows, f.Close()
}

// Write renders sheets to an arbitrary writer, used for direct HTTP streaming.
func Write(w io.Writer, format string, sheets []Sheet) (int64, error) {
	if len(sheets) == 0 {
		return 0, fmt.Errorf("no sheets to export")
	}

	switch format {
	case FormatCSV:
		return WriteCSV(w, sheets[0])
	case FormatJSON:
		return WriteJSON(w, sheets[0])
	case FormatXLSX:
		return WriteXLSX(w, sheets)
	default:
		return 0, fmt.Errorf("unsupported export format: %s", format)
	}
}
