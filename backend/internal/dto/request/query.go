package request

// ExecuteQueryRequest defines the input payload for analytical query execution
type ExecuteQueryRequest struct {
	ProjectID    string         `json:"project_id"`
	EnvTag       string         `json:"env_tag"`       // dynamic custom environment tag
	RawSQL       string         `json:"raw_sql"`       // DuckDB SQL query
	Parameters   map[string]any `json:"parameters"`    // dynamic parameters for {{var}}
	CacheTTL     int            `json:"cache_ttl"`     // TTL in seconds (default 300)
	ForceRefresh bool           `json:"force_refresh"` // bypass Redis cache
	MaxRows      int            `json:"max_rows"`      // guardrail row limit (default 10000)
}
