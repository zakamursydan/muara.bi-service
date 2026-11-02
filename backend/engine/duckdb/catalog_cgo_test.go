//go:build cgo

package duckdb

import (
	"context"
	"os"
	"strings"
	"testing"
	"time"
)

// Pairs with catalog_test.go: the grouping rules are unit tested there, and this
// proves the catalogue SQL is valid against DuckDB itself.

func catalogSession(t *testing.T, extDir string) Session {
	t.Helper()
	engine, err := NewEngine(GuardrailConfig{
		MaxMemory:          "256MB",
		StatementTimeout:   20 * time.Second,
		ExtensionDirectory: extDir,
	})
	if err != nil {
		t.Fatalf("new engine: %v", err)
	}
	t.Cleanup(func() { _ = engine.Close() })

	session, err := engine.NewSession(context.Background())
	if err != nil {
		t.Fatalf("new session: %v", err)
	}
	t.Cleanup(func() { _ = session.Close() })
	return session
}

// A mounted view is the cheapest relation DuckDB can be given without a source
// server, and DescribeSource has to answer with the columns DuckDB inferred from
// the SELECT — which is the same inference a file source's reader performs.
func TestDescribeSourceReadsAMountedView(t *testing.T) {
	session := catalogSession(t, "")
	ctx := context.Background()

	if err := session.RegisterTempView(ctx, "probe_view", "SELECT 1 AS order_id, 'web' AS channel, 2.5 AS total"); err != nil {
		t.Fatalf("register temp view: %v", err)
	}

	tables, err := session.DescribeSource(ctx, "temp")
	if err != nil {
		t.Fatalf("describe source: %v", err)
	}
	if len(tables) != 1 {
		t.Fatalf("want the mounted view, got %#v", tables)
	}

	got := tables[0]
	if got.Name != "probe_view" {
		t.Errorf("table = %q, want probe_view", got.Name)
	}
	want := []string{"order_id", "channel", "total"}
	if len(got.Columns) != len(want) {
		t.Fatalf("columns = %#v, want %d of them", got.Columns, len(want))
	}
	for i, column := range got.Columns {
		// Only the names are asserted strictly: the inferred types are DuckDB's to
		// choose, and a release that widens 2.5 to DOUBLE must not break this test.
		if column.Name != want[i] {
			t.Errorf("column %d = %#v, want name %q", i, column, want[i])
		}
		if column.Type == "" {
			t.Errorf("column %d has no reported type: %#v", i, column)
		}
	}
}

func TestDescribeSourceRejectsAnAliasThatIsNotAnIdentifier(t *testing.T) {
	session := catalogSession(t, "")

	// The name comes from a saved connection, and a crafted one must not be able to
	// reach the catalogue query as anything but a bound value.
	if _, err := session.DescribeSource(context.Background(), `probe"; DROP`); err == nil {
		t.Fatal("want the non-identifier alias refused")
	}
}

// An alias that was never attached has no columns to report. Returning an empty
// list rather than an error keeps the caller in charge of what that means, and
// the service treats it as a failed sync instead of wiping a good cache.
func TestDescribeSourceOnAnUnattachedAliasIsSilentlyEmpty(t *testing.T) {
	session := catalogSession(t, "")

	tables, err := session.DescribeSource(context.Background(), "nothing_here")
	if err != nil {
		t.Fatalf("describe source: %v", err)
	}
	if len(tables) != 0 {
		t.Fatalf("want nothing from an unattached catalog, got %#v", tables)
	}
}

