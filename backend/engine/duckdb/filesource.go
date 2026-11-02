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

// ErrFileSourceGlobPattern is returned for a path containing a glob character.
// All three core readers treat their argument as a pattern, not a name: on
// DuckDB v1.1.3 a stored "x*.csv" reads every matching file (measured: two files,
// 3 rows from a source whose definition names one), and "x?.csv" does the same.
// This build has no way to turn it off — "disable_glob_matching" is rejected as
// an unknown parameter for read_csv_auto. So the character is refused rather
// than silently meaning "whatever matches", and reading a whole folder stays an
// explicit product feature with its own row budget.
var ErrFileSourceGlobPattern = fmt.Errorf("file source path contains a glob pattern")

// FileSourceQuery builds the SELECT that reads one local file, inferring both
// the reader and the column types from the file itself. The path is quoted as a
// literal and slash-normalised because DuckDB parses it as a SQL string on
// every platform, including Windows.
func FileSourceQuery(path string) (string, error) {
	trimmed := strings.TrimSpace(path)
	if trimmed == "" {
		return "", fmt.Errorf("%w: path is empty", ErrUnsupportedFileSource)
	}
	if strings.ContainsAny(trimmed, `*?[]`) {
		return "", fmt.Errorf("%w: %q — name one file, or register each file as its own source",
			ErrFileSourceGlobPattern, trimmed)
	}
	reader, ok := fileReaders[strings.ToLower(filepath.Ext(trimmed))]
	if !ok {
		return "", fmt.Errorf("%w: %s has no core DuckDB reader (use .csv, .tsv, .parquet, .json or .jsonl)",
			ErrUnsupportedFileSource, filepath.Base(trimmed))
	}
	return fmt.Sprintf("SELECT * FROM %s(%s)", reader, quoteLiteral(filepath.ToSlash(trimmed))), nil
}
