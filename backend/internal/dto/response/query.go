package response

import "muara.bi/core/internal/duckdb"

// ExecuteQueryResponse defines the standardized analytical query output
type ExecuteQueryResponse struct {
	Columns    []duckdb.ColumnMeta `json:"columns"`
	Rows       []map[string]any    `json:"rows"`
	RowCount   int64               `json:"row_count"`
	DurationMs int64               `json:"duration_ms"`
	Cached     bool                `json:"cached"`
	EnvTag     string              `json:"env_tag"`
}
