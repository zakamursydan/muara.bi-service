//go:build cgo

package duckdb

import (
	"context"
	"testing"
)

// Pairs with cgo_disabled_test.go: the build tags must not be inverted, and the
// CGO path must answer from DuckDB itself.
func TestEngine_WithCGOAnswers(t *testing.T) {
	engine, err := NewEngine(DefaultGuardrailConfig())
	if err != nil {
		t.Fatalf("new engine: %v", err)
	}
	defer engine.Close()

	ctx := context.Background()
	session, err := engine.NewSession(ctx)
	if err != nil {
		t.Fatalf("new session: %v", err)
	}
	defer session.Close()

	res, err := session.Query(ctx, "SELECT 1 AS one")
	if err != nil {
		t.Fatalf("query: %v", err)
	}
	if res.RowCount != 1 {
		t.Fatalf("expected 1 row, got %d", res.RowCount)
	}
	if got := res.Rows[0]["one"]; got != int32(1) && got != int64(1) {
		t.Fatalf("expected duckdb to answer 1, got %#v", got)
	}
}
