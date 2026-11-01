package duckdb

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"
)

func TestValidateQuery_Clean(t *testing.T) {
	validQueries := []string{
		"SELECT * FROM users",
		"SELECT id, name, count(*) FROM orders GROUP BY id, name",
		"WITH summary AS (SELECT * FROM sales) SELECT * FROM summary",
		"/* safe comment */ SELECT 1",
		"-- safe line comment\nSELECT 1",
	}

	for _, q := range validQueries {
		if err := ValidateQuery(q); err != nil {
			t.Errorf("expected valid query, got error for %q: %v", q, err)
		}
	}
}

func TestValidateQuery_Forbidden(t *testing.T) {
	forbiddenQueries := []string{
		"DROP TABLE users",
		"DELETE FROM users WHERE id = 1",
		"UPDATE users SET name = 'hacked'",
		"INSERT INTO users (name) VALUES ('bad')",
		"ALTER TABLE users ADD COLUMN secret text",
		"TRUNCATE TABLE users",
		"PRAGMA version",
		"CREATE TABLE bad (id int)",
		"/* comment */ DROP TABLE users",
		"-- line comment\nDELETE FROM orders",
	}

	for _, q := range forbiddenQueries {
		if err := ValidateQuery(q); err == nil {
			t.Errorf("expected error for forbidden query %q, got nil", q)
		}
	}
}

func TestApplyLimit(t *testing.T) {
	q1 := "SELECT * FROM transactions"
	res1 := ApplyLimit(q1, 1000)
	if !strings.HasSuffix(res1, "LIMIT 1000") {
		t.Errorf("expected LIMIT 1000, got: %s", res1)
	}

	q2 := "SELECT * FROM transactions LIMIT 50"
	res2 := ApplyLimit(q2, 1000)
	if res2 != q2 {
		t.Errorf("expected unmodified limit, got: %s", res2)
	}

	// An author LIMIT may only narrow the cap, never widen it.
	res3 := ApplyLimit("SELECT * FROM transactions LIMIT 500000", 1000)
	if res3 != "SELECT * FROM transactions LIMIT 1000" {
		t.Errorf("expected clamp to 1000, got: %s", res3)
	}

	res4 := ApplyLimit("SELECT * FROM transactions LIMIT 500000 OFFSET 10", 1000)
	if res4 != "SELECT * FROM transactions LIMIT 1000 OFFSET 10" {
		t.Errorf("expected offset preserved, got: %s", res4)
	}

	// A LIMIT buried in a subquery does not bound the outer row count.
	res5 := ApplyLimit("SELECT * FROM (SELECT id FROM t LIMIT 99999) s", 1000)
	if !strings.HasSuffix(res5, "LIMIT 1000") {
		t.Errorf("expected outer limit appended, got: %s", res5)
	}
}

func TestPlanLimit(t *testing.T) {
	// An author limit that already narrows the cap is neither rewritten nor
	// counted as platform truncation.
	narrow := PlanLimit("SELECT * FROM transactions LIMIT 50", 1000)
	if narrow.SQL != "SELECT * FROM transactions LIMIT 50" || narrow.Limit != 50 || narrow.Injected {
		t.Errorf("author limit kept: %+v", narrow)
	}

	widened := PlanLimit("SELECT * FROM transactions LIMIT 500000 OFFSET 10", 1000)
	if widened.SQL != "SELECT * FROM transactions LIMIT 1000 OFFSET 10" || widened.Limit != 1000 || !widened.Injected {
		t.Errorf("author limit clamped: %+v", widened)
	}

	appended := PlanLimit("SELECT * FROM transactions;", 1000)
	if appended.SQL != "SELECT * FROM transactions LIMIT 1000" || appended.Limit != 1000 || !appended.Injected {
		t.Errorf("limit appended and semicolon stripped: %+v", appended)
	}

	// Zero falls back to the platform default instead of running unbounded.
	fallback := PlanLimit("SELECT 1", 0)
	if !strings.HasSuffix(fallback.SQL, fmt.Sprintf("LIMIT %d", DefaultMaxRows)) || fallback.Limit != DefaultMaxRows {
		t.Errorf("default cap applied: %+v", fallback)
	}
}

func TestNormalizeMaxMemory(t *testing.T) {
	for _, ok := range []string{"2GB", "512MB", "8b", "0.5GB", "2GiB", "512MiB"} {
		got, err := NormalizeMaxMemory(ok)
		if err != nil {
			t.Errorf("%q accepted: %v", ok, err)
		}
		if got != strings.TrimSpace(ok) {
			t.Errorf("%q normalized to %q", ok, got)
		}
	}

	// Empty means "use the platform default", not "unbounded".
	if got, err := NormalizeMaxMemory("  "); err != nil || got != DefaultMaxMemory {
		t.Errorf("empty falls back to default: %q err=%v", got, err)
	}
	// Both products ship this same default, and it is spelled in binary units so
	// the number an operator reads is the number DuckDB reserves.
	if DefaultMaxMemory != "2GiB" {
		t.Errorf("DefaultMaxMemory = %q, want the binary spelling", DefaultMaxMemory)
	}

	// DuckDB rejects a bare number itself ("Unknown unit for memory_limit"), and
	// "-1" means unlimited — both must fail here rather than at first use.
	for _, bad := range []string{"2048", "2 GB", "large", "-2GB", "-1", "2GB; DROP TABLE t"} {
		if got, err := NormalizeMaxMemory(bad); err == nil {
			t.Errorf("%q rejected: got %q, err=nil", bad, got)
		}
	}

	// Zero matches the shape of a valid budget, so only a value check catches it.
	// An unbounded session is exactly what the row cap and the budget exist to
	// prevent, and a config error that survives to the first query is the failure
	// mode M1.5 promised to remove.
	for _, zero := range []string{"0", "0B", "0GB", "0GiB", "0.0GiB", "0MiB", " 0TiB "} {
		got, err := NormalizeMaxMemory(zero)
		if err == nil {
			t.Errorf("%q accepted as %q; a zero budget must fail at startup", zero, got)
			continue
		}
		if !errors.Is(err, ErrInvalidMemoryBudget) {
			t.Errorf("%q rejected with the wrong error: %v", zero, err)
		}
	}
}