// Runs against the desktop repo's SQLite fixture, which is the only offline
// source this machine has. sqlite_scanner is the driver that proved DuckDB's own
// catalogue is the only uniform path: litesrc.information_schema.columns answers
// "Catalog Error: Table with name columns does not exist" while duckdb_columns()
// lists the attached catalog normally.
//
//	DUCKDB_EXTENSION_DIRECTORY=/Applications/muara-bi.app/Contents/Resources/duckdb/extensions \
//	MUARA_E2E_SQLITE_FILE=/Users/Go/muara.bi-desktop/tests/fixtures/spike.sqlite \
//	  go test ./duckdb -run TestDescribeSourceOnAttachedSQLite -v
func TestDescribeSourceOnAttachedSQLite(t *testing.T) {
	bundle := os.Getenv("DUCKDB_EXTENSION_DIRECTORY")
	file := os.Getenv("MUARA_E2E_SQLITE_FILE")
	if bundle == "" || file == "" {
		t.Skip("set DUCKDB_EXTENSION_DIRECTORY to the connector bundle and MUARA_E2E_SQLITE_FILE to a SQLite database")
	}

	session := catalogSession(t, bundle)
	ctx := context.Background()
	if err := session.AttachSQLite(ctx, "litesrc", file); err != nil {
		t.Fatalf("attach sqlite: %v", err)
	}

	tables, err := session.DescribeSource(ctx, "litesrc")
	if err != nil {
		t.Fatalf("describe source: %v", err)
	}

	orders := findCatalogTable(tables, "orders")
	if orders == nil {
		t.Fatalf("fixture tables not found: %#v", tables)
	}
	// The type spellings are the scanner's mapping, measured rather than assumed:
	// SQLite declares INTEGER and the scanner reports BIGINT.
	want := map[string]string{"id": "BIGINT", "channel": "VARCHAR", "total": "DOUBLE"}
	if len(orders.Columns) != len(want) {
		t.Fatalf("orders columns = %#v, want %d", orders.Columns, len(want))
	}
	for _, column := range orders.Columns {
		if expected, ok := want[column.Name]; !ok {
			t.Errorf("unexpected column %q", column.Name)
		} else if column.Type != expected {
			t.Errorf("column %s type = %q, want the scanner's %q", column.Name, column.Type, expected)
		}
	}

	// The schema the scanner mounted, and the key it read out of the SQLite
	// declaration: CREATE TABLE orders (id INTEGER PRIMARY KEY, ...).
	if orders.Schema != "main" {
		t.Errorf("schema = %q, want main", orders.Schema)
	}
	if got := strings.Join(orders.PrimaryKey, ","); got != "id" {
		t.Errorf("primary key = %v, want the single declared key", orders.PrimaryKey)
	}
	if len(orders.ForeignKeys) != 0 {
		t.Errorf("foreign keys = %#v, want none for this fixture", orders.ForeignKeys)
	}

	// Nullability is the declaration, not the key: measured, id is reported
	// nullable because an INTEGER PRIMARY KEY is a rowid alias, while the two NOT
	// NULL columns are reported as not nullable.
	nullable := map[string]bool{"id": true, "channel": false, "total": false}
	for name, wantNullable := range nullable {
		column := findCatalogColumn(orders, name)
		if column == nil {
			t.Fatalf("column %q missing: %#v", name, orders.Columns)
		}
		if column.Nullable != wantNullable {
			t.Errorf("column %s nullable = %v, want %v", name, column.Nullable, wantNullable)
		}
	}

	// SQLite has no comment concept, so the catalogue answers NULL and the field
	// stays empty rather than invented.
	if orders.Comment != "" {
		t.Errorf("table comment = %q, want empty for a source without comments", orders.Comment)
	}

	// The scanner does report a size. Measured on 2026-11-02: estimated_size = 5
	// for a fixture that holds 6 rows, so this number is the source's own reading
	// and not a count this engine took — a UI that shows it has to call it an
	// estimate.
	if orders.EstimatedRows == UnknownRowCount {
		t.Errorf("rows = %d, want the size the scanner reported", orders.EstimatedRows)
	}
	for _, table := range tables {
		if table.Schema == "information_schema" || table.Schema == "pg_catalog" {
			t.Errorf("system schema %q reached the discovery", table.Schema)
		}
	}
}

func findCatalogTable(tables []CatalogTable, name string) *CatalogTable {
	for i := range tables {
		if tables[i].Name == name {
			return &tables[i]
		}
	}
	return nil
}

