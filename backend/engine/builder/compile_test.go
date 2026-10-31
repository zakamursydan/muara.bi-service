package builder

import (
	"strings"
	"testing"
)

func TestCompileBuilderSQL(t *testing.T) {
	state := WidgetBuilderState{
		DatasourceAlias: "sales",
		Table:           "orders",
		Dimensions:      []string{"branch"},
		Metrics: []BuilderMetric{
			{Column: "revenue", Aggregation: "SUM"},
		},
		Filters: []BuilderFilter{
			{Column: "branch", Operator: "=", Value: "JKT-01"},
		},
		Limit: 500,
	}

	sql, err := CompileBuilderSQL(state)
	if err != nil {
		t.Fatalf("CompileBuilderSQL() error = %v", err)
	}

	want := "SELECT branch, SUM(revenue) AS sum_revenue\n" +
		"FROM sales.orders\n" +
		"WHERE branch = {{flt_branch}}\n" +
		"GROUP BY branch\n" +
		"LIMIT 500"
	if sql != want {
		t.Fatalf("CompileBuilderSQL()\n got: %s\nwant: %s", sql, want)
	}
	if strings.Contains(sql, "JKT-01") {
		t.Fatalf("filter value leaked into SQL text: %s", sql)
	}
}

func TestCompileBuilderSQLRejectsInjection(t *testing.T) {
	tests := []struct {
		name  string
		state WidgetBuilderState
	}{
		{
			name: "table with stacked statement",
			state: WidgetBuilderState{
				Table:      "orders; DROP TABLE users",
				Dimensions: []string{"branch"},
			},
		},
		{
			name: "dimension with subquery",
			state: WidgetBuilderState{
				Table:      "orders",
				Dimensions: []string{"(SELECT password FROM users)"},
			},
		},
		{
			name: "metric column with comment",
			state: WidgetBuilderState{
				Table:   "orders",
				Metrics: []BuilderMetric{{Column: "revenue -- ", Aggregation: "SUM"}},
			},
		},
		{
			name: "unsupported aggregation",
			state: WidgetBuilderState{
				Table:   "orders",
				Metrics: []BuilderMetric{{Column: "revenue", Aggregation: "SUM(revenue)) AS x --"}},
			},
		},
		{
			name: "alias with quote",
			state: WidgetBuilderState{
				Table:   "orders",
				Metrics: []BuilderMetric{{Column: "revenue", Aggregation: "SUM", Alias: "total' FROM users --"}},
			},
		},
		{
			name: "datasource alias with dot traversal",
			state: WidgetBuilderState{
				DatasourceAlias: "sales.public",
				Table:           "orders",
				Dimensions:      []string{"branch"},
			},
		},
		{
			name: "filter column with injection",
			state: WidgetBuilderState{
				Table:      "orders",
				Dimensions: []string{"branch"},
				Filters:    []BuilderFilter{{Column: "branch = 'x' OR 1=1 --", Operator: "="}},
			},
		},
		{
			name: "unsupported filter operator",
			state: WidgetBuilderState{
				Table:      "orders",
				Dimensions: []string{"branch"},
				Filters:    []BuilderFilter{{Column: "branch", Operator: "IN (SELECT 1)"}},
			},
		},
		{
			name:  "missing table",
			state: WidgetBuilderState{Dimensions: []string{"branch"}},
		},
		{
			name:  "no dimensions or metrics",
			state: WidgetBuilderState{Table: "orders"},
		},
		{
			name: "raw metric alongside dimensions",
			state: WidgetBuilderState{
				Table:      "orders",
				Dimensions: []string{"branch"},
				Metrics:    []BuilderMetric{{Column: "revenue", Aggregation: "NONE"}},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if sql, err := CompileBuilderSQL(tt.state); err == nil {
				t.Fatalf("expected error, got SQL: %s", sql)
			}
		})
	}
}

func TestCompileBuilderSQLQualifiedColumns(t *testing.T) {
	state := WidgetBuilderState{
		Table:      "orders",
		Dimensions: []string{"o.branch"},
		Metrics: []BuilderMetric{
			{Column: "o.revenue", Aggregation: "COUNT_DISTINCT"},
		},
		Filters: []BuilderFilter{
			{Column: "o.status", Operator: "!=", Parameter: "status"},
		},
	}

	sql, err := CompileBuilderSQL(state)
	if err != nil {
		t.Fatalf("CompileBuilderSQL() error = %v", err)
	}
	if !strings.Contains(sql, "COUNT(DISTINCT o.revenue) AS count_distinct_revenue") {
		t.Fatalf("unexpected aggregation output: %s", sql)
	}
	if !strings.Contains(sql, "o.status != {{status}}") {
		t.Fatalf("unexpected filter output: %s", sql)
	}
	if !strings.Contains(sql, "GROUP BY o.branch") {
		t.Fatalf("expected GROUP BY for aggregated metric: %s", sql)
	}
}