func TestTransformParameters(t *testing.T) {
	raw := "SELECT * FROM orders WHERE customer_id = {{customer_id}} AND status = {{status}}"
	params := map[string]any{
		"customer_id": 42,
		"status":      "COMPLETED",
	}

	res, err := TransformParameters(raw, params)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	expectedSQL := "SELECT * FROM orders WHERE customer_id = $1 AND status = $2"
	if res.ParameterizedSQL != expectedSQL {
		t.Errorf("expected SQL %q, got %q", expectedSQL, res.ParameterizedSQL)
	}

	if len(res.OrderedArgs) != 2 || res.OrderedArgs[0] != 42 || res.OrderedArgs[1] != "COMPLETED" {
		t.Errorf("unexpected args: %v", res.OrderedArgs)
	}
}

func TestApplyRLS(t *testing.T) {
	raw := "SELECT id, amount, branch_id FROM sales"
	rules := []RLSRule{
		{Column: "branch_id", Operator: "=", Value: "JKT-01"},
	}

	wrapped, args, err := ApplyRLS(raw, rules, 1)
	if err != nil {
		t.Fatalf("ApplyRLS() error = %v", err)
	}
	if !strings.Contains(wrapped, "branch_id = $1") {
		t.Errorf("expected wrapped query with RLS condition, got: %s", wrapped)
	}
	if !strings.Contains(wrapped, "__rls_subquery") {
		t.Errorf("expected the original query to be wrapped, got: %s", wrapped)
	}
	if len(args) != 1 || args[0] != "JKT-01" {
		t.Errorf("expected args ['JKT-01'], got: %v", args)
	}
}

func TestApplyRLSContinuesParameterIndex(t *testing.T) {
	rules := []RLSRule{
		{Column: "branch_id", Operator: "=", Value: "JKT-01"},
		{Column: "team", Operator: "!=", Value: "x"},
	}

	wrapped, args, err := ApplyRLS("SELECT 1", rules, 3)
	if err != nil {
		t.Fatalf("ApplyRLS() error = %v", err)
	}
	if !strings.Contains(wrapped, "branch_id = $3 AND team != $4") {
		t.Fatalf("unexpected RLS clause: %s", wrapped)
	}
	if len(args) != 2 {
		t.Fatalf("expected 2 bound args, got %v", args)
	}
}

func TestApplyRLSRejectsUnsafeRules(t *testing.T) {
	tests := []struct {
		name string
		rule RLSRule
	}{
		{"empty column", RLSRule{Column: "", Operator: "=", Value: "x"}},
		{"stacked statement in column", RLSRule{Column: "branch_id; DROP TABLE users", Operator: "=", Value: "x"}},
		{"boolean bypass in column", RLSRule{Column: "branch_id = 'x' OR 1=1 --", Operator: "=", Value: "x"}},
		{"subquery in column", RLSRule{Column: "(SELECT password FROM users)", Operator: "=", Value: "x"}},
		{"operator injection", RLSRule{Column: "branch_id", Operator: "= 'x' OR 1=1 --", Value: "x"}},
		{"unsupported operator", RLSRule{Column: "branch_id", Operator: "IN", Value: "x"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sql, args, err := ApplyRLS("SELECT 1", []RLSRule{tt.rule}, 1)
			if err == nil {
				t.Fatalf("expected error, got sql=%q args=%v", sql, args)
			}
			if !errors.Is(err, ErrInvalidRLSRule) {
				t.Fatalf("expected ErrInvalidRLSRule, got %v", err)
			}
			if sql != "" || args != nil {
				t.Fatalf("expected no output on rejection, got sql=%q args=%v", sql, args)
			}
		})
	}
}

func TestApplyRLSNoRulesIsPassthrough(t *testing.T) {
	sql, args, err := ApplyRLS("SELECT 1", nil, 1)
	if err != nil {
		t.Fatalf("ApplyRLS() error = %v", err)
	}
	if sql != "SELECT 1" || args != nil {
		t.Fatalf("expected passthrough, got sql=%q args=%v", sql, args)
	}
}

func TestWithGuardrailTimeout(t *testing.T) {
	ctx, cancel := WithGuardrailTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	select {
	case <-time.After(100 * time.Millisecond):
		t.Error("timeout did not trigger")
	case <-ctx.Done():
		// expected
	}
}
