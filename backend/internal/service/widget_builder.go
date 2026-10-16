package service

import (
	"fmt"
	"strings"

	"muara.bi/core/internal/domain/entity"
	"muara.bi/core/internal/helper/sqlident"
)

// builderAggregations maps a visual builder aggregation to its SQL template.
var builderAggregations = map[string]string{
	"COUNT":          "COUNT(%s)",
	"SUM":            "SUM(%s)",
	"AVG":            "AVG(%s)",
	"MIN":            "MIN(%s)",
	"MAX":            "MAX(%s)",
	"COUNT_DISTINCT": "COUNT(DISTINCT %s)",
	"NONE":           "%s",
}

// builderFilterOperators is the closed set of comparisons a no-code filter may
// express. Filter values always become {{parameter}} placeholders.
var builderFilterOperators = map[string]struct{}{
	"=": {}, "!=": {}, ">": {}, "<": {}, ">=": {}, "<=": {}, "LIKE": {}, "ILIKE": {},
}

// CompileBuilderSQL turns a SYNCED visual builder state into a DuckDB query.
// Identifiers are validated against a strict pattern and every filter value is
// emitted as a {{parameter}} placeholder, so no user supplied text is ever
// concatenated into the statement.
func CompileBuilderSQL(state entity.WidgetBuilderState) (string, error) {
	table, err := compileTableRef(state)
	if err != nil {
		return "", err
	}

	if len(state.Dimensions) == 0 && len(state.Metrics) == 0 {
		return "", fmt.Errorf("builder state needs at least one dimension or metric")
	}

	var selectParts []string
	var groupByParts []string
	needsGroupBy := false

	for _, dimension := range state.Dimensions {
		if !sqlident.IsQualified(dimension) {
			return "", fmt.Errorf("invalid dimension identifier %q", dimension)
		}
		selectParts = append(selectParts, dimension)
		groupByParts = append(groupByParts, dimension)
	}

	for _, metric := range state.Metrics {
		if !sqlident.IsQualified(metric.Column) {
			return "", fmt.Errorf("invalid metric column %q", metric.Column)
		}

		aggregation := strings.ToUpper(strings.TrimSpace(metric.Aggregation))
		if aggregation == "" {
			aggregation = "NONE"
		}
		template, ok := builderAggregations[aggregation]
		if !ok {
			return "", fmt.Errorf("unsupported aggregation %q", metric.Aggregation)
		}
		if aggregation == "NONE" && len(state.Dimensions) > 0 {
			return "", fmt.Errorf("metric %q must use an aggregation when dimensions are present", metric.Column)
		}
		if aggregation != "NONE" {
			needsGroupBy = true
		}

		expression := fmt.Sprintf(template, metric.Column)
		alias := strings.TrimSpace(metric.Alias)
		if alias == "" {
			alias = defaultMetricAlias(aggregation, metric.Column)
		}
		if !sqlident.IsPlain(alias) {
			return "", fmt.Errorf("invalid metric alias %q", alias)
		}
		selectParts = append(selectParts, fmt.Sprintf("%s AS %s", expression, alias))
	}

	whereParts, err := compileFilters(state.Filters)
	if err != nil {
		return "", err
	}

	var sb strings.Builder
	sb.WriteString("SELECT ")
	sb.WriteString(strings.Join(selectParts, ", "))
	sb.WriteString("\nFROM ")
	sb.WriteString(table)

	if len(whereParts) > 0 {
		sb.WriteString("\nWHERE ")
		sb.WriteString(strings.Join(whereParts, " AND "))
	}
	if needsGroupBy && len(groupByParts) > 0 {
		sb.WriteString("\nGROUP BY ")
		sb.WriteString(strings.Join(groupByParts, ", "))
	}
	if state.Limit > 0 {
		fmt.Fprintf(&sb, "\nLIMIT %d", state.Limit)
	}

	return sb.String(), nil
}

func compileTableRef(state entity.WidgetBuilderState) (string, error) {
	table := strings.TrimSpace(state.Table)
	if table == "" {
		return "", fmt.Errorf("builder state requires a source table")
	}
	if !sqlident.IsPlain(table) {
		return "", fmt.Errorf("invalid table identifier %q", table)
	}

	alias := strings.TrimSpace(state.DatasourceAlias)
	if alias == "" {
		return table, nil
	}
	if !sqlident.IsPlain(alias) {
		return "", fmt.Errorf("invalid datasource alias %q", alias)
	}
	return alias + "." + table, nil
}

func compileFilters(filters []entity.BuilderFilter) ([]string, error) {
	parts := make([]string, 0, len(filters))

	for _, filter := range filters {
		if !sqlident.IsQualified(filter.Column) {
			return nil, fmt.Errorf("invalid filter column %q", filter.Column)
		}

		operator := strings.ToUpper(strings.TrimSpace(filter.Operator))
		if operator == "" {
			operator = "="
		}
		if _, ok := builderFilterOperators[operator]; !ok {
			return nil, fmt.Errorf("unsupported filter operator %q", filter.Operator)
		}

		parameter := strings.TrimSpace(filter.Parameter)
		if parameter == "" {
			parameter = "flt_" + strings.ReplaceAll(filter.Column, ".", "_")
		}
		if !sqlident.IsPlain(parameter) {
			return nil, fmt.Errorf("invalid filter parameter %q", parameter)
		}

		parts = append(parts, fmt.Sprintf("%s %s {{%s}}", filter.Column, operator, parameter))
	}

	return parts, nil
}

func defaultMetricAlias(aggregation, column string) string {
	leaf := column
	if idx := strings.LastIndex(column, "."); idx >= 0 {
		leaf = column[idx+1:]
	}
	if aggregation == "NONE" {
		return leaf
	}
	return strings.ToLower(aggregation) + "_" + leaf
}
