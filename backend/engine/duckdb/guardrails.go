package duckdb

import (
	"context"
	"errors"
	"fmt"
	"regexp"
	"runtime"
	"strconv"
	"strings"
	"time"

	"muara.bi/engine/sqlident"
)

var (
	ErrForbiddenStatement  = errors.New("forbidden statement: only SELECT queries are permitted")
	ErrEmptyQuery          = errors.New("query cannot be empty")
	ErrInvalidRLSRule      = errors.New("invalid row-level security rule")
	ErrInvalidMemoryBudget = errors.New("invalid duckdb memory budget")
	ErrInvalidThreadCount  = errors.New("invalid duckdb thread count")
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

// trailingLimit matches a LIMIT clause at the very end of the statement. Only
// that clause bounds the rows the caller receives, so it is the one ApplyLimit
// has to police.
var trailingLimit = regexp.MustCompile(`(?i)\bLIMIT\s+(\d+)(\s+OFFSET\s+\d+)?\s*$`)

// Regex to find template parameters like {{param_name}} or {{ param_name }}
var templateParamPattern = regexp.MustCompile(`\{\{\s*([a-zA-Z0-9_]+)\s*\}\}`)

// DefaultMaxRows is the platform row cap applied when a caller does not ask for
// one. Every analytical statement is executed under a LIMIT: an unbounded read
// of a remote table would exhaust the session's memory budget.
const DefaultMaxRows = 10000

// DefaultMaxMemory is the per-session DuckDB memory budget used when the
// deployment does not configure one. Sessions are per query, so this is not the
// process total: DUCKDB_MAX_CONCURRENCY sessions may each hold this much.
// Spelled in binary units on purpose — DuckDB also parses decimal GB, which
// would silently mean 7% less than every label says.
const DefaultMaxMemory = "2GiB"

// maxMemoryPattern accepts a number with an explicit DuckDB unit; the value
// itself is checked separately because the pattern cannot express "not zero".
// Both size families DuckDB parses are allowed: KB/MB/GB/TB are 1000^i while
// KiB/MiB/GiB/TiB are 1024^i. A UI that labels its slider "GiB" can therefore
// send that spelling through and have the setting mean exactly what it says.
var maxMemoryPattern = regexp.MustCompile(`(?i)^(\d+(?:\.\d+)?)(B|KB|MB|GB|TB|KIB|MIB|GIB|TIB)$`)

// NormalizeMaxMemory validates a DuckDB memory budget such as "2GiB" or "512MB"
// and returns the default when empty. Units are mandatory: DuckDB itself refuses
// a bare number ("Unknown unit for memory_limit"), so accepting one here would
// only defer the failure to the first session. Spaces are rejected too, even
// though DuckDB tolerates "2 GB", because config strings like
// DUCKDB_MAX_MEMORY=2 GB are ambiguous at read time.
//
// A zero budget is rejected on shape *and* value. The pattern happily matches
// "0GiB", but a session that may allocate nothing is a misconfiguration either
// way: if DuckDB refuses it the failure lands at the first query instead of at
// startup, and if it treats 0 like the unlimited it already gives to "-1" then
// every guardrail built on the budget quietly stops applying.
func NormalizeMaxMemory(raw string) (string, error) {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return DefaultMaxMemory, nil
	}
	match := maxMemoryPattern.FindStringSubmatch(trimmed)
	if match == nil {
		return "", fmt.Errorf("%w: %q (want a number followed by B, KB, MB, GB, TB, KiB, MiB, GiB or TiB)", ErrInvalidMemoryBudget, trimmed)
	}
	value, err := strconv.ParseFloat(match[1], 64)
	if err != nil {
		return "", fmt.Errorf("%w: %q is not a number", ErrInvalidMemoryBudget, trimmed)
	}
	if value == 0 {
		return "", fmt.Errorf("%w: %q is zero; a memory budget must be greater than 0", ErrInvalidMemoryBudget, trimmed)
	}
	return trimmed, nil
}

// DefaultThreads is the DuckDB worker threads a session gets when the
// deployment does not name a number. It is well below a modern core count on
// purpose: DuckDB's own default, measured on 1.1.3, is every core of the machine
// (8 on the machine this was checked on), and the web product runs
// DUCKDB_MAX_CONCURRENCY sessions in one process — so threads is a share of the
// pod's CPU budget rather than a speed dial, and the multiplication that matters
// is threads x sessions.
const DefaultThreads = 2

