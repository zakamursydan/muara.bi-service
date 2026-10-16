package response

type AISettingsResponse struct {
	WorkspaceID        string   `json:"workspace_id"`
	Provider           string   `json:"provider"`
	HasAPIKey          bool     `json:"has_api_key"`
	BaseURL            string   `json:"base_url,omitempty"`
	DefaultModel       string   `json:"default_model"`
	FastModel          string   `json:"fast_model"`
	Temperature        float64  `json:"temperature"`
	MaxTokensBudget    int      `json:"max_tokens_budget"`
	CustomInstructions string   `json:"custom_instructions,omitempty"`
	SchemaMasking      []string `json:"schema_masking"`
}

type GeneratedSQLResponse struct {
	Prompt         string   `json:"prompt"`
	RawSQL         string   `json:"raw_sql"`
	Explanation    string   `json:"explanation,omitempty"`
	RelevantTables []string `json:"relevant_tables"`
}

type SuggestedWidgetResponse struct {
	Title         string         `json:"title"`
	ChartType     string         `json:"chart_type"` // line, bar, pie, kpi, table, etc.
	RawSQL        string         `json:"raw_sql"`
	EchartsConfig map[string]any `json:"echarts_config"`
}

type GeneratedDashboardResponse struct {
	Title       string                    `json:"title"`
	Description string                    `json:"description"`
	Widgets     []SuggestedWidgetResponse `json:"widgets"`
}
