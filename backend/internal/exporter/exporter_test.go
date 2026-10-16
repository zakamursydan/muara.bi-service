package exporter

import (
	"bytes"
	"strings"
	"testing"

	"github.com/xuri/excelize/v2"
	"muara.bi/core/internal/duckdb"
)

func sampleResult() *duckdb.QueryResult {
	return &duckdb.QueryResult{
		Columns: []duckdb.ColumnMeta{
			{Name: "branch", Type: "VARCHAR"},
			{Name: "revenue", Type: "BIGINT"},
		},
		Rows: []map[string]any{
			{"branch": "SUB_01", "revenue": int64(1500)},
			{"branch": "JKT_02", "revenue": nil},
		},
		RowCount: 2,
	}
}

func TestWriteCSV(t *testing.T) {
	var buf bytes.Buffer
	rows, err := WriteCSV(&buf, NewSheet("Data", sampleResult()))
	if err != nil {
		t.Fatalf("WriteCSV error: %v", err)
	}
	if rows != 2 {
		t.Fatalf("expected 2 rows, got %d", rows)
	}

	lines := strings.Split(strings.TrimSpace(buf.String()), "\n")
	if len(lines) != 3 {
		t.Fatalf("expected header + 2 rows, got %d lines", len(lines))
	}
	if lines[0] != "branch,revenue" {
		t.Fatalf("unexpected header: %s", lines[0])
	}
	if lines[2] != "JKT_02," {
		t.Fatalf("nil cell should render empty, got %q", lines[2])
	}
}

func TestWriteJSON(t *testing.T) {
	var buf bytes.Buffer
	if _, err := WriteJSON(&buf, NewSheet("Data", sampleResult())); err != nil {
		t.Fatalf("WriteJSON error: %v", err)
	}

	out := buf.String()
	if !strings.HasPrefix(out, "[") || !strings.HasSuffix(strings.TrimSpace(out), "]") {
		t.Fatalf("output is not a json array: %s", out)
	}
	if !strings.Contains(out, `"branch":"SUB_01"`) {
		t.Fatalf("missing row payload: %s", out)
	}
}

func TestWriteXLSXMultiSheet(t *testing.T) {
	sheets := []Sheet{
		NewSheet("Revenue / Q1", sampleResult()),
		NewSheet("Revenue / Q1", sampleResult()),
		NewSheet("[]:*?/\\", sampleResult()),
	}

	var buf bytes.Buffer
	rows, err := WriteXLSX(&buf, sheets)
	if err != nil {
		t.Fatalf("WriteXLSX error: %v", err)
	}
	if rows != 6 {
		t.Fatalf("expected 6 total rows, got %d", rows)
	}

	f, err := excelize.OpenReader(&buf)
	if err != nil {
		t.Fatalf("failed to reopen workbook: %v", err)
	}
	defer f.Close()

	names := f.GetSheetList()
	if len(names) != 3 {
		t.Fatalf("expected 3 worksheets, got %v", names)
	}
	if names[0] != "Revenue  Q1" {
		t.Fatalf("illegal characters not stripped: %q", names[0])
	}
	if names[1] != "Revenue  Q1_2" {
		t.Fatalf("duplicate sheet name not disambiguated: %q", names[1])
	}
	if names[2] != "Sheet3" {
		t.Fatalf("empty sheet name not defaulted: %q", names[2])
	}

	cell, err := f.GetCellValue(names[0], "B2")
	if err != nil {
		t.Fatalf("failed to read cell: %v", err)
	}
	if cell != "1500" {
		t.Fatalf("unexpected cell value: %q", cell)
	}
}

func TestUniqueSheetNameTruncates(t *testing.T) {
	used := map[string]struct{}{}
	name := uniqueSheetName(strings.Repeat("x", 60), 0, used)
	if len(name) != maxSheetNameLen {
		t.Fatalf("expected %d chars, got %d", maxSheetNameLen, len(name))
	}
}