// NormalizeThreads resolves a per-session thread count.
//
// Zero means "not configured" and becomes DefaultThreads. That is the only
// spelling Go gives an unset int, and DuckDB refuses it outright (measured on
// 1.1.3: SET threads=0 and SET threads=-1 both answer
// "Syntax Error: Must have at least 1 thread!", and the previous setting stays
// in effect), so the config layer has to translate it rather than pass it on.
//
// A count above the cores this process can see is clamped, not rejected.
// DuckDB applies no cap of its own — SET threads=999 was accepted on an 8-core
// machine — so an over-eager config would otherwise oversubscribe the CPU of a
// pod shared with other tenants. The clamp follows ApplyLimit: asking for more
// than exists gets what exists. It is also checkable after the fact, because
// current_setting('threads') echoes the number actually in effect.
func NormalizeThreads(n int) (int, error) {
	if n == 0 {
		n = DefaultThreads
	} else if n < 0 {
		return 0, fmt.Errorf("%w: %d is negative; a session needs at least 1 thread", ErrInvalidThreadCount, n)
	}
	if cores := runtime.NumCPU(); n > cores {
		return cores, nil
	}
	return n, nil
}

// GuardrailConfig holds execution limits
type GuardrailConfig struct {
	MaxRows          int           `json:"max_rows"`
	StatementTimeout time.Duration `json:"statement_timeout"`
	// MaxMemory is the DuckDB per-session memory budget, e.g. "2GiB" (binary) or
	// "2GB" (decimal, 7% smaller than it reads). Empty keeps DefaultMaxMemory.
	MaxMemory string `json:"max_memory"`
	// Threads is how many CPU workers one DuckDB session may use. Zero keeps
	// DefaultThreads; a negative value is rejected at startup and a count above
	// the machine's visible cores is clamped to them.
	Threads int `json:"threads"`
	// ExtensionDirectory is where DuckDB looks for already-installed extensions.
	// Empty keeps DuckDB's default (the user home directory).
	ExtensionDirectory string `json:"extension_directory"`
}

// DefaultGuardrailConfig returns standard enterprise guardrails
func DefaultGuardrailConfig() GuardrailConfig {
	return GuardrailConfig{
		MaxRows:          DefaultMaxRows,
		StatementTimeout: 30 * time.Second,
		MaxMemory:        DefaultMaxMemory,
		Threads:          DefaultThreads,
	}
}

// maxMemory resolves the budget a session should apply.
func (c GuardrailConfig) maxMemory() (string, error) {
	return NormalizeMaxMemory(c.MaxMemory)
}

// threads resolves the CPU budget a session should apply.
func (c GuardrailConfig) threads() (int, error) {
	return NormalizeThreads(c.Threads)
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

// LimitPlan is one policed statement plus the row cap it now enforces.
type LimitPlan struct {
	SQL string
	// Limit is the trailing LIMIT the statement carries after policing.
	Limit int
	// Injected is true when the platform added or widened the clause, i.e. the
	// author's own text did not already bound the row count this tightly.
	Injected bool
}

// PlanLimit is ApplyLimit plus the accounting the executor needs to report
// truncation honestly. The cap is widened by one row: that sentinel row is
// fetched and discarded, and its existence is how the engine knows the answer
// was cut short instead of guessing from a full-looking page.
func PlanLimit(query string, maxRows int) LimitPlan {
	if maxRows <= 0 {
		maxRows = DefaultMaxRows
	}

	trimmed := strings.TrimSpace(query)
	// Strip trailing semicolon if any
	trimmed = strings.TrimSuffix(trimmed, ";")

	if m := trailingLimit.FindStringSubmatch(trimmed); m != nil {
		if authorLimit, err := strconv.Atoi(m[1]); err == nil && authorLimit <= maxRows {
			return LimitPlan{SQL: trimmed, Limit: authorLimit}
		}
		prefix := trimmed[:len(trimmed)-len(m[0])]
		return LimitPlan{
			SQL:      fmt.Sprintf("%sLIMIT %d%s", prefix, maxRows, m[2]),
			Limit:    maxRows,
			Injected: true,
		}
	}

	return LimitPlan{
		SQL:      fmt.Sprintf("%s LIMIT %d", trimmed, maxRows),
		Limit:    maxRows,
		Injected: true,
	}
}

// ApplyLimit guarantees the statement returns at most maxRows rows. An author's
// own trailing LIMIT may only narrow that cap, never raise it; ignoring one that
// was already present would let `LIMIT 999999999` defeat the row guardrail. A
// LIMIT inside a subquery is left alone because it does not bound the outer row
// count, so one is still appended.
func ApplyLimit(query string, maxRows int) string {
	return PlanLimit(query, maxRows).SQL
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
