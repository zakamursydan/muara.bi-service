//go:build cgo

package duckdb

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"time"

	goduckdb "github.com/marcboeker/go-duckdb"

	"muara.bi/engine/sqlident"
)

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
	threads, err := cfg.threads()
	if err != nil {
		return nil, err
	}
	return &cgoEngine{
		maxMemory:        maxMemory,
		threads:          threads,
		statementTimeout: cfg.StatementTimeout,
		// Threads comes from the profile, not from the machine: a web pod sizes it
		// as a share of the CPU budget it is given, a single-user desktop as most of
		// its own cores. NormalizeThreads clamps either way.
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
	// bundledExtensions is true when the operator pointed the session at a
	// directory of pre-shipped extension files.
	bundledExtensions bool
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
		// A configured bundle is a promise that nothing has to be downloaded.
		// Without these two settings DuckDB quietly reaches for
		// extensions.duckdb.org, so an offline machine stalls until the fetch
		// times out instead of reporting the missing connector.
		initStatements = append(initStatements,
			"SET extension_directory="+quoteLiteral(e.extensionDirectory),
			"SET autoinstall_known_extensions=false",
			"SET autoload_known_extensions=false",
		)
	}

	for _, stmt := range initStatements {
		if _, err := db.ExecContext(ctx, stmt); err != nil {
			_ = db.Close()
			return nil, fmt.Errorf("failed to apply duckdb guardrail statement %q: %w", stmt, err)
		}
	}

	return &cgoSession{
		db:                db,
		statementTimeout:  e.statementTimeout,
		bundledExtensions: e.extensionDirectory != "",
	}, nil
}

// loadExtension makes a connector available for ATTACH. When the operator
// configured an extension directory, the file is expected to already be there:
// INSTALL is skipped entirely, because it reaches for extensions.duckdb.org
// even with autoinstall disabled, and an offline machine would stall on the
// fetch instead of reporting the missing connector. Without a bundle INSTALL
// stays best effort, and only a failed LOAD is reported — swallowing it used to
// surface as a confusing "Catalog Error" later.
func (s *cgoSession) loadExtension(ctx context.Context, name string) error {
	if !s.bundledExtensions {
		_, _ = s.db.ExecContext(ctx, "INSTALL "+name)
	}
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

// RegisterFileSource mounts one local file as a temporary view. The name is
// held to the identifier rule because it reaches DDL, like the semantic layer;
// the path only ever reaches a quoted string literal.
func (s *cgoSession) RegisterFileSource(ctx context.Context, viewName string, path string) error {
	if !sqlident.IsPlain(viewName) {
		return fmt.Errorf("invalid file source name %q: use letters, digits and underscores", viewName)
	}
	query, err := FileSourceQuery(path)
	if err != nil {
		return err
	}
	return s.RegisterTempView(ctx, viewName, query)
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

// DescribeSource reads the tables, columns and keys of an attached catalog. It
// runs on the session's own connection, so it answers for exactly what a query
// executed in this session could see — including a catalog mounted from a bundle
// on an offline machine.
func (s *cgoSession) DescribeSource(ctx context.Context, alias string) ([]CatalogTable, error) {
	if !sqlident.IsPlain(alias) {
		return nil, fmt.Errorf("invalid source name %q: use letters, digits and underscores", alias)
	}
	if s.statementTimeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, s.statementTimeout)
		defer cancel()
	}

	columns, err := s.scanCatalogColumns(ctx, alias)
	if err != nil {
		return nil, err
	}
	constraints, err := s.scanCatalogConstraints(ctx, alias)
	if err != nil {
		return nil, err
	}
	tables, err := s.scanCatalogTables(ctx, alias)
	if err != nil {
		return nil, err
	}

	return buildCatalog(columns, constraints, tables), nil
}

// catalogTooWide reports a catalogue read that went past the cap. Each query asks
// for one row more than it will accept, so reaching this means the source really
// is wider — and the honest answer is to refuse rather than return a list that
// reads like the whole catalogue.
func catalogTooWide(alias, part string, limit int) error {
	return fmt.Errorf("source %q has more than %d %s; point this connection at a narrower database",
		alias, limit, part)
}

