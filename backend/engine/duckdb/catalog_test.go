package duckdb

import (
	"strings"
	"testing"
)

func TestCatalogColumnsQueryBindsTheAlias(t *testing.T) {
	query := catalogColumnsQuery(1234)

	// The alias comes from a user-saved connection, so it must reach DuckDB as a
	// parameter, never as text inside the statement.
	if !strings.Contains(query, "database_name = ?") {
		t.Fatalf("query does not bind the catalog name: %s", query)
	}
	if strings.Contains(query, "litesrc") || strings.ContainsAny(query, "'") {
		t.Errorf("query carries quoted text, want the alias bound: %s", query)
	}
	if !strings.Contains(query, "LIMIT 1234") {
		t.Errorf("query lost the row budget: %s", query)
	}
	if !strings.Contains(query, "ORDER BY schema_name, table_name, column_index") {
		t.Errorf("query lost the declaration order: %s", query)
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
