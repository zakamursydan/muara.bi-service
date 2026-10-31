package response

import "muara.bi/engine/duckdb"

// ExecuteQueryResponse defines the standardized analytical query output
type ExecuteQueryResponse struct {
	Columns    []duckdb.ColumnMeta `json:"columns"`
	Rows       []map[string]any    `json:"rows"`
	RowCount   int64               `json:"row_count"`
	DurationMs int64               `json:"duration_ms"`

	// RowLimit, Truncated, TruncatedBy and CappedSources are copied verbatim from
	// the engine result: whether this is the whole answer is decided there, and
	// the UI only renders it.
	RowLimit      int                   `json:"row_limit"`
	Truncated     bool                  `json:"truncated"`
	TruncatedBy   duckdb.TruncateReason `json:"truncated_by,omitempty"`
	CappedSources []string              `json:"capped_sources,omitempty"`

	Cached bool   `json:"cached"`
	EnvTag string `json:"env_tag"`
}
