package response

import (
	"encoding/json"
	"strings"
	"testing"

	"muara.bi/engine/duckdb"
)

// The UI renders these four keys verbatim (frontend/src/types/api.ts and
// components/query/truncation-note.tsx), so the wire names are part of the
// contract: renaming a tag here silently removes the truncation marker from
// every result panel, and a partial answer would then read as a whole one.
func TestExecuteQueryResponseCarriesTheTruncationContract(t *testing.T) {
	payload, err := json.Marshal(&ExecuteQueryResponse{
		Columns:       []duckdb.ColumnMeta{{Name: "total", Type: "DOUBLE"}},
		Rows:          []map[string]any{{"total": 1}},
		RowCount:      1,
		DurationMs:    7,
		RowLimit:      10000,
		Truncated:     true,
		TruncatedBy:   duckdb.TruncateSourceCap,
		CappedSources: []string{"logs_sessions"},
		EnvTag:        "production",
	})
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	var keys map[string]json.RawMessage
	if err := json.Unmarshal(payload, &keys); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	for _, key := range []string{"row_limit", "truncated", "truncated_by", "capped_sources"} {
		if _, ok := keys[key]; !ok {
			t.Errorf("wire payload has no %q key: %s", key, payload)
		}
	}

	// row_limit is reported even for a complete answer, because that is how a
	// caller tells "500 of 500" apart from "500 of many".
	complete, err := json.Marshal(&ExecuteQueryResponse{RowCount: 500, RowLimit: 10000})
	if err != nil {
		t.Fatalf("marshal complete answer: %v", err)
	}
	if !strings.Contains(string(complete), `"row_limit":10000`) ||
		!strings.Contains(string(complete), `"truncated":false`) {
		t.Errorf("a complete answer must still state its cap: %s", complete)
	}
	// The reason and the source list are omitempty, so they stay out of the
	// payload instead of arriving as "" and [] the UI would have to decode.
	if strings.Contains(string(complete), "truncated_by") ||
		strings.Contains(string(complete), "capped_sources") {
		t.Errorf("untruncated payload should carry no reason: %s", complete)
	}
}
