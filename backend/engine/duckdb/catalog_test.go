package duckdb

import (
	"regexp"
	"strings"
	"testing"
)

// quotedLiteral finds every single-quoted literal in a catalogue statement.
var quotedLiteral = regexp.MustCompile(`'([^']*)'`)

func TestCatalogColumnsQueryBindsTheAlias(t *testing.T) {
	query := catalogColumnsQuery(1234)

	// The alias comes from a user-saved connection, so it must reach DuckDB as a
	// parameter, never as text inside the statement.
	if !strings.Contains(query, "database_name = ?") {
		t.Fatalf("query does not bind the catalog name: %s", query)
	}
	if strings.Contains(query, "litesrc") {
		t.Errorf("query carries a catalog name as text: %s", query)
	}
	assertOnlySystemLiterals(t, query)
	if !strings.Contains(query, "LIMIT 1234") {
		t.Errorf("query lost the row budget: %s", query)
	}
	if !strings.Contains(query, "ORDER BY schema_name, table_name, column_index") {
		t.Errorf("query lost the declaration order: %s", query)
	}
}

// assertOnlySystemLiterals proves no caller text ever reaches a catalogue
// statement: every quoted literal in it has to be one of this package's own
// system-schema constants.
func assertOnlySystemLiterals(t *testing.T, query string) {
	t.Helper()
	for _, match := range quotedLiteral.FindAllStringSubmatch(query, -1) {
		if !systemSchemas[match[1]] {
			t.Errorf("query quotes %q, which is not a system schema constant: %s", match[1], query)
		}
	}
}

func TestGroupCatalogFoldsRowsIntoTables(t *testing.T) {
	rows := []catalogRow{
		{Schema: "main", Table: "orders", Column: "id", Type: "BIGINT"},
		{Schema: "main", Table: "orders", Column: "channel", Type: "VARCHAR"},
		{Schema: "main", Table: "customers", Column: "name", Type: "VARCHAR"},
	}

	tables := groupCatalog(rows)
	if len(tables) != 2 {
		t.Fatalf("want 2 tables, got %d: %#v", len(tables), tables)
	}
	if tables[0].Name != "orders" || len(tables[0].Columns) != 2 {
		t.Errorf("first table = %#v, want orders with two columns", tables[0])
	}
	// The schema tree and the AI prompt both show columns in this order, so the
	// column_index ordering the query asks for has to survive the grouping.
	if tables[0].Columns[0].Name != "id" || tables[0].Columns[1].Name != "channel" {
		t.Errorf("column order lost: %#v", tables[0].Columns)
	}
}

func TestGroupCatalogDropsMirroredSystemSchemas(t *testing.T) {
	rows := []catalogRow{
		{Schema: "main", Table: "orders", Column: "id", Type: "BIGINT"},
		{Schema: "information_schema", Table: "columns", Column: "table_name", Type: "VARCHAR"},
		{Schema: "PG_CATALOG", Table: "pg_class", Column: "relname", Type: "VARCHAR"},
		{Schema: "mysql", Table: "user", Column: "Host", Type: "VARCHAR"},
		{Schema: "performance_schema", Table: "events_waits_current", Column: "id", Type: "BIGINT"},
		{Schema: "sys", Table: "host_summary", Column: "host", Type: "VARCHAR"},
	}

	tables := groupCatalog(rows)
	if len(tables) != 1 || tables[0].Name != "orders" {
		t.Fatalf("system schemas leaked into the discovery: %#v", tables)
	}
}

// Two databases of one source can hold the same table name, and grouping is keyed
// per schema, so both have to survive as separate entries.
func TestGroupCatalogKeepsSameTableInDifferentSchemas(t *testing.T) {
	rows := []catalogRow{
		{Schema: "sales", Table: "orders", Column: "id", Type: "BIGINT"},
		{Schema: "staging", Table: "orders", Column: "id", Type: "INTEGER"},
	}

	tables := groupCatalog(rows)
	if len(tables) != 2 {
		t.Fatalf("want both schemas kept, got %#v", tables)
	}
	if tables[0].Schema != "sales" || tables[1].Schema != "staging" {
		t.Errorf("schema attribution lost: %#v", tables)
	}
}

func TestGroupCatalogOnNoRows(t *testing.T) {
	// An unattached or empty catalog answers with nothing; the caller has to be
	// able to tell that apart from a failure, so it is an empty slice, not nil.
	tables := groupCatalog(nil)
	if tables == nil {
		t.Fatal("want an initialised slice so the caller can store an empty map")
	}
	if len(tables) != 0 {
		t.Fatalf("want no tables, got %#v", tables)
	}
}

func TestCatalogQueriesBindTheAliasAndKeepTheirOrder(t *testing.T) {
	for name, query := range map[string]string{
		"columns":     catalogColumnsQuery(1234),
		"constraints": catalogConstraintsQuery(1234),
		"table":       catalogTablesQuery(1234),
	} {
		if !strings.Contains(query, "database_name = ?") {
			t.Errorf("%s query does not bind the catalog name: %s", name, query)
		}
		assertOnlySystemLiterals(t, query)
		if !strings.Contains(query, "LIMIT 1234") {
			t.Errorf("%s query lost the row budget: %s", name, query)
		}
		if !strings.Contains(query, "ORDER BY schema_name, table_name") {
			t.Errorf("%s query lost the tree order: %s", name, query)
		}
		// Bookkeeping relations must not consume the budget a wide source is
		// measured against — the Go-side filter drops them after the fetch.
		if !strings.Contains(query, systemSchemaFilter) {
			t.Errorf("%s query reads the mirrored system schemas too: %s", name, query)
		}
	}

	// The column budget is what keeps a wide source from being cached half-read.
	if !strings.Contains(catalogColumnsQuery(1234), "column_index") {
		t.Error("columns query no longer orders by column_index")
	}
	if !strings.Contains(catalogConstraintsQuery(1234), "constraint_index") {
		t.Error("constraints query no longer orders by constraint_index")
	}
}

