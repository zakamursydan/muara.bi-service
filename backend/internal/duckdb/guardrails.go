package duckdb

import (
	"context"
	"errors"
	"fmt"
	"regexp"
	"strings"
	"time"

	"muara.bi/core/internal/helper/sqlident"
)

var (
	ErrForbiddenStatement = errors.New("forbidden statement: only SELECT queries are permitted")
	ErrEmptyQuery         = errors.New("query cannot be empty")
	ErrInvalidRLSRule     = errors.New("invalid row-level security rule")
)

// rlsOperators is the closed set of comparisons an RLS rule may use. Values are
// always bound as parameters, so only the operator and column reach SQL text.
var rlsOperators = map[string]struct{}{
	"=": {}, "!=": {}, ">": {}, "<": {}, ">=": {}, "<=": {},
}

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
	MaxRows          int           `json:"max_rows"`
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

// RLSRule is one resolved row-level filter, e.g. column "branch_id", operator
// "=", value "JKT-01". Operator must be one of rlsOperators.
type RLSRule struct {
	Column   string `json:"column"`
	Operator string `json:"operator"`
	Value    any    `json:"value"`
}

// ApplyRLS wraps the query so the tenant's row filters cannot be bypassed by
// anything the caller wrote:
//
//	SELECT * FROM ( <sql> ) AS __rls_subquery WHERE <rule> AND <rule>
//
// Column names are validated against a strict identifier whitelist and values
// are always bound as positional parameters starting at startParamIndex. An
// invalid rule fails closed with ErrInvalidRLSRule instead of being skipped,
// because silently dropping a filter would widen the result set.
func ApplyRLS(sql string, rules []RLSRule, startParamIndex int) (string, []any, error) {
	if len(rules) == 0 {
		return sql, nil, nil
	}
	if startParamIndex < 1 {
		startParamIndex = 1
	}

	conditions := make([]string, 0, len(rules))
	rlsArgs := make([]any, 0, len(rules))
	paramIdx := startParamIndex

	for _, rule := range rules {
		column := strings.TrimSpace(rule.Column)
		if !sqlident.IsQualified(column) {
			return "", nil, fmt.Errorf("%w: column %q is not a valid identifier", ErrInvalidRLSRule, rule.Column)
		}

		op := strings.ToUpper(strings.TrimSpace(rule.Operator))
		if op == "" {
			op = "="
		}
		if _, ok := rlsOperators[op]; !ok {
			return "", nil, fmt.Errorf("%w: operator %q is not supported", ErrInvalidRLSRule, rule.Operator)
		}

		conditions = append(conditions, fmt.Sprintf("%s %s $%d", column, op, paramIdx))
		rlsArgs = append(rlsArgs, rule.Value)
		paramIdx++
	}

	combinedRLS := strings.Join(conditions, " AND ")
	wrappedSQL := fmt.Sprintf(
		"SELECT * FROM (\n%s\n) AS __rls_subquery WHERE %s",
		strings.TrimSuffix(strings.TrimSpace(sql), ";"),
		combinedRLS,
	)

	return wrappedSQL, rlsArgs, nil
}

// WithGuardrailTimeout creates a context bounded by guardrail timeout
func WithGuardrailTimeout(parent context.Context, timeout time.Duration) (context.Context, context.CancelFunc) {
	if timeout <= 0 {
		timeout = 30 * time.Second
	}
	return context.WithTimeout(parent, timeout)
}
