package duckdb

import (
	"fmt"
	"path/filepath"
	"strings"
)

// fileReaders maps a file extension to the DuckDB table function that reads it.
// Every reader here is compiled into DuckDB itself — none of them needs a
// connector from the per-OS extension bundle, which is what makes a file source
// work on an offline desktop without shipping anything extra.
var fileReaders = map[string]string{
	".csv":     "read_csv_auto",
	".tsv":     "read_csv_auto",
	".parquet": "read_parquet",
	".pq":      "read_parquet",
	".json":    "read_json_auto",
	".jsonl":   "read_json_auto",
}

// ErrUnsupportedFileSource is returned for a path DuckDB has no core reader for.
var ErrUnsupportedFileSource = fmt.Errorf("unsupported file source")

// FileSourceQuery builds the SELECT that reads one local file, inferring both
// the reader and the column types from the file itself. The path is quoted as a
// literal and slash-normalised because DuckDB parses it as a SQL string on
// every platform, including Windows.
func FileSourceQuery(path string) (string, error) {
	trimmed := strings.TrimSpace(path)
	if trimmed == "" {
		return "", fmt.Errorf("%w: path is empty", ErrUnsupportedFileSource)
	}
	reader, ok := fileReaders[strings.ToLower(filepath.Ext(trimmed))]
	if !ok {
		return "", fmt.Errorf("%w: %s has no core DuckDB reader (use .csv, .tsv, .parquet, .json or .jsonl)",
			ErrUnsupportedFileSource, filepath.Base(trimmed))
	}
	return fmt.Sprintf("SELECT * FROM %s(%s)", reader, quoteLiteral(filepath.ToSlash(trimmed))), nil
}
