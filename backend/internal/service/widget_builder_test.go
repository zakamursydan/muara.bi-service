package service

import (
	"strings"
	"testing"

	"muara.bi/core/internal/domain/entity"
)

func TestCompileBuilderSQL(t *testing.T) {
	state := entity.WidgetBuilderState{
		DatasourceAlias: "sales",
		Table:           "orders",
		Dimensions:      []string{"branch"},
		Metrics: []entity.BuilderMetric{
			{Column: "revenue", Aggregation: "SUM"},
		},
		Filters: []entity.BuilderFilter{
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
		state entity.WidgetBuilderState
	}{
		{
			name: "table with stacked statement",
			state: entity.WidgetBuilderState{
				Table:      "orders; DROP TABLE users",
				Dimensions: []string{"branch"},
			},
		},
		{
			name: "dimension with subquery",
			state: entity.WidgetBuilderState{
				Table:      "orders",
				Dimensions: []string{"(SELECT password FROM users)"},
			},
		},
		{
			name: "metric column with comment",
			state: entity.WidgetBuilderState{
				Table:   "orders",
				Metrics: []entity.BuilderMetric{{Column: "revenue -- ", Aggregation: "SUM"}},
			},
		},
		{
			name: "unsupported aggregation",
			state: entity.WidgetBuilderState{
				Table:   "orders",
				Metrics: []entity.BuilderMetric{{Column: "revenue", Aggregation: "SUM(revenue)) AS x --"}},
			},
		},
		{
			name: "alias with quote",
			state: entity.WidgetBuilderState{
				Table:   "orders",
				Metrics: []entity.BuilderMetric{{Column: "revenue", Aggregation: "SUM", Alias: "total' FROM users --"}},
			},
		},
		{
			name: "datasource alias with dot traversal",
			state: entity.WidgetBuilderState{
				DatasourceAlias: "sales.public",
				Table:           "orders",
				Dimensions:      []string{"branch"},
			},
		},
		{
			name: "filter column with injection",
			state: entity.WidgetBuilderState{
				Table:      "orders",
				Dimensions: []string{"branch"},
				Filters:    []entity.BuilderFilter{{Column: "branch = 'x' OR 1=1 --", Operator: "="}},
			},
		},
		{
			name: "unsupported filter operator",
			state: entity.WidgetBuilderState{
				Table:      "orders",
				Dimensions: []string{"branch"},
				Filters:    []entity.BuilderFilter{{Column: "branch", Operator: "IN (SELECT 1)"}},
			},
		},
		{
			name:  "missing table",
			state: entity.WidgetBuilderState{Dimensions: []string{"branch"}},
		},
		{
			name:  "no dimensions or metrics",
			state: entity.WidgetBuilderState{Table: "orders"},
		},
		{
			name: "raw metric alongside dimensions",
			state: entity.WidgetBuilderState{
				Table:      "orders",
				Dimensions: []string{"branch"},
				Metrics:    []entity.BuilderMetric{{Column: "revenue", Aggregation: "NONE"}},
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
	state := entity.WidgetBuilderState{
		Table:      "orders",
		Dimensions: []string{"o.branch"},
		Metrics: []entity.BuilderMetric{
			{Column: "o.revenue", Aggregation: "COUNT_DISTINCT"},
		},
		Filters: []entity.BuilderFilter{
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
