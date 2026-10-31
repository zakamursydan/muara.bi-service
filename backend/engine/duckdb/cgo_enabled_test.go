//go:build cgo

package duckdb

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"testing"
	"time"
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

	res, err := session.Query(ctx, "SELECT 1 AS one", QueryOptions{})
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

// The memory budget has to reach DuckDB, otherwise a deployment that lowers it
// still OOMs the container at the old hardcoded 2GB.
func TestEngineAppliesConfiguredMemoryBudget(t *testing.T) {
	engine, err := NewEngine(GuardrailConfig{MaxMemory: "256MB", StatementTimeout: 5 * time.Second})
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

	res, err := session.Query(ctx, "SELECT current_setting('max_memory') AS budget", QueryOptions{})
	if err != nil {
		t.Fatalf("query: %v", err)
	}
	got, _ := res.Rows[0]["budget"].(string)

	// DuckDB reads "256MB" as 256 million bytes but prints the setting in MiB, so
	// the budget is compared in bytes instead of by string.
	reported, err := parseDuckSize(got)
	if err != nil {
		t.Fatalf("duckdb reports %q: %v", got, err)
	}
	const want = int64(256_000_000)
	if diff := reported - want; diff < -want/100 || diff > want/100 {
		t.Fatalf("256MB budget is not in effect: duckdb reports %q = %d bytes", got, reported)
	}
}

func TestEngineRejectsUnusableMemoryBudget(t *testing.T) {
	if _, err := NewEngine(GuardrailConfig{MaxMemory: "2048"}); err == nil {
		t.Fatal("a bare number must fail at startup, not silently mean 2048 bytes")
	}
}

// parseDuckSize turns a DuckDB size such as "244.1 MiB" into bytes.
func parseDuckSize(s string) (int64, error) {
	fields := strings.Fields(s)
	if len(fields) != 2 {
		return 0, fmt.Errorf("unexpected duckdb size %q", s)
	}
	value, err := strconv.ParseFloat(fields[0], 64)
	if err != nil {
		return 0, fmt.Errorf("unexpected duckdb size %q: %w", s, err)
	}
	units := map[string]float64{
		"b": 1, "kb": 1e3, "mb": 1e6, "gb": 1e9, "tb": 1e12,
		"kib": 1 << 10, "mib": 1 << 20, "gib": 1 << 30, "tib": 1 << 40,
	}
	multiplier, ok := units[strings.ToLower(fields[1])]
	if !ok {
		return 0, fmt.Errorf("unknown duckdb size unit %q", fields[1])
	}
	return int64(value * multiplier), nil
}
