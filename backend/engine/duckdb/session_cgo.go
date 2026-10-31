//go:build cgo

package duckdb

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	goduckdb "github.com/marcboeker/go-duckdb"
)

// quoteIdent wraps an object name in double quotes so DuckDB reads it as one
// identifier instead of re-parsing characters like "-".
func quoteIdent(s string) string {
	return `"` + strings.ReplaceAll(s, `"`, `""`) + `"`
}

// quoteLiteral turns driver text into a DuckDB string literal. Credentials are
// user supplied, so an embedded quote must not be able to close the ATTACH text.
func quoteLiteral(s string) string {
	return "'" + strings.ReplaceAll(s, "'", "''") + "'"
}

type cgoEngine struct {
	maxMemory          string
	threads            int
	statementTimeout   time.Duration
	extensionDirectory string
}

func NewEngine(cfg GuardrailConfig) (Engine, error) {
	maxMemory, err := cfg.maxMemory()
	if err != nil {
		return nil, err
	}
	return &cgoEngine{
		maxMemory: maxMemory,
		threads:   2,
		// threads is deliberately not configurable yet: the CPU budget per
		// session only makes sense once desktop M0 fixes the machine profile.
		statementTimeout:   cfg.StatementTimeout,
		extensionDirectory: cfg.ExtensionDirectory,
	}, nil
}

func (e *cgoEngine) Close() error {
	return nil
}

type cgoSession struct {
	db               *sql.DB
	tempFiles        []string
	statementTimeout time.Duration
}

func (e *cgoEngine) NewSession(ctx context.Context) (Session, error) {
	db, err := sql.Open("duckdb", "")
	if err != nil {
		return nil, fmt.Errorf("failed to initialize duckdb in-memory session: %w", err)
	}

	// Apply strict safety bounds. There is no DuckDB `statement_timeout` setting,
	// so the per-query deadline is enforced with a context in Query.
	initStatements := []string{
		"SET max_memory=" + quoteLiteral(e.maxMemory),
		fmt.Sprintf("SET threads=%d", e.threads),
	}
	if e.extensionDirectory != "" {
		initStatements = append(initStatements,
			"SET extension_directory="+quoteLiteral(e.extensionDirectory))
	}

	for _, stmt := range initStatements {
		if _, err := db.ExecContext(ctx, stmt); err != nil {
			_ = db.Close()
			return nil, fmt.Errorf("failed to apply duckdb guardrail statement %q: %w", stmt, err)
		}
	}

	return &cgoSession{
		db:               db,
		statementTimeout: e.statementTimeout,
	}, nil
}

// loadExtension makes a connector available for ATTACH. The production image
// ships pre-bundled extensions, so INSTALL is best effort, but a failed LOAD is
// reported: swallowing it used to surface as a confusing "Catalog Error" later.
func (s *cgoSession) loadExtension(ctx context.Context, name string) error {
	_, _ = s.db.ExecContext(ctx, "INSTALL "+name)
	if _, err := s.db.ExecContext(ctx, "LOAD "+name); err != nil {
		return fmt.Errorf("duckdb extension %q is unavailable: %w", name, err)
	}
	return nil
}

func (s *cgoSession) AttachPostgres(ctx context.Context, alias string, dsn string) error {
	if err := s.loadExtension(ctx, "postgres"); err != nil {
		return err
	}
	query := fmt.Sprintf("ATTACH %s AS %s (TYPE POSTGRES, READ_ONLY);", quoteLiteral(dsn), quoteIdent(alias))
	if _, err := s.db.ExecContext(ctx, query); err != nil {
		return fmt.Errorf("failed to attach postgres db %s: %w", alias, err)
	}
	return nil
}

func (s *cgoSession) AttachMySQL(ctx context.Context, alias string, dsn string) error {
	if err := s.loadExtension(ctx, "mysql"); err != nil {
		return err
	}
	query := fmt.Sprintf("ATTACH %s AS %s (TYPE MYSQL, READ_ONLY);", quoteLiteral(dsn), quoteIdent(alias))
	if _, err := s.db.ExecContext(ctx, query); err != nil {
		return fmt.Errorf("failed to attach mysql db %s: %w", alias, err)
	}
	return nil
}

func (s *cgoSession) AttachSQLite(ctx context.Context, alias string, path string) error {
	if err := s.loadExtension(ctx, "sqlite"); err != nil {
		return err
	}
	query := fmt.Sprintf("ATTACH %s AS %s (TYPE SQLITE, READ_ONLY);", quoteLiteral(path), quoteIdent(alias))
	if _, err := s.db.ExecContext(ctx, query); err != nil {
		return fmt.Errorf("failed to attach sqlite db %s: %w", alias, err)
	}
	return nil
}

