package duckdb

import (
	"errors"
	"path/filepath"
	"strings"
	"testing"
)

func TestFileSourceQueryPicksTheCoreReader(t *testing.T) {
	cases := []struct {
		path   string
		reader string
	}{
		{"sales.csv", "read_csv_auto"},
		{"measurements.tsv", "read_csv_auto"},
		{"events.parquet", "read_parquet"},
		{"events.PQ", "read_parquet"},
		{"audit.json", "read_json_auto"},
		{"audit.jsonl", "read_json_auto"},
		{"/tmp/with spaces/orders.csv", "read_csv_auto"},
	}
	for _, tc := range cases {
		got, err := FileSourceQuery(tc.path)
		if err != nil {
			t.Fatalf("%s: %v", tc.path, err)
		}
		if !strings.HasPrefix(got, "SELECT * FROM "+tc.reader+"(") {
			t.Errorf("%s mapped to %q, want the %s reader", tc.path, got, tc.reader)
		}
	}
}

// An .xlsx source is the open product question, not a supported one: saying so
// here is what keeps the UI from promising a file type DuckDB cannot read.
func TestFileSourceQueryRejectsWhatDuckDBHasNoCoreReaderFor(t *testing.T) {
	for _, path := range []string{"", "   ", "notes.txt", "report.xlsx", "Makefile", "data.pqt"} {
		if _, err := FileSourceQuery(path); !errors.Is(err, ErrUnsupportedFileSource) {
			t.Errorf("%q returned %v, want ErrUnsupportedFileSource", path, err)
		}
	}
}

// The path becomes a SQL string literal, so a quote in a filename must not be
// able to end it and start a second argument.
func TestFileSourceQueryEscapesQuotesInPath(t *testing.T) {
	got, err := FileSourceQuery("/tmp/q1' or 1=1--.csv")
	if err != nil {
		t.Fatalf("path with a quote: %v", err)
	}
	if want := `'/tmp/q1'' or 1=1--.csv'`; !strings.Contains(got, want) {
		t.Errorf("quote not escaped: %q, want the literal %s", got, want)
	}
}

func TestFileSourceQueryUsesForwardSlashes(t *testing.T) {
	got, err := FileSourceQuery(filepath.Join("data", "2026", "orders.csv"))
	if err != nil {
		t.Fatalf("joined path: %v", err)
	}
	if strings.Contains(got, `\`) {
		t.Errorf("path kept a native separator, DuckDB would read it as an escape: %q", got)
	}
}
