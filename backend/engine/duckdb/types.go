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

	// RowLimit is the row cap that was in force for this statement (always
	// reported, so a caller can tell a full answer from a capped one).
	RowLimit int `json:"row_limit"`

	// Truncated is the BRD §7.3 contract: a result that is not the complete
	// answer must say so. The engine decides it, never the UI.
	Truncated bool `json:"truncated"`

	// TruncatedBy names the mechanism; empty when Truncated is false.
	TruncatedBy TruncateReason `json:"truncated_by,omitempty"`

	// CappedSources lists the mounted views whose source read was capped, e.g.
	// a MongoDB collection that streamed the maximum document count.
	CappedSources []string `json:"capped_sources,omitempty"`
}

// TruncateReason is why a QueryResult is not the full answer.
type TruncateReason string

const (
	// TruncateRowLimit means the platform row cap cut rows the query itself
	// would have produced.
	TruncateRowLimit TruncateReason = "row_limit"

	// TruncateSourceCap means the source was read only partially before DuckDB
	// ever saw it (a MongoDB document cap, a partial file read).
	TruncateSourceCap TruncateReason = "source_cap"
)

// QueryOptions carries the row budget of one execution.
type QueryOptions struct {
	// MaxRows is the platform row cap. Zero or below uses DefaultMaxRows; the
	// engine always enforces one, so no caller can request an unbounded read.
	MaxRows int

	// CappedSources names mounted views whose source read was already truncated
	// (see StreamMongoCollections). It is reported back on the result.
	CappedSources []string
}

// rowLimit resolves the cap in force for one execution.
func (o QueryOptions) rowLimit() int {
	if o.MaxRows <= 0 {
		return DefaultMaxRows
	}
	return o.MaxRows
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

	// Query executes an analytical statement under opts.MaxRows and reports
	// honestly whether the answer was cut short.
	Query(ctx context.Context, sql string, opts QueryOptions, args ...any) (*QueryResult, error)

	// Close terminates the session and frees all allocated in-memory buffers
	Close() error
}

// Engine produces isolated Sessions
type Engine interface {
	NewSession(ctx context.Context) (Session, error)
	Close() error
}
