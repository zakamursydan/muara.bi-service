//go:build cgo

package duckdb

import (
	"context"
	"fmt"
	"os"
	"runtime"
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
// still OOMs the container at the old hardcoded 2GB. Both size families are
// asserted because a settings UI that labels its slider "GiB" must land on an
// exact 1024^i budget, while the decimal "GB" spelling means less than it says.
func TestEngineAppliesConfiguredMemoryBudget(t *testing.T) {
	cases := []struct {
		budget string
		want   int64
	}{
		{"256MB", 256_000_000},
		{"2GiB", 2 << 30},
	}

	for _, tc := range cases {
		t.Run(tc.budget, func(t *testing.T) {
			engine, err := NewEngine(GuardrailConfig{MaxMemory: tc.budget, StatementTimeout: 5 * time.Second})
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

			// DuckDB prints the setting in binary units no matter how it was set,
			// so the budget is compared in bytes instead of by string.
			reported, err := parseDuckSize(got)
			if err != nil {
				t.Fatalf("duckdb reports %q: %v", got, err)
			}
			if diff := reported - tc.want; diff < -tc.want/100 || diff > tc.want/100 {
				t.Fatalf("%s budget is not in effect: duckdb reports %q = %d bytes, want %d", tc.budget, got, reported, tc.want)
			}
		})
	}
}

func TestEngineRejectsUnusableMemoryBudget(t *testing.T) {
	if _, err := NewEngine(GuardrailConfig{MaxMemory: "2048"}); err == nil {
		t.Fatal("a bare number must fail at startup, not silently mean 2048 bytes")
	}
	// DUCKDB_MAX_MEMORY=0GiB used to pass validation and reach DuckDB; the point of
	// checking the value is that the operator learns about it here.
	if _, err := NewEngine(GuardrailConfig{MaxMemory: "0GiB"}); err == nil {
		t.Fatal("a zero budget must fail at startup, not disable the memory guardrail")
	}
}

// The thread count has to land in the session, not only in the config. Two things
// make the read-back worth asserting: DuckDB keeps the previous value when a SET
// threads fails, so a config that never applies looks exactly like a working
// limit; and it applies no cap of its own (999 was accepted on an 8-core machine),
// so the visible number is this engine's clamp rather than the host's core count.
func TestEngineAppliesConfiguredThreads(t *testing.T) {
	cores := runtime.NumCPU()
	cases := []struct {
		n    int
		want int
	}{
		{n: 1, want: min(1, cores)},
		{n: 2, want: min(2, cores)},
		{n: 999, want: cores},
	}

	for _, tc := range cases {
		t.Run(fmt.Sprintf("threads=%d", tc.n), func(t *testing.T) {
			engine, err := NewEngine(GuardrailConfig{
				MaxMemory:        "256MB",
				StatementTimeout: 5 * time.Second,
				Threads:          tc.n,
			})
			if err != nil {
				t.Fatalf("new engine with threads=%d: %v", tc.n, err)
			}
			defer engine.Close()

			ctx := context.Background()
			session, err := engine.NewSession(ctx)
			if err != nil {
				t.Fatalf("new session: %v", err)
			}
			defer session.Close()

			res, err := session.Query(ctx, "SELECT current_setting('threads') AS threads", QueryOptions{})
			if err != nil {
				t.Fatalf("read threads setting: %v", err)
			}
			got := strings.TrimSpace(fmt.Sprint(res.Rows[0]["threads"]))
			if got != strconv.Itoa(tc.want) {
				t.Fatalf("configured %d landed as %q on a %d-core host, want %d", tc.n, got, cores, tc.want)
			}
		})
	}
}

func TestEngineRejectsUnusableThreadCount(t *testing.T) {
	// DuckDB answers "Must have at least 1 thread!" for these and leaves the session
	// on its own all-cores default, so the refusal has to happen before that.
	if _, err := NewEngine(GuardrailConfig{Threads: -1}); err == nil {
		t.Fatal("a negative thread count must fail at startup, not widen the CPU budget")
	}
}

// An extension directory is how a desktop bundle promises that its connectors
// ship with the app. DuckDB used to break that promise quietly: with autoinstall
// on and INSTALL still issued, a machine with no network stalled until the fetch
// timed out, and an online one downloaded into the "bundled" folder instead of
// reporting the missing connector. So: a configured bundle that lacks the file
// must fail fast and leave the directory empty.
func TestBundledExtensionDirectoryStaysOffline(t *testing.T) {
	bundle := t.TempDir()
	engine, err := NewEngine(GuardrailConfig{
		MaxMemory:          "256MB",
		StatementTimeout:   5 * time.Second,
		ExtensionDirectory: bundle,
	})
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

	err = session.AttachPostgres(ctx, "sample", "host=127.0.0.1 port=1 user=nobody dbname=none")
	if err == nil {
		t.Fatal("attach succeeded, so the connector came from somewhere other than the bundle")
	}
	if !strings.Contains(err.Error(), "unavailable") {
		t.Fatalf("want the missing-connector message, got %v", err)
	}

	entries, err := os.ReadDir(bundle)
	if err != nil {
		t.Fatalf("read bundle dir: %v", err)
	}
	if len(entries) > 0 {
		t.Fatalf("a bundle miss must not download anything, but %q now holds %d entries", bundle, len(entries))
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