func TestBuildCatalogAttachesKeysCommentsAndRowCounts(t *testing.T) {
	columns := []catalogRow{
		{Schema: "main", Table: "child", Column: "pid", Type: "BIGINT"},
		{Schema: "main", Table: "child", Column: "org", Type: "VARCHAR", Default: "'acme'"},
		{Schema: "main", Table: "parent", Column: "id", Type: "BIGINT"},
		{Schema: "main", Table: "parent", Column: "org", Type: "VARCHAR"},
		{Schema: "main", Table: "plain_view", Column: "a", Type: "INTEGER", Nullable: true},
	}
	constraints := []constraintRow{
		{Schema: "main", Table: "child", Type: "PRIMARY KEY", Name: "child_pkey", Columns: []string{"pid", "org"}},
		{Schema: "main", Table: "child", Type: "FOREIGN KEY", Name: "child_fkey",
			Columns: []string{"pid", "org"}, RefTable: "parent", RefColumns: []string{"id", "org"}},
		// A key whose table nobody described is not a table: the column list decides
		// what the discovery contains.
		{Schema: "main", Table: "hidden", Type: "PRIMARY KEY", Name: "hidden_pkey", Columns: []string{"id"}},
		// Reported by every scanner, and already carried by is_nullable.
		{Schema: "main", Table: "child", Type: "NOT NULL", Name: "child_pid_not_null", Columns: []string{"pid"}},
	}
	tables := []tableRow{
		{Schema: "main", Name: "child", Comment: "order lines", Rows: 12},
		{Schema: "main", Name: "parent", Rows: 3},
		// A view has no duckdb_tables() entry, so its absence stays UnknownRowCount.
	}

	got := buildCatalog(columns, constraints, tables)
	if len(got) != 3 {
		t.Fatalf("want 3 relations, got %#v", got)
	}

	child := got[0]
	if child.Name != "child" {
		t.Fatalf("first relation = %q, want child", child.Name)
	}
	if strings.Join(child.PrimaryKey, ",") != "pid,org" {
		t.Errorf("primary key = %v, want the composite pair", child.PrimaryKey)
	}
	if len(child.ForeignKeys) != 1 {
		t.Fatalf("foreign keys = %#v, want the one reference", child.ForeignKeys)
	}
	fk := child.ForeignKeys[0]
	if fk.RefTable != "parent" || strings.Join(fk.RefColumns, ",") != "id,org" {
		t.Errorf("foreign key = %#v, want parent(id, org)", fk)
	}
	if child.Comment != "order lines" || child.EstimatedRows != 12 {
		t.Errorf("table metadata = %#v, want the comment and the reported size", child)
	}
	if child.Columns[1].Default != "'acme'" {
		t.Errorf("column default = %q, want the source's own text", child.Columns[1].Default)
	}

	// A relation with columns but no table row keeps keys and an unknown size; a
	// 0 here would tell the tree "this is empty" about something never measured.
	view := got[2]
	if view.Name != "plain_view" || view.EstimatedRows != UnknownRowCount {
		t.Errorf("view = %#v, want %d rows reported", view, UnknownRowCount)
	}
	if len(view.PrimaryKey) != 0 || len(view.ForeignKeys) != 0 {
		t.Errorf("view invented keys: %#v", view)
	}
}

func TestBuildCatalogDropsSystemSchemaRelations(t *testing.T) {
	columns := []catalogRow{
		{Schema: "main", Table: "orders", Column: "id", Type: "BIGINT"},
		{Schema: "information_schema", Table: "columns", Column: "table_name", Type: "VARCHAR"},
	}
	constraints := []constraintRow{
		{Schema: "information_schema", Table: "columns", Type: "PRIMARY KEY", Columns: []string{"table_name"}},
	}

	got := buildCatalog(columns, constraints, nil)
	if len(got) != 1 || got[0].Name != "orders" {
		t.Fatalf("bookkeeping relation reached the discovery: %#v", got)
	}
}

func TestCatalogStringsFlattensDriverLists(t *testing.T) {
	// The driver hands a DuckDB LIST to database/sql as []interface{}; a nil, a
	// non-string element and an empty name must not become a column of a key.
	if got := catalogStrings([]any{"pid", "org"}); strings.Join(got, ",") != "pid,org" {
		t.Errorf("list = %v, want pid,org", got)
	}
	if got := catalogStrings([]any{"pid", 7, "", nil}); strings.Join(got, ",") != "pid" {
		t.Errorf("mixed list = %v, want only the usable name", got)
	}
	if got := catalogStrings(nil); got != nil {
		t.Errorf("nil list = %v, want no columns", got)
	}
	if got := catalogStrings([]string{"a", "b"}); strings.Join(got, ",") != "a,b" {
		t.Errorf("typed list = %v, want both names", got)
	}
	if got := catalogStrings(42); got != nil {
		t.Errorf("unexpected driver value = %v, want no columns", got)
	}
}