func (s *cgoSession) scanCatalogColumns(ctx context.Context, alias string) ([]catalogRow, error) {
	rows, err := s.db.QueryContext(ctx, catalogColumnsQuery(MaxCatalogColumns+1), alias)
	if err != nil {
		return nil, fmt.Errorf("failed to read the columns of %s: %w", alias, err)
	}
	defer rows.Close()

	scanned := make([]catalogRow, 0, 64)
	for rows.Next() {
		var (
			row          catalogRow
			nullable     sql.NullBool
			defaultValue sql.NullString
			comment      sql.NullString
		)
		if err := rows.Scan(&row.Schema, &row.Table, &row.Column, &row.Type,
			&nullable, &defaultValue, &comment); err != nil {
			return nil, fmt.Errorf("failed to scan a column row of %s: %w", alias, err)
		}
		// A catalogue that does not say whether a column is nullable is read as
		// nullable: inventing a NOT NULL is the direction that breaks a statement.
		row.Nullable = !nullable.Valid || nullable.Bool
		row.Default = defaultValue.String
		row.Comment = comment.String
		scanned = append(scanned, row)

		if len(scanned) > MaxCatalogColumns {
			return nil, catalogTooWide(alias, "columns", MaxCatalogColumns)
		}
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error while reading the columns of %s: %w", alias, err)
	}
	return scanned, nil
}

func (s *cgoSession) scanCatalogConstraints(ctx context.Context, alias string) ([]constraintRow, error) {
	rows, err := s.db.QueryContext(ctx, catalogConstraintsQuery(MaxCatalogColumns+1), alias)
	if err != nil {
		return nil, fmt.Errorf("failed to read the keys of %s: %w", alias, err)
	}
	defer rows.Close()

	scanned := make([]constraintRow, 0, 64)
	for rows.Next() {
		var (
			row        constraintRow
			conName    sql.NullString
			columns    any
			refTable   sql.NullString
			refColumns any
		)
		if err := rows.Scan(&row.Schema, &row.Table, &row.Type, &conName,
			&columns, &refTable, &refColumns); err != nil {
			return nil, fmt.Errorf("failed to scan a constraint row of %s: %w", alias, err)
		}
		// A generated key name is the source's business; a constraint without one
		// still describes a real key.
		row.Name = conName.String
		row.Columns = catalogStrings(columns)
		row.RefTable = refTable.String
		row.RefColumns = catalogStrings(refColumns)
		scanned = append(scanned, row)

		if len(scanned) > MaxCatalogColumns {
			return nil, catalogTooWide(alias, "constraints", MaxCatalogColumns)
		}
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error while reading the keys of %s: %w", alias, err)
	}
	return scanned, nil
}

func (s *cgoSession) scanCatalogTables(ctx context.Context, alias string) ([]tableRow, error) {
	rows, err := s.db.QueryContext(ctx, catalogTablesQuery(MaxCatalogColumns+1), alias)
	if err != nil {
		return nil, fmt.Errorf("failed to read the table metadata of %s: %w", alias, err)
	}
	defer rows.Close()

	scanned := make([]tableRow, 0, 64)
	for rows.Next() {
		var (
			row       tableRow
			comment   sql.NullString
			estimated sql.NullInt64
		)
		if err := rows.Scan(&row.Schema, &row.Name, &comment, &estimated); err != nil {
			return nil, fmt.Errorf("failed to scan a table row of %s: %w", alias, err)
		}
		row.Comment = comment.String
		// Views have no duckdb_tables() entry and a table can carry a NULL size;
		// both stay UnknownRowCount instead of the 0 that reads as "empty".
		if estimated.Valid {
			row.Rows = estimated.Int64
		} else {
			row.Rows = UnknownRowCount
		}
		scanned = append(scanned, row)

		if len(scanned) > MaxCatalogColumns {
			return nil, catalogTooWide(alias, "tables", MaxCatalogColumns)
		}
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error while reading the table metadata of %s: %w", alias, err)
	}
	return scanned, nil
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
