//go:build cgo

package duckdb

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"time"

	_ "github.com/marcboeker/go-duckdb"
)

type cgoEngine struct {
	maxMemory        string
	threads          int
	statementTimeout time.Duration
}

func NewEngine(cfg GuardrailConfig) (Engine, error) {
	return &cgoEngine{
		maxMemory:        "2GB",
		threads:          2,
		statementTimeout: cfg.StatementTimeout,
	}, nil
}

func (e *cgoEngine) Close() error {
	return nil
}

type cgoSession struct {
	db        *sql.DB
	tempFiles []string
}

func (e *cgoEngine) NewSession(ctx context.Context) (Session, error) {
	db, err := sql.Open("duckdb", "")
	if err != nil {
		return nil, fmt.Errorf("failed to initialize duckdb in-memory session: %w", err)
	}

	// Apply strict safety bounds
	initStatements := []string{
		fmt.Sprintf("SET max_memory='%s'", e.maxMemory),
		fmt.Sprintf("SET threads=%d", e.threads),
	}
	if e.statementTimeout > 0 {
		initStatements = append(initStatements, fmt.Sprintf("SET statement_timeout='%s'", e.statementTimeout.String()))
	}

	for _, stmt := range initStatements {
		if _, err := db.ExecContext(ctx, stmt); err != nil {
			_ = db.Close()
			return nil, fmt.Errorf("failed to apply duckdb guardrail statement %q: %w", stmt, err)
		}
	}

	return &cgoSession{
		db: db,
	}, nil
}

func (s *cgoSession) AttachPostgres(ctx context.Context, alias string, dsn string) error {
	_, _ = s.db.ExecContext(ctx, "INSTALL postgres; LOAD postgres;")
	query := fmt.Sprintf("ATTACH '%s' AS %s (TYPE POSTGRES, READ_ONLY);", dsn, alias)
	if _, err := s.db.ExecContext(ctx, query); err != nil {
		return fmt.Errorf("failed to attach postgres db %s: %w", alias, err)
	}
	return nil
}

func (s *cgoSession) AttachMySQL(ctx context.Context, alias string, dsn string) error {
	_, _ = s.db.ExecContext(ctx, "INSTALL mysql; LOAD mysql;")
	query := fmt.Sprintf("ATTACH '%s' AS %s (TYPE MYSQL, READ_ONLY);", dsn, alias)
	if _, err := s.db.ExecContext(ctx, query); err != nil {
		return fmt.Errorf("failed to attach mysql db %s: %w", alias, err)
	}
	return nil
}

func (s *cgoSession) AttachSQLite(ctx context.Context, alias string, path string) error {
	_, _ = s.db.ExecContext(ctx, "INSTALL sqlite; LOAD sqlite;")
	query := fmt.Sprintf("ATTACH '%s' AS %s (TYPE SQLITE, READ_ONLY);", path, alias)
	if _, err := s.db.ExecContext(ctx, query); err != nil {
		return fmt.Errorf("failed to attach sqlite db %s: %w", alias, err)
	}
	return nil
}

func (s *cgoSession) RegisterTempView(ctx context.Context, viewName string, query string) error {
	stmt := fmt.Sprintf("CREATE OR REPLACE TEMP VIEW %s AS (%s)", viewName, query)
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
	stmt := fmt.Sprintf("CREATE OR REPLACE TEMP VIEW %s AS SELECT * FROM read_json_auto('%s')", viewName, normalizedPath)
	if _, err := s.db.ExecContext(ctx, stmt); err != nil {
		return fmt.Errorf("failed to register json temp view %s: %w", viewName, err)
	}
	return nil
}

func (s *cgoSession) Query(ctx context.Context, sqlQuery string, args ...any) (*QueryResult, error) {
	start := time.Now()
	rows, err := s.db.QueryContext(ctx, sqlQuery, args...)
	if err != nil {
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

	var resultRows []map[string]any
	colNames, err := rows.Columns()
	if err != nil {
		return nil, fmt.Errorf("failed to read column names: %w", err)
	}

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
			val := values[i]
			if b, ok := val.([]byte); ok {
				rowMap[colName] = string(b)
			} else {
				rowMap[colName] = val
			}
		}
		resultRows = append(resultRows, rowMap)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error during row iteration: %w", err)
	}

	duration := time.Since(start).Milliseconds()

	return &QueryResult{
		Columns:    columns,
		Rows:       resultRows,
		RowCount:   int64(len(resultRows)),
		DurationMs: duration,
	}, nil
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
