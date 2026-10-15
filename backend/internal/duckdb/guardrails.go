package duckdb

import (
	"context"
	"errors"
	"fmt"
	"regexp"
	"strings"
	"time"
)

var (
	ErrForbiddenStatement = errors.New("forbidden statement: only SELECT queries are permitted")
	ErrEmptyQuery         = errors.New("query cannot be empty")
)

// Regex patterns to detect forbidden DDL/DML keywords
var forbiddenPatterns = []*regexp.Regexp{
	regexp.MustCompile(`(?i)\b(DROP|DELETE|UPDATE|INSERT|ALTER|TRUNCATE|GRANT|REVOKE)\b`),
	regexp.MustCompile(`(?i)\bPRAGMA\b`),
	regexp.MustCompile(`(?i)\bCREATE\s+(TABLE|USER|ROLE|SCHEMA|DATABASE)\b`),
	regexp.MustCompile(`(?i)\bEXEC(UTE)?\b`),
	regexp.MustCompile(`(?i)\bCOPY\s+.*\s+TO\b`), // prevent arbitrary file writes via DuckDB COPY TO
}

// Regex to detect if LIMIT clause is already present in outermost query
var limitPattern = regexp.MustCompile(`(?i)\bLIMIT\s+\d+`)

// Regex to find template parameters like {{param_name}} or {{ param_name }}
var templateParamPattern = regexp.MustCompile(`\{\{\s*([a-zA-Z0-9_]+)\s*\}\}`)

// GuardrailConfig holds execution limits
type GuardrailConfig struct {
	MaxRows         int           `json:"max_rows"`
	StatementTimeout time.Duration `json:"statement_timeout"`
}

// DefaultGuardrailConfig returns standard enterprise guardrails
func DefaultGuardrailConfig() GuardrailConfig {
	return GuardrailConfig{
		MaxRows:          10000,
		StatementTimeout: 30 * time.Second,
	}
}

// ValidateQuery inspects raw SQL against forbidden keywords
func ValidateQuery(rawSQL string) error {
	trimmed := strings.TrimSpace(rawSQL)
	if trimmed == "" {
		return ErrEmptyQuery
	}

	// Remove SQL comments before checking to prevent obfuscated injection
	cleaned := stripSQLComments(trimmed)

	for _, pattern := range forbiddenPatterns {
		if pattern.MatchString(cleaned) {
			return fmt.Errorf("%w (matched forbidden pattern)", ErrForbiddenStatement)
		}
	}

	return nil
}

// stripSQLComments removes both single-line -- and multi-line /* */ comments
func stripSQLComments(sql string) string {
	// Remove /* */
	multiLine := regexp.MustCompile(`/\*[\s\S]*?\*/`)
	sql = multiLine.ReplaceAllString(sql, " ")

	// Remove -- comments
	singleLine := regexp.MustCompile(`--[^\r\n]*`)
	sql = singleLine.ReplaceAllString(sql, " ")

	return strings.TrimSpace(sql)
}

// ApplyLimit ensures the query ends with a strict LIMIT if not present
func ApplyLimit(query string, maxRows int) string {
	if maxRows <= 0 {
		maxRows = 10000
	}

	trimmed := strings.TrimSpace(query)
	// Strip trailing semicolon if any
	trimmed = strings.TrimSuffix(trimmed, ";")

	if limitPattern.MatchString(trimmed) {
		return trimmed
	}

	return fmt.Sprintf("%s LIMIT %d", trimmed, maxRows)
}

// ParameterBinding transforms {{param}} into parameterized $1, $2 and extracts ordered values
type ParameterBindingResult struct {
	ParameterizedSQL string
	OrderedArgs      []any
	ParameterNames   []string
}

// TransformParameters converts mustache parameters {{var}} to positional parameters ($1, $2...)
func TransformParameters(rawSQL string, paramValues map[string]any) (*ParameterBindingResult, error) {
	var orderedArgs []any
	var paramNames []string
	paramIndex := 1

	parameterizedSQL := templateParamPattern.ReplaceAllStringFunc(rawSQL, func(match string) string {
		submatches := templateParamPattern.FindStringSubmatch(match)
		if len(submatches) < 2 {
			return match
		}
		paramName := submatches[1]
		val, exists := paramValues[paramName]
		if !exists {
			// If not provided, treat as NULL
			orderedArgs = append(orderedArgs, nil)
		} else {
			orderedArgs = append(orderedArgs, val)
		}
		paramNames = append(paramNames, paramName)

		placeholder := fmt.Sprintf("$%d", paramIndex)
		paramIndex++
		return placeholder
	})

	return &ParameterBindingResult{
		ParameterizedSQL: parameterizedSQL,
		OrderedArgs:      orderedArgs,
		ParameterNames:   paramNames,
	}, nil
}

// InjectRLS injects Row-Level Security conditions into the SQL query
// e.g. column: "branch_id", operator: "=", value: "JKT-01"
type RLSRule struct {
	Column   string `json:"column"`
	Operator string `json:"operator"` // "=", "IN", "!=", etc.
	Value    any    `json:"value"`
}

// ApplyRLS wraps or appends RLS filter condition cleanly
func ApplyRLS(sql string, rules []RLSRule, startParamIndex int) (string, []any) {
	if len(rules) == 0 {
		return sql, nil
	}

	var rlsConditions []string
	var rlsArgs []any
	paramIdx := startParamIndex

	for _, rule := range rules {
		if rule.Column == "" {
			continue
		}
		op := strings.ToUpper(strings.TrimSpace(rule.Operator))
		if op == "" {
			op = "="
		}

		condition := fmt.Sprintf("%s %s $%d", rule.Column, op, paramIdx)
		rlsConditions = append(rlsConditions, condition)
		rlsArgs = append(rlsArgs, rule.Value)
		paramIdx++
	}

	if len(rlsConditions) == 0 {
		return sql, nil
	}

	combinedRLS := strings.Join(rlsConditions, " AND ")

	// Wrap original query in a CTE or subquery to guarantee RLS scoping
	// SELECT * FROM ( <original_sql> ) AS __rls_scoped_view WHERE <rls_conditions>
	wrappedSQL := fmt.Sprintf("SELECT * FROM (\n%s\n) AS __rls_subquery WHERE %s", strings.TrimSuffix(strings.TrimSpace(sql), ";"), combinedRLS)

	return wrappedSQL, rlsArgs
}

// WithGuardrailTimeout creates a context bounded by guardrail timeout
func WithGuardrailTimeout(parent context.Context, timeout time.Duration) (context.Context, context.CancelFunc) {
	if timeout <= 0 {
		timeout = 30 * time.Second
	}
	return context.WithTimeout(parent, timeout)
}
