package duckdb

import (
	"fmt"
	"strings"
)

// MaxCatalogColumns bounds how many rows one catalogue query may contribute to a
// discovery. Rows are held in memory while they are grouped, and a source wider
// than this could not be used by the query path anyway, so the honest answer is
// to refuse rather than to cache a schema that looks complete.
const MaxCatalogColumns = 5000

// UnknownRowCount is what a table carries when the catalogue reported no size for
// it. DuckDB's duckdb_tables() lists tables only, so a view — a mounted file
// source, a temp view — has columns but no row count, and 0 would read like
// "this relation is empty".
const UnknownRowCount int64 = -1

// CatalogColumn is one column as the attached source exposes it. The type is
// DuckDB's spelling, not the source's: a scanner maps what it reads onto a DuckDB
// type, and that is the type a statement against the mounted table really sees.
type CatalogColumn struct {
	Name string
	Type string

	// Nullable is what the catalogue declares. A column it does not describe is
	// treated as nullable, because claiming NOT NULL is the reading that can make
	// a generated statement wrong.
	Nullable bool

	// Default and Comment are empty when the source has none, which is also the
	// reading for a scanner that carries no comment concept (measured: SQLite
	// reports every comment as NULL).
	Default string
	Comment string
}

// CatalogForeignKey is a reference this table declares to another one — the
// evidence the builder and the AI prompt use to propose a join. The catalogue
// reports the target as a bare table name (measured: referenced_table="parent"),
// so there is no referenced schema to carry.
type CatalogForeignKey struct {
	Name       string
	Columns    []string
	RefTable   string
	RefColumns []string
}

// CatalogTable is one relation of an attached source. Schema and Name are what a
// statement qualifies with, and they stay separate because the alias that makes a
// federated reference complete belongs to the connection, not to the source.
type CatalogTable struct {
	Schema  string
	Name    string
	Comment string

	// EstimatedRows is duckdb_tables().estimated_size: DuckDB's own number, which
	// for a scanner-mounted source is whatever the source reported, not a count of
	// rows this engine read. UnknownRowCount when the catalogue has no entry.
	EstimatedRows int64

	PrimaryKey  []string
	ForeignKeys []CatalogForeignKey
	Columns     []CatalogColumn
}

// catalogRow is one flat row of the column catalogue, kept as its own type so the
// grouping rule can be unit tested without a live source.
type catalogRow struct {
	Schema   string
	Table    string
	Column   string
	Type     string
	Nullable bool
	Default  string
	Comment  string
}

// constraintRow is one flat row of the constraint catalogue. Columns and
// RefColumns come from DuckDB LIST columns, flattened by catalogStrings.
type constraintRow struct {
	Schema     string
	Table      string
	Type       string
	Name       string
	Columns    []string
	RefTable   string
	RefColumns []string
}

