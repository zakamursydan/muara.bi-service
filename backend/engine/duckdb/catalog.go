package duckdb

import (
	"fmt"
	"strings"
)

// MaxCatalogColumns bounds how many columns one attached source contributes to a
// discovery. Rows are held in memory while they are grouped, and a source wider
// than this could not be used by the query path anyway, so the honest answer is
// to refuse rather than to cache a schema that looks complete.
const MaxCatalogColumns = 5000

// CatalogColumn is one column as the attached source exposes it. The type is
// DuckDB's spelling, not the source's: a scanner maps what it reads onto a DuckDB
// type, and that is the type a statement against the mounted table really sees.
type CatalogColumn struct {
	Name string
	Type string
}

// CatalogTable is one relation of an attached source.
type CatalogTable struct {
	Schema  string
	Name    string
	Columns []CatalogColumn
}

// catalogRow is one flat row of the catalogue query, kept as its own type so the
// grouping rule can be unit tested without a live source.
type catalogRow struct {
	Schema string
	Table  string
	Column string
	Type   string
}

// systemSchemas are the catalogue schemas the scanners mirror into an attached
// catalog. They hold no user data, and a schema discovery that lists them spends
// the analyst's tree and the AI prompt on bookkeeping tables instead.
var systemSchemas = map[string]bool{
	"information_schema": true,
	"pg_catalog":         true,
	"mysql":              true,
	"performance_schema": true,
	"sys":                true,
}

// catalogColumnsQuery reads one attached catalog's columns from DuckDB itself.
// duckdb_columns() is used rather than the source's own information_schema
// because sqlite_scanner exposes none: measured on 2026-11-02 with DuckDB 1.1.3
// and the bundled scanner, litesrc.information_schema.columns answers
// "Catalog Error: Table with name columns does not exist", while duckdb_columns()
// lists the attached catalog like every other one. column_index keeps the
// declaration order the tree shows, and the limit is inlined because it is a
// constant of this package, never caller text.
func catalogColumnsQuery(limit int) string {
	return fmt.Sprintf(`SELECT schema_name, table_name, column_name, data_type
		FROM duckdb_columns()
		WHERE database_name = ? AND internal = false
		ORDER BY schema_name, table_name, column_index
		LIMIT %d`, limit)
}

// groupCatalog folds the flat column rows into tables, preserving the order
// DuckDB reported. The system-schema filter lives here instead of in the WHERE
// clause so the rule is testable without a source to attach.
func groupCatalog(rows []catalogRow) []CatalogTable {
	tables := make([]CatalogTable, 0, len(rows))
	positions := make(map[string]int, len(rows))

	for _, row := range rows {
		if systemSchemas[strings.ToLower(row.Schema)] {
			continue
		}
		key := row.Schema + "." + row.Table
		idx, seen := positions[key]
		if !seen {
			idx = len(tables)
			positions[key] = idx
			tables = append(tables, CatalogTable{
				Schema:  row.Schema,
				Name:    row.Table,
				Columns: make([]CatalogColumn, 0, 8),
			})
		}
		tables[idx].Columns = append(tables[idx].Columns, CatalogColumn{
			Name: row.Column,
			Type: row.Type,
		})
	}

	return tables
}
