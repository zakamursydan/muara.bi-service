package duckdb

import (
	"context"
	"errors"
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
