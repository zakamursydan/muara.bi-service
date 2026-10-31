package duckdb

import (
	"context"
	"errors"
)

// ErrCGODisabled is returned when the binary was built without CGO. DuckDB is a
// C library, so an analytical session cannot exist in that build; callers must
// refuse to start rather than serve results from a stub.
var ErrCGODisabled = errors.New("duckdb engine unavailable: binary built without CGO (rebuild with CGO_ENABLED=1 and a C toolchain)")

// ColumnMeta describes a column in the query result
type ColumnMeta struct {
	Name string `json:"name"`
	Type string `json:"type"`
}

// QueryResult encapsulates tabular query output
type QueryResult struct {
	Columns    []ColumnMeta     `json:"columns"`
	Rows       []map[string]any `json:"rows"`
	RowCount   int64            `json:"row_count"`
	DurationMs int64            `json:"duration_ms"`
}

// Session represents an isolated in-memory analytical query session
type Session interface {
	// AttachPostgres mounts a remote PostgreSQL database alias in READ_ONLY mode
	AttachPostgres(ctx context.Context, alias string, dsn string) error

	// AttachMySQL mounts a remote MySQL database alias in READ_ONLY mode
	AttachMySQL(ctx context.Context, alias string, dsn string) error

	// AttachSQLite mounts a local SQLite database in READ_ONLY mode
	AttachSQLite(ctx context.Context, alias string, path string) error

	// RegisterTempView creates or replaces an in-memory temporary virtual view
	RegisterTempView(ctx context.Context, viewName string, query string) error

	// RegisterJSONData mounts an in-memory JSON array as a temporary table/view
	RegisterJSONData(ctx context.Context, viewName string, jsonData []byte) error

	// Query executes an analytical query with parameterized arguments
	Query(ctx context.Context, sql string, args ...any) (*QueryResult, error)

	// Close terminates the session and frees all allocated in-memory buffers
	Close() error
}

// Engine produces isolated Sessions
type Engine interface {
	NewSession(ctx context.Context) (Session, error)
	Close() error
}
