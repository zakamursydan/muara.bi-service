//go:build cgo

package duckdb

import (
	"context"
	"os"
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