func (s *cgoSession) RegisterTempView(ctx context.Context, viewName string, query string) error {
	stmt := fmt.Sprintf("CREATE OR REPLACE TEMP VIEW %s AS (%s)", quoteIdent(viewName), query)
	if _, err := s.db.ExecContext(ctx, stmt); err != nil {
		return fmt.Errorf("failed to register temp view %s: %w", viewName, err)
	}
	return nil
}

func (s *cgoSession) RegisterJSONData(ctx context.Context, viewName string, jsonData []byte) error {
	tmpFile, err := os.CreateTemp("", "duckdb_json_*.json")
	if err != nil {
		return fmt.Errorf("failed to create temp json buffer: %w", err)
	}
	if _, err := tmpFile.Write(jsonData); err != nil {
		_ = tmpFile.Close()
		return fmt.Errorf("failed to write json buffer: %w", err)
	}
	_ = tmpFile.Close()

	s.tempFiles = append(s.tempFiles, tmpFile.Name())

	// Normalize path for DuckDB
	normalizedPath := filepath.ToSlash(tmpFile.Name())
	stmt := fmt.Sprintf("CREATE OR REPLACE TEMP VIEW %s AS SELECT * FROM read_json_auto(%s)", quoteIdent(viewName), quoteLiteral(normalizedPath))
	if _, err := s.db.ExecContext(ctx, stmt); err != nil {
		return fmt.Errorf("failed to register json temp view %s: %w", viewName, err)
	}
	return nil
}

func (s *cgoSession) Query(ctx context.Context, sqlQuery string, opts QueryOptions, args ...any) (*QueryResult, error) {
	if s.statementTimeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, s.statementTimeout)
		defer cancel()
	}

	// The cap the caller is served, and the cap the statement is run under: the
	// extra row is only a sentinel that proves whether more rows existed.
	rowLimit := opts.rowLimit()
	plan := PlanLimit(sqlQuery, rowLimit+1)

	start := time.Now()
	rows, err := s.db.QueryContext(ctx, plan.SQL, args...)
	if err != nil {
		if ctx.Err() == context.DeadlineExceeded {
			return nil, fmt.Errorf("query exceeded the %s statement timeout: %w", s.statementTimeout, err)
		}
		return nil, fmt.Errorf("query execution failed: %w", err)
	}
	defer rows.Close()

	colTypes, err := rows.ColumnTypes()
	if err != nil {
		return nil, fmt.Errorf("failed to read column types: %w", err)
	}

	columns := make([]ColumnMeta, len(colTypes))
	for i, ct := range colTypes {
		columns[i] = ColumnMeta{
			Name: ct.Name(),
			Type: ct.DatabaseTypeName(),
		}
	}

	// Initialised as a slice so an empty result set serialises as [] rather than null.
	resultRows := make([]map[string]any, 0)
	colNames, err := rows.Columns()
	if err != nil {
		return nil, fmt.Errorf("failed to read column names: %w", err)
	}

	overCap := false
	for rows.Next() {
		values := make([]any, len(colNames))
		valuePtrs := make([]any, len(colNames))
		for i := range values {
			valuePtrs[i] = &values[i]
		}

		if err := rows.Scan(valuePtrs...); err != nil {
			return nil, fmt.Errorf("failed to scan row: %w", err)
		}

		rowMap := make(map[string]any, len(colNames))
		for i, colName := range colNames {
			rowMap[colName] = jsonValue(values[i])
		}
		resultRows = append(resultRows, rowMap)

		if len(resultRows) > rowLimit {
			overCap = true
			break
		}
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error during row iteration: %w", err)
	}

	if overCap {
		resultRows = resultRows[:rowLimit]
	}

	duration := time.Since(start).Milliseconds()

	result := &QueryResult{
		Columns:       columns,
		Rows:          resultRows,
		RowCount:      int64(len(resultRows)),
		DurationMs:    duration,
		RowLimit:      rowLimit,
		CappedSources: opts.CappedSources,
	}

	switch {
	case overCap:
		result.Truncated = true
		result.TruncatedBy = TruncateRowLimit
	case len(opts.CappedSources) > 0:
		// The statement answered in full, but part of what it could read was
		// already cut off at the source, so the number is not the whole truth.
		result.Truncated = true
		result.TruncatedBy = TruncateSourceCap
	}

	return result, nil
}

// jsonValue flattens driver-only types into values the API envelope can encode.
// DECIMAL arrives as a Go struct without JSON tags, which would otherwise reach
// the browser as an object instead of a number.
func jsonValue(v any) any {
	switch t := v.(type) {
	case nil:
		return nil
	case []byte:
		return string(t)
	case goduckdb.Decimal:
		if t.Value == nil {
			return nil
		}
		return t.Float64()
	default:
		return v
	}
}

func (s *cgoSession) Close() error {
	for _, f := range s.tempFiles {
		_ = os.Remove(f)
	}
	if s.db != nil {
		return s.db.Close()
	}
	return nil
}
