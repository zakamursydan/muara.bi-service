//go:build cgo

package duckdb

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"testing"
)

func writeSourceFile(t *testing.T, name, content string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), name)
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatalf("write fixture: %v", err)
	}
	return path
}

func openSession(t *testing.T, cfg GuardrailConfig) Session {
	t.Helper()
	engine, err := NewEngine(cfg)
	if errors.Is(err, ErrCGODisabled) {
		t.Skip(ErrCGODisabled.Error())
	}
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

// FR-2 needs more than rows: the picker shows column names and types, and both
// come from DuckDB's own inference, not from a parser this codebase would have
// to keep in step with every CSV dialect.
func TestRegisterFileSourceReadsRowsAndInfersColumns(t *testing.T) {
	session := openSession(t, DefaultGuardrailConfig())
	ctx := context.Background()

	path := writeSourceFile(t, "orders.csv", "channel,total\nonline,120.5\nretail,80\nwholesale,540.25\n")
	if err := session.RegisterFileSource(ctx, "orders", path); err != nil {
		t.Fatalf("register file source: %v", err)
	}

	counted, err := session.Query(ctx, "SELECT count(*) AS rows_in_file FROM orders", QueryOptions{})
	if err != nil {
		t.Fatalf("count the mounted file: %v", err)
	}
	if got := asInt(counted.Rows[0]["rows_in_file"]); got != 3 {
		t.Errorf("rows_in_file = %d, want 3", got)
	}

	shaped, err := session.Query(ctx, "DESCRIBE SELECT * FROM orders", QueryOptions{})
	if err != nil {
		t.Fatalf("describe the mounted file: %v", err)
	}
	types := map[string]string{}
	for _, row := range shaped.Rows {
		types[text(row["column_name"])] = text(row["column_type"])
	}
	if len(types) != 2 {
		t.Fatalf("inferred %d columns, want 2: %v", len(types), types)
	}
	if types["channel"] != "VARCHAR" {
		t.Errorf("channel inferred as %q, want VARCHAR", types["channel"])
	}
	if types["total"] != "DOUBLE" {
		t.Errorf("total inferred as %q, want DOUBLE", types["total"])
	}
}

// The claim that makes file sources usable on an offline desktop: they read
// through code compiled into DuckDB, so an empty connector bundle — the one
// state where every ATTACH must fail — must not stop them.
func TestFileSourceNeedsNoBundledConnector(t *testing.T) {
	empty := t.TempDir()
	if err := os.Chmod(empty, 0o500); err != nil {
		t.Skipf("cannot make the bundle read-only: %v", err)
	}
	t.Cleanup(func() { _ = os.Chmod(empty, 0o700) })

	session := openSession(t, GuardrailConfig{
		MaxRows:            DefaultMaxRows,
		ExtensionDirectory: empty,
	})
	ctx := context.Background()

	path := writeSourceFile(t, "events.jsonl", `{"kind":"click","ms":12}
{"kind":"view","ms":40}
`)
	if err := session.RegisterFileSource(ctx, "events", path); err != nil {
		t.Fatalf("register jsonl with no connectors available: %v", err)
	}
	res, err := session.Query(ctx, "SELECT count(*) AS n FROM events WHERE ms > 20", QueryOptions{})
	if err != nil {
		t.Fatalf("query the jsonl source: %v", err)
	}
	if got := asInt(res.Rows[0]["n"]); got != 1 {
		t.Errorf("filtered rows = %d, want 1", got)
	}

	entries, err := os.ReadDir(empty)
	if err != nil {
		t.Fatalf("re-read bundle dir: %v", err)
	}
	if len(entries) != 0 {
		t.Errorf("the bundle dir gained %d entries, so something was downloaded", len(entries))
	}
}

// Rows arrive as driver values, so the assertions spell out the two shapes a
// caller actually sees.
func asInt(v any) int64 {
	switch n := v.(type) {
	case int64:
		return n
	case int:
		return int64(n)
	default:
		return -1
	}
}

func text(v any) string {
	if v == nil {
		return ""
	}
	return fmt.Sprint(v)
}

func TestRegisterFileSourceRefusesUnsafeNamesAndTypes(t *testing.T) {
	session := openSession(t, DefaultGuardrailConfig())
	ctx := context.Background()
	path := writeSourceFile(t, "orders.csv", "a\n1\n")

	// The view name reaches DDL, so it is held to the identifier rule.
	if err := session.RegisterFileSource(ctx, "orders; DROP TABLE t", path); err == nil {
		t.Error("a view name carrying DDL was accepted")
	}
	if err := session.RegisterFileSource(ctx, "orders", filepath.Join(path, "..", "orders.xlsx")); err == nil {
		t.Error("an xlsx path was accepted as a file source")
	} else if !errors.Is(err, ErrUnsupportedFileSource) {
		t.Errorf("xlsx rejected with %v, want ErrUnsupportedFileSource", err)
	}
}
