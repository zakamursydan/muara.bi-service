package duckdb

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"testing"
)

// Guardrails are unit-tested as text transformations; these cases run the same
// pipeline SessionPreparer.Execute uses against a real DuckDB session so the
// row filtering, parameter continuation and binding are proven at execution
// time, not just on the composed string.
func newGuardrailSession(t *testing.T) Session {
	t.Helper()

	engine, err := NewEngine(DefaultGuardrailConfig())
	if errors.Is(err, ErrCGODisabled) {
		t.Skip(ErrCGODisabled.Error())
	}
	if err != nil {
		t.Fatalf("new engine: %v", err)
	}
	t.Cleanup(func() { _ = engine.Close() })

	ctx := context.Background()
	session, err := engine.NewSession(ctx)
	if err != nil {
		t.Fatalf("new session: %v", err)
	}
	t.Cleanup(func() { _ = session.Close() })

	rows := []map[string]any{
		{"order_id": 1, "branch_code": "BRG-01", "channel": "web", "revenue": 120000},
		{"order_id": 2, "branch_code": "BRG-02", "channel": "app", "revenue": 85000},
		{"order_id": 3, "branch_code": "BRG-01", "channel": "web", "revenue": 135000},
		{"order_id": 4, "branch_code": "BRG-03", "channel": "pos", "revenue": 45000},
		{"order_id": 5, "branch_code": "BRG-02", "channel": "app", "revenue": 92000},
		{"order_id": 6, "branch_code": "BRG-01", "channel": "web", "revenue": 60000},
	}
	buffer, err := json.Marshal(rows)
	if err != nil {
		t.Fatalf("marshal fixture: %v", err)
	}
	if err := session.RegisterJSONData(ctx, "orders", buffer); err != nil {
		t.Fatalf("register fixture: %v", err)
	}
	return session
}

// runGuardrailed mirrors SessionPreparer.Execute: validate, bind {{param}},
// wrap with RLS continuing the parameter index, then apply the row limit.
func runGuardrailed(t *testing.T, session Session, rawSQL string, params map[string]any, rules []RLSRule, maxRows int) *QueryResult {
	t.Helper()
	ctx := context.Background()

	if err := ValidateQuery(rawSQL); err != nil {
		t.Fatalf("validate: %v", err)
	}
	binding, err := TransformParameters(rawSQL, params)
	if err != nil {
		t.Fatalf("transform: %v", err)
	}
	sqlText := binding.ParameterizedSQL
	args := binding.OrderedArgs

	scopedSQL, rlsArgs, err := ApplyRLS(sqlText, rules, len(args)+1)
	if err != nil {
		t.Fatalf("apply rls: %v", err)
	}
	args = append(args, rlsArgs...)
	sqlText = ApplyLimit(scopedSQL, maxRows)

	res, err := session.Query(ctx, sqlText, args...)
	if err != nil {
		t.Fatalf("query: %v", err)
	}
	return res
}

func TestRLSExecutionFiltersRows(t *testing.T) {
	session := newGuardrailSession(t)

	res := runGuardrailed(t, session,
		`SELECT order_id, branch_code, revenue FROM orders`,
		nil,
		[]RLSRule{{Column: "branch_code", Operator: "=", Value: "BRG-01"}},
		10000)

	if res.RowCount != 3 {
		t.Fatalf("rows = %d, want 3: %v", res.RowCount, res.Rows)
	}
	var total float64
	for _, row := range res.Rows {
		if row["branch_code"] != "BRG-01" {
			t.Errorf("leaked row: %v", row)
		}
		total += asFloat(t, row["revenue"])
	}
	if total != 315000 {
		t.Errorf("visible revenue = %v, want 315000", total)
	}
}

// asFloat reads a DuckDB numeric cell without assuming the exact Go width the
// driver chose for it.
func asFloat(t *testing.T, v any) float64 {
	t.Helper()
	n, err := strconv.ParseFloat(fmt.Sprintf("%v", v), 64)
	if err != nil {
		t.Fatalf("numeric cell %v (%T): %v", v, v, err)
	}
	return n
}

func TestRLSExecutionComposesWithBoundParameters(t *testing.T) {
	session := newGuardrailSession(t)

	// {{channel}} takes $1, so the RLS rule must continue at $2. Sharing one
	// index is what keeps the caller's placeholder and the tenant filter from
	// colliding on a parameter.
	res := runGuardrailed(t, session,
		`SELECT order_id, branch_code, channel, revenue FROM orders WHERE channel = {{channel}}`,
		map[string]any{"channel": "web"},
		[]RLSRule{{Column: "branch_code", Operator: "=", Value: "BRG-01"}},
		10000)

	if res.RowCount != 3 {
		t.Fatalf("rows = %d, want 3: %v", res.RowCount, res.Rows)
	}
	for _, row := range res.Rows {
		if row["branch_code"] != "BRG-01" || row["channel"] != "web" {
			t.Errorf("row outside both filters: %v", row)
		}
	}
}

func TestRLSValueIsBoundNotInterpolated(t *testing.T) {
	session := newGuardrailSession(t)

	// A tenant value that would open up every row if it were concatenated into
	// the SQL text must instead be compared literally and match nothing.
	res := runGuardrailed(t, session,
		`SELECT order_id, branch_code FROM orders`,
		nil,
		[]RLSRule{{Column: "branch_code", Operator: "=", Value: "BRG-01' OR '1'='1"}},
		10000)

	if res.RowCount != 0 {
		t.Errorf("injected rule returned %d rows, want 0: %v", res.RowCount, res.Rows)
	}
}

func TestRLSColumnMustBeProjected(t *testing.T) {
	session := newGuardrailSession(t)
	ctx := context.Background()

	// The tenant predicate is applied outside the author's projection, so a rule
	// on a column they did not select fails with a binder error. That is the
	// fail-closed direction: the alternative would be returning unfiltered rows.
	scopedSQL, rlsArgs, err := ApplyRLS(`SELECT revenue FROM orders`, []RLSRule{
		{Column: "branch_code", Operator: "=", Value: "BRG-01"},
	}, 1)
	if err != nil {
		t.Fatalf("apply rls: %v", err)
	}

	if _, err := session.Query(ctx, ApplyLimit(scopedSQL, 100), rlsArgs...); err == nil {
		t.Fatal("expected the query to fail when the guarded column is not projected")
	}
}

func TestRLSWrapSurvivesAuthorQueryTricks(t *testing.T) {
	session := newGuardrailSession(t)
	rules := []RLSRule{{Column: "branch_code", Operator: "!=", Value: "BRG-03"}}

	// Trailing comment and a caller-supplied LIMIT are both inside the wrapper,
	// so neither can drop the tenant predicate.
	for _, rawSQL := range []string{
		"SELECT order_id, branch_code FROM orders ORDER BY order_id -- grab everything",
		"SELECT order_id, branch_code FROM orders ORDER BY order_id LIMIT 100",
	} {
		res := runGuardrailed(t, session, rawSQL, nil, rules, 2)
		if res.RowCount != 2 {
			t.Fatalf("%q: rows = %d, want 2 (limit must still apply)", rawSQL, res.RowCount)
		}
		for _, row := range res.Rows {
			if row["branch_code"] == "BRG-03" {
				t.Errorf("%q: leaked forbidden row %v", rawSQL, row)
			}
		}
	}
}