// tableRow is one flat row of the table catalogue.
type tableRow struct {
	Schema  string
	Name    string
	Comment string
	Rows    int64
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

// systemSchemaFilter is the same rule rendered for SQL. It is not redundant:
// the row budget is what makes a discovery honest, and a Postgres source keeps
// thousands of information_schema columns before a single user one. The map
// above stays the tested rule — it is case-insensitive and needs no source —
// while this constant only keeps the fetch cheap. The names are constants of
// this package, never caller text, and the catalog alias stays a bound parameter.
const systemSchemaFilter = "schema_name NOT IN ('information_schema', 'pg_catalog', 'mysql', 'performance_schema', 'sys')"

// catalogColumnsQuery reads one attached catalog's columns from DuckDB itself.
// duckdb_columns() is used rather than the source's own information_schema
// because sqlite_scanner exposes none: measured on 2026-11-02 with DuckDB 1.1.3
// and the bundled scanner, litesrc.information_schema.columns answers
// "Catalog Error: Table with name columns does not exist", while duckdb_columns()
// lists the attached catalog like every other one. column_index keeps the
// declaration order the tree shows, and the limit is inlined because it is a
// constant of this package, never caller text.
func catalogColumnsQuery(limit int) string {
	return fmt.Sprintf(`SELECT schema_name, table_name, column_name, data_type,
			is_nullable, column_default, comment
		FROM duckdb_columns()
		WHERE database_name = ? AND internal = false AND %s
		ORDER BY schema_name, table_name, column_index
		LIMIT %d`, systemSchemaFilter, limit)
}

// catalogConstraintsQuery reads the keys of one attached catalog. Every row is
// fetched and sorted out in Go rather than with a WHERE on constraint_type,
// because the spelling is the scanner's: measured on the bundled sqlite_scanner a
// table reports PRIMARY KEY and NOT NULL rows, a native table adds FOREIGN KEY
// with referenced_table and referenced_column_names filled in. Filtering the
// strings here would silently drop keys from a source that spells them
// differently, and a schema cache with no keys reads as "this table has no key".
func catalogConstraintsQuery(limit int) string {
	return fmt.Sprintf(`SELECT schema_name, table_name, constraint_type, constraint_name,
			constraint_column_names, referenced_table, referenced_column_names
		FROM duckdb_constraints()
		WHERE database_name = ? AND %s
		ORDER BY schema_name, table_name, constraint_index
		LIMIT %d`, systemSchemaFilter, limit)
}

// catalogTablesQuery reads the table-level metadata duckdb_columns() cannot
// answer: a comment, and the row count the source reported. Views are absent from
// duckdb_tables() (measured: a temp view has columns but no table row), which is
// why a missing entry becomes UnknownRowCount rather than zero.
func catalogTablesQuery(limit int) string {
	return fmt.Sprintf(`SELECT schema_name, table_name, comment, estimated_size
		FROM duckdb_tables()
		WHERE database_name = ? AND %s
		ORDER BY schema_name, table_name
		LIMIT %d`, systemSchemaFilter, limit)
}

// groupCatalog folds the flat column rows into tables, preserving the order
// DuckDB reported. The system-schema rule is enforced here even though the queries
// filter it too: this is the copy that is testable without a source to attach, and
// it is the case-insensitive one.
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
				Schema:        row.Schema,
				Name:          row.Table,
				EstimatedRows: UnknownRowCount,
				Columns:       make([]CatalogColumn, 0, 8),
			})
		}
		tables[idx].Columns = append(tables[idx].Columns, CatalogColumn{
			Name:     row.Column,
			Type:     row.Type,
			Nullable: row.Nullable,
			Default:  row.Default,
			Comment:  row.Comment,
		})
	}

	return tables
}

// buildCatalog joins the three catalogue reads into the tables one source
// exposes. The column list decides which tables appear: a constraint or a table
// row for a relation nobody described is dropped instead of inventing one, and a
// relation with columns but no table row keeps UnknownRowCount.
func buildCatalog(columns []catalogRow, constraints []constraintRow, tables []tableRow) []CatalogTable {
	grouped := groupCatalog(columns)
	positions := make(map[string]int, len(grouped))
	for i, t := range grouped {
		positions[t.Schema+"."+t.Name] = i
	}

	for _, row := range tables {
		idx, ok := positions[row.Schema+"."+row.Name]
		if !ok {
			continue
		}
		if row.Comment != "" {
			grouped[idx].Comment = row.Comment
		}
		grouped[idx].EstimatedRows = row.Rows
	}

	for _, row := range constraints {
		idx, ok := positions[row.Schema+"."+row.Table]
		if !ok {
			continue
		}
		switch strings.ToUpper(row.Type) {
		case "PRIMARY KEY":
			// DuckDB reports a composite key as one row holding both columns, so
			// the first PRIMARY KEY row is the key; anything later is a duplicate
			// reading of the same table, not a second key.
			if len(grouped[idx].PrimaryKey) == 0 {
				grouped[idx].PrimaryKey = row.Columns
			}
		case "FOREIGN KEY":
			grouped[idx].ForeignKeys = append(grouped[idx].ForeignKeys, CatalogForeignKey{
				Name:       row.Name,
				Columns:    row.Columns,
				RefTable:   row.RefTable,
				RefColumns: row.RefColumns,
			})
		}
		// NOT NULL is deliberately not read here: is_nullable already carries it,
		// and CHECK has no meaning for a tree, a completion list or a join hint.
	}

	return grouped
}

// catalogStrings flattens a DuckDB LIST of VARCHAR into Go strings. The driver
// hands a LIST to database/sql as []interface{} (measured on 1.1.3: a composite
// key arrives as []interface{}{"pid","org"}), and an element that is not a usable
// name is dropped rather than turned into an empty column a JOIN could not
// resolve. It lives here, untagged, because the rule has to be testable in a
// CGO_ENABLED=0 build as well.
func catalogStrings(v any) []string {
	switch list := v.(type) {
	case nil:
		return nil
	case []string:
		return list
	case []any:
		out := make([]string, 0, len(list))
		for _, item := range list {
			s, ok := item.(string)
			if !ok || s == "" {
				continue
			}
			out = append(out, s)
		}
		return out
	default:
		return nil
	}
}