// execSourceDDL runs statement the Session interface deliberately does not
// expose. It stays inside the package so the constraint path can be measured
// against DuckDB's own catalog instead of being asserted from a source server
// this machine does not have.
func execSourceDDL(t *testing.T, session Session, stmt string) {
	t.Helper()
	cgo, ok := session.(*cgoSession)
	if !ok {
		t.Fatalf("want the CGO session to create the fixture tables, got %T", session)
	}
	if _, err := cgo.db.ExecContext(context.Background(), stmt); err != nil {
		t.Fatalf("%s: %v", stmt, err)
	}
}

// The keys and the row count have to be read from a real DuckDB catalog, not just
// grouped from test rows. A temp table pair is the only source with a composite
// primary key and a foreign key that this machine can offer without a server.
func TestDescribeSourceReadsKeysFromADuckDBCatalog(t *testing.T) {
	session := catalogSession(t, "")
	ctx := context.Background()

	execSourceDDL(t, session, "CREATE TEMP TABLE parent(id BIGINT, org VARCHAR, note VARCHAR, PRIMARY KEY(id, org))")
	execSourceDDL(t, session, "CREATE TEMP TABLE child(pid BIGINT, org VARCHAR, total DOUBLE, PRIMARY KEY(pid, org), FOREIGN KEY(pid, org) REFERENCES parent(id, org))")
	execSourceDDL(t, session, "CREATE TEMP VIEW keyless_view AS SELECT 1 AS a")

	tables, err := session.DescribeSource(ctx, "temp")
	if err != nil {
		t.Fatalf("describe source: %v", err)
	}

	child := findCatalogTable(tables, "child")
	if child == nil {
		t.Fatalf("child not found: %#v", tables)
	}
	if got := strings.Join(child.PrimaryKey, ","); got != "pid,org" {
		t.Errorf("primary key = %v, want the composite pair DuckDB declared", child.PrimaryKey)
	}
	if len(child.ForeignKeys) != 1 {
		t.Fatalf("foreign keys = %#v, want the reference to parent", child.ForeignKeys)
	}
	fk := child.ForeignKeys[0]
	// Measured: DuckDB reports the target as a bare table name with its columns
	// filled in, so this is the join evidence a builder or a prompt can use.
	if fk.RefTable != "parent" || strings.Join(fk.RefColumns, ",") != "id,org" {
		t.Errorf("foreign key = %#v, want parent(id, org)", fk)
	}

	parent := findCatalogTable(tables, "parent")
	if parent == nil || len(parent.ForeignKeys) != 0 {
		t.Fatalf("parent = %#v, want it found with no outgoing reference", parent)
	}
	if got := strings.Join(parent.PrimaryKey, ","); got != "id,org" {
		t.Errorf("parent primary key = %v, want id,org", parent.PrimaryKey)
	}

	// A column the source did not declare NOT NULL is reported as nullable, and one
	// inside the primary key is not.
	if column := findCatalogColumn(child, "total"); column == nil || !column.Nullable {
		t.Errorf("total = %#v, want it reported nullable", column)
	}
	if column := findCatalogColumn(child, "pid"); column == nil || column.Nullable {
		t.Errorf("pid = %#v, want it reported not nullable through the key", column)
	}

	// Both tables were just created and are empty, so 0 is the catalogue's own
	// answer — which is what makes UnknownRowCount a different statement.
	if child.EstimatedRows != 0 {
		t.Errorf("child rows = %d, want the 0 an empty table reports", child.EstimatedRows)
	}

	// A view keeps its columns and its no-key reading: it is not in
	// duckdb_tables(), and its size is unknown rather than zero.
	view := findCatalogTable(tables, "keyless_view")
	if view == nil {
		t.Fatalf("view missing from the discovery: %#v", tables)
	}
	if view.EstimatedRows != UnknownRowCount {
		t.Errorf("view rows = %d, want %d (no duckdb_tables entry)", view.EstimatedRows, UnknownRowCount)
	}
	if len(view.PrimaryKey) != 0 || len(view.ForeignKeys) != 0 {
		t.Errorf("view keys = %#v / %#v, want none", view.PrimaryKey, view.ForeignKeys)
	}
}

func findCatalogColumn(table *CatalogTable, name string) *CatalogColumn {
	if table == nil {
		return nil
	}
	for i := range table.Columns {
		if table.Columns[i].Name == name {
			return &table.Columns[i]
		}
	}
	return nil
}
