//go:build cgo

package service

import (
	"context"
	"encoding/json"
	"os"
	"testing"
	"time"

	"muara.bi/core/internal/domain/entity"
	"muara.bi/engine/duckdb"
)

// TestSyncSchemaStoresTheScannerCatalog walks the whole SQLite path against a real
// source file: the same attach the query path performs, DuckDB's own catalogue, and
// the JSON that the schema tree, the completion source and the AI prompt read back.
// It is the measurement behind "mysql dan sqlite tidak lagi diam".
//
//	DUCKDB_EXTENSION_DIRECTORY=/Applications/muara-bi.app/Contents/Resources/duckdb/extensions \
//	MUARA_E2E_SQLITE_FILE=/Users/Go/muara.bi-desktop/tests/fixtures/spike.sqlite \
//	  go test ./internal/service -run TestSyncSchema -v
func TestSyncSchemaStoresTheScannerCatalog(t *testing.T) {
	bundle, file := requireSQLiteSource(t)

	engine := schemaEngine(t, bundle)
	repo := &fakeConnRepo{
		conn: entity.DataConnection{ID: "conn-1", WorkspaceID: "ws-1", AliasName: "ledger", DriverType: "sqlite"},
	}
	// A SQLite source keeps its location in the database field, exactly as the
	// query path reads it.
	repo.envs = []entity.ConnectionEnvironment{
		sealEnv(t, "conn-1", "production", entity.ConnectionConfig{Database: file}),
	}

	if err := newTestConnectionService(t, repo, engine).
		SyncSchema(context.Background(), "ws-1", "conn-1", "production"); err != nil {
		t.Fatalf("SyncSchema: %v", err)
	}
	if len(repo.savedSchema) == 0 {
		t.Fatal("nothing was persisted to cached_schema")
	}

	// Read back with the shape the consumers declare, so a change they cannot parse
	// fails this test instead of only the dashboard.
	var cached map[string][]struct {
		Name string `json:"name"`
		Type string `json:"type"`
	}
	if err := json.Unmarshal(repo.savedSchema, &cached); err != nil {
		t.Fatalf("cached_schema is not what the readers parse: %v (%s)", err, repo.savedSchema)
	}

	orders, ok := cached["orders"]
	if !ok {
		t.Fatalf("orders missing from %s", repo.savedSchema)
	}
	want := map[string]string{"id": "BIGINT", "channel": "VARCHAR", "total": "DOUBLE"}
	if len(orders) != len(want) {
		t.Fatalf("orders = %+v, want %d columns", orders, len(want))
	}
	for _, column := range orders {
		expected, ok := want[column.Name]
		if !ok {
			t.Errorf("unexpected column %q", column.Name)
			continue
		}
		if column.Type != expected {
			t.Errorf("column %s stored as %q, want the scanner's %q", column.Name, column.Type, expected)
		}
	}

	// The mirrored catalogue schemas are metadata noise, and the AI prompt budget is
	// spent on them if they survive.
	for _, table := range []string{"columns", "tables", "pg_class"} {
		if _, leaked := cached[table]; leaked {
			t.Errorf("system table %q reached the cache: %s", table, repo.savedSchema)
		}
	}
}

func requireSQLiteSource(t *testing.T) (bundle, file string) {
	t.Helper()
	bundle = os.Getenv("DUCKDB_EXTENSION_DIRECTORY")
	file = os.Getenv("MUARA_E2E_SQLITE_FILE")
	if bundle == "" || file == "" {
		t.Skip("set DUCKDB_EXTENSION_DIRECTORY to the connector bundle and MUARA_E2E_SQLITE_FILE to a SQLite database")
	}
	return bundle, file
}

func schemaEngine(t *testing.T, bundle string) duckdb.Engine {
	t.Helper()
	engine, err := duckdb.NewEngine(duckdb.GuardrailConfig{
		MaxMemory:          "256MB",
		StatementTimeout:   20 * time.Second,
		ExtensionDirectory: bundle,
	})
	if err != nil {
		t.Fatalf("new engine: %v", err)
	}
	t.Cleanup(func() { _ = engine.Close() })
	return engine
}
