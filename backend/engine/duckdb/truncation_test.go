//go:build cgo

package duckdb

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"testing"
)

// BRD §7.3: a result that is not the complete answer must carry that fact, and
// the engine — not the UI — is the one that decides it.
func newTruncationSession(t *testing.T, rows int) Session {
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

	docs := make([]map[string]any, 0, rows)
	for i := range rows {
		docs = append(docs, map[string]any{"order_id": i + 1, "revenue": (i + 1) * 1000})
	}
	buffer, err := json.Marshal(docs)
	if err != nil {
		t.Fatalf("marshal fixture: %v", err)
	}
	if err := session.RegisterJSONData(ctx, "sales", buffer); err != nil {
		t.Fatalf("register fixture: %v", err)
	}
	return session
}

func query(t *testing.T, session Session, sql string, opts QueryOptions) *QueryResult {
	t.Helper()

	res, err := session.Query(context.Background(), sql, opts)
	if err != nil {
		t.Fatalf("query %q: %v", sql, err)
	}
	return res
}

func TestQueryReportsRowLimitTruncation(t *testing.T) {
	session := newTruncationSession(t, 12)

	res := query(t, session, "SELECT order_id, revenue FROM sales ORDER BY order_id", QueryOptions{MaxRows: 5})

	if res.RowCount != 5 {
		t.Fatalf("rows = %d, want the cap of 5", res.RowCount)
	}
	if !res.Truncated || res.TruncatedBy != TruncateRowLimit {
		t.Fatalf("capped result must be flagged, got truncated=%v by=%q", res.Truncated, res.TruncatedBy)
	}
	if res.RowLimit != 5 {
		t.Errorf("row_limit = %d, want 5", res.RowLimit)
	}
	// The rows served are still the first five, so the sentinel row must not leak.
	if last := res.Rows[4]["order_id"]; fmt.Sprintf("%v", last) != "5" {
		t.Errorf("rows not trimmed to the cap: %v", res.Rows)
	}
}

// A page that exactly fills the cap is not truncation: the sentinel row is what
// separates "there was more" from "that was all of it".
func TestQueryDoesNotFlagCompleteResult(t *testing.T) {
	for _, tc := range []struct {
		name    string
		maxRows int
	}{
		{"under the cap", 100},
		{"exactly the cap", 12},
	} {
		t.Run(tc.name, func(t *testing.T) {
			session := newTruncationSession(t, 12)

			res := query(t, session, "SELECT order_id FROM sales", QueryOptions{MaxRows: tc.maxRows})

			if res.RowCount != 12 {
				t.Fatalf("rows = %d, want 12", res.RowCount)
			}
			if res.Truncated {
				t.Fatalf("complete answer flagged as truncated: by=%q", res.TruncatedBy)
			}
			if res.TruncatedBy != "" {
				t.Errorf("truncated_by = %q, want empty", res.TruncatedBy)
			}
		})
	}
}

// A narrower LIMIT written by the author is their choice, not a platform cut,
// so it must neither be widened nor reported as truncation.
func TestQueryKeepsAuthorLimitNarrowerThanCap(t *testing.T) {
	session := newTruncationSession(t, 12)

	res := query(t, session, "SELECT order_id FROM sales LIMIT 3", QueryOptions{MaxRows: 100})

	if res.RowCount != 3 {
		t.Fatalf("rows = %d, want the author's 3", res.RowCount)
	}
	if res.Truncated {
		t.Fatalf("author limit flagged as platform truncation: by=%q", res.TruncatedBy)
	}
}

func TestQueryDefaultsToPlatformCap(t *testing.T) {
	session := newTruncationSession(t, 3)

	res := query(t, session, "SELECT order_id FROM sales", QueryOptions{})

	if res.RowLimit != DefaultMaxRows {
		t.Errorf("row_limit = %d, want the platform default %d", res.RowLimit, DefaultMaxRows)
	}
}

// A MongoDB collection streamed to its document ceiling is a cut answer even
// when the SQL itself returned every row DuckDB was given.
func TestQueryReportsSourceCap(t *testing.T) {
	session := newTruncationSession(t, 3)

	res := query(t, session, "SELECT order_id FROM sales", QueryOptions{
		MaxRows:       100,
		CappedSources: []string{"logs_sessions"},
	})

	if res.TruncatedBy != TruncateSourceCap {
		t.Fatalf("truncated_by = %q, want %q", res.TruncatedBy, TruncateSourceCap)
	}
	if !res.Truncated {
		t.Fatal("capped source must flag the result")
	}
	if len(res.CappedSources) != 1 || res.CappedSources[0] != "logs_sessions" {
		t.Errorf("capped_sources = %v, want [logs_sessions]", res.CappedSources)
	}
}

// The row cap is the tighter, more actionable truth, so it wins when both apply.
func TestQueryPrefersRowLimitOverSourceCap(t *testing.T) {
	session := newTruncationSession(t, 12)

	res := query(t, session, "SELECT order_id FROM sales", QueryOptions{
		MaxRows:       4,
		CappedSources: []string{"logs_sessions"},
	})

	if res.TruncatedBy != TruncateRowLimit {
		t.Fatalf("truncated_by = %q, want %q", res.TruncatedBy, TruncateRowLimit)
	}
}
