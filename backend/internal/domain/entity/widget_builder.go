package entity

// WidgetBuilderState is the no-code Visual Builder payload stored on a widget
// whose query_mode is SYNCED. The backend compiles it into raw_sql so the
// stored query can never drift from the visual configuration.
type WidgetBuilderState struct {
	DatasourceAlias string          `json:"datasource_alias"`
	Table           string          `json:"table"`
	Dimensions      []string        `json:"dimensions"`
	Metrics         []BuilderMetric `json:"metrics"`
	Filters         []BuilderFilter `json:"filters"`
	Limit           int             `json:"limit"`
}

// BuilderMetric is one aggregated measure. Aggregation "NONE" selects the raw
// column, which is only valid alongside at least one dimension.
type BuilderMetric struct {
	Column      string `json:"column"`
	Aggregation string `json:"aggregation"`
	Alias       string `json:"alias"`
}

// BuilderFilter compares a column against a runtime parameter. Value is only
// used as the default supplied to the {{parameter}} placeholder at execution
// time; it is never concatenated into SQL text.
type BuilderFilter struct {
	Column    string `json:"column"`
	Operator  string `json:"operator"`
	Value     any    `json:"value"`
	Parameter string `json:"parameter"`
}
