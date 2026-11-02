package duckdb

import (
	"encoding/json"
	"strings"
	"testing"
)

// The web cache stores exactly this struct as JSON and serves the second read
// from it, so a truncation flag that does not survive the round trip disappears
// on a cached result and the reader sees a partial answer marked as complete.
func TestQueryResultRoundTripsTheTruncationContract(t *testing.T) {
	original := &QueryResult{
		Columns:       []ColumnMeta{{Name: "total", Type: "DOUBLE"}},
		Rows:          []map[string]any{{"total": 1.0}},
		RowCount:      1,
		DurationMs:    7,
		RowLimit:      10000,
		Truncated:     true,
		TruncatedBy:   TruncateSourceCap,
		CappedSources: []string{"logs_sessions"},
	}

	buffer, err := json.Marshal(original)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	var restored QueryResult
	if err := json.Unmarshal(buffer, &restored); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if !restored.Truncated || restored.TruncatedBy != TruncateSourceCap {
		t.Errorf("flag lost: truncated=%v by=%q", restored.Truncated, restored.TruncatedBy)
	}
	if restored.RowLimit != original.RowLimit {
		t.Errorf("row_limit = %d, want %d", restored.RowLimit, original.RowLimit)
	}
	if len(restored.CappedSources) != 1 || restored.CappedSources[0] != "logs_sessions" {
		t.Errorf("capped_sources = %v, want [logs_sessions]", restored.CappedSources)
	}

	// A complete answer must not gain a reason on the way through the cache.
	complete, err := json.Marshal(&QueryResult{RowCount: 500, RowLimit: 10000})
	if err != nil {
		t.Fatalf("marshal complete answer: %v", err)
	}
	if strings.Contains(string(complete), "truncated_by") {
		t.Errorf("untruncated result carries a reason: %s", complete)
	}
}
