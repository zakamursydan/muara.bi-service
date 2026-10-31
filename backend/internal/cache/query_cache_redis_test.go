package cache

import (
	"context"
	"errors"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/redis/go-redis/v9"
	"muara.bi/core/internal/duckdb"
)

// Runs the real Redis round-trip behind ExecuteCachedOrLive: hit/miss accounting,
// scope separation so two tenants can never be served each other's rows, forced
// refresh, expiry and workspace-wide invalidation.
//
//	MUARA_E2E_REDIS=127.0.0.1:6379 go test ./internal/cache -run Redis -v
//
// Keys are written to database 15 and removed again by the cleanup.
func newRedisCache(t *testing.T) *QueryCache {
	t.Helper()

	addr := os.Getenv("MUARA_E2E_REDIS")
	if addr == "" {
		t.Skip("set MUARA_E2E_REDIS=<host:port> to run against a live redis")
	}

	client := redis.NewClient(&redis.Options{Addr: addr, DB: 15})
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	if err := client.Ping(ctx).Err(); err != nil {
		t.Fatalf("redis at %s is not reachable: %v", addr, err)
	}

	t.Cleanup(func() {
		keys, err := client.Keys(context.Background(), "cache:query:*").Result()
		if err != nil {
			t.Errorf("list test keys: %v", err)
			return
		}
		if len(keys) > 0 {
			if err := client.Del(context.Background(), keys...).Err(); err != nil {
				t.Errorf("delete test keys: %v", err)
			}
		}
		_ = client.Close()
	})

	return NewQueryCache(client, time.Minute)
}

func resultWith(rows []map[string]any) *duckdb.QueryResult {
	return &duckdb.QueryResult{
		Columns:  []duckdb.ColumnMeta{{Name: "branch_code", Type: "VARCHAR"}},
		Rows:     rows,
		RowCount: int64(len(rows)),
	}
}

func TestQueryCacheRedisServesSecondReadFromCache(t *testing.T) {
	c := newRedisCache(t)
	ctx := context.Background()

	calls := 0
	queryFunc := func(context.Context) (*duckdb.QueryResult, error) {
		calls++
		return resultWith([]map[string]any{{"branch_code": "BRG-01"}}), nil
	}

	key := c.GenerateKey("ws-a", "prod", "SELECT branch_code FROM orders", QueryCacheScopeFixture{})

	first, cached, err := c.ExecuteCachedOrLive(ctx, key, time.Minute, false, queryFunc)
	if err != nil {
		t.Fatalf("first execution: %v", err)
	}
	if cached || calls != 1 {
		t.Fatalf("first call must miss: cached=%v calls=%d", cached, calls)
	}

	second, cached, err := c.ExecuteCachedOrLive(ctx, key, time.Minute, false, queryFunc)
	if err != nil {
		t.Fatalf("second execution: %v", err)
	}
	if !cached || calls != 1 {
		t.Errorf("second call must hit without re-executing: cached=%v calls=%d", cached, calls)
	}
	if second.RowCount != first.RowCount || second.Rows[0]["branch_code"] != first.Rows[0]["branch_code"] {
		t.Errorf("cached payload differs: %+v vs %+v", second.Rows, first.Rows)
	}
	if len(second.Columns) != 1 || second.Columns[0].Name != "branch_code" {
		t.Errorf("column metadata lost in round-trip: %+v", second.Columns)
	}

	// forceRefresh must drop the entry and re-run the query.
	third, cached, err := c.ExecuteCachedOrLive(ctx, key, time.Minute, true, queryFunc)
	if err != nil {
		t.Fatalf("forced refresh: %v", err)
	}
	if cached || calls != 2 {
		t.Errorf("force refresh should re-execute: cached=%v calls=%d", cached, calls)
	}
	if third.Rows[0]["branch_code"] != "BRG-01" {
		t.Errorf("unexpected rows after refresh: %v", third.Rows)
	}
}

// QueryCacheScopeFixture mirrors service.QueryCacheScope. The cache package must
// not import service, so scope separation is verified through the JSON shape the
// service actually hashes: {"parameters":...,"rls":[{"column":...}]}.
type QueryCacheScopeFixture struct {
	Parameters map[string]any   `json:"parameters"`
	RLS        []duckdb.RLSRule `json:"rls"`
}

func TestQueryCacheKeysSeparateScopes(t *testing.T) {
	c := newRedisCache(t)
	ctx := context.Background()

	sql := "SELECT branch_code FROM orders"
	tenantA := QueryCacheScopeFixture{RLS: []duckdb.RLSRule{{Column: "branch_code", Operator: "=", Value: "BRG-01"}}}
	tenantB := QueryCacheScopeFixture{RLS: []duckdb.RLSRule{{Column: "branch_code", Operator: "=", Value: "BRG-02"}}}

	keyA := c.GenerateKey("ws-a", "prod", sql, tenantA)
	keyB := c.GenerateKey("ws-a", "prod", sql, tenantB)
	if keyA == keyB {
		t.Fatal("two different RLS scopes produced the same cache key")
	}
	if again := c.GenerateKey("ws-a", "prod", sql, tenantA); again != keyA {
		t.Fatal("cache key is not deterministic")
	}
	if keyA == c.GenerateKey("ws-b", "prod", sql, tenantA) {
		t.Error("workspace must be part of the key")
	}
	if keyA == c.GenerateKey("ws-a", "staging", sql, tenantA) {
		t.Error("environment tag must be part of the key")
	}
	// Trailing whitespace in the author's SQL must not fork a second cache entry.
	if padded := c.GenerateKey("ws-a", "prod", sql+"  \n", tenantA); padded != keyA {
		t.Error("whitespace-only differences should not fork the key")
	}

	// Each scope caches its own rows; serving tenant B from tenant A's entry
	// would be a data leak, so both must execute independently.
	perKey := map[string][]map[string]any{
		keyA: {{"branch_code": "BRG-01"}},
		keyB: {{"branch_code": "BRG-02"}},
	}
	for key, rows := range perKey {
		want := rows
		if _, _, err := c.ExecuteCachedOrLive(ctx, key, time.Minute, false,
			func(context.Context) (*duckdb.QueryResult, error) { return resultWith(want), nil }); err != nil {
			t.Fatalf("seed %s: %v", key, err)
		}
	}
	for key, want := range perKey {
		got, _, err := c.Get(ctx, key)
		if err != nil {
			t.Fatalf("get %s: %v", key, err)
		}
		if got == nil || got.Rows[0]["branch_code"] != want[0]["branch_code"] {
			t.Errorf("scope %s served %v, want %v", key, got, want)
		}
	}
}

func TestQueryCacheRedisExpiryAndInvalidation(t *testing.T) {
	c := newRedisCache(t)
	ctx := context.Background()

	ws := fmt.Sprintf("ws-expiry-%d", time.Now().UnixNano())
	key := c.GenerateKey(ws, "prod", "SELECT 1", QueryCacheScopeFixture{})

	if _, _, err := c.ExecuteCachedOrLive(ctx, key, 150*time.Millisecond, false,
		func(context.Context) (*duckdb.QueryResult, error) { return resultWith([]map[string]any{{"a": 1}}), nil }); err != nil {
		t.Fatalf("seed: %v", err)
	}
	if _, found, err := c.Get(ctx, key); err != nil || !found {
		t.Fatalf("expected a fresh entry: found=%v err=%v", found, err)
	}

	time.Sleep(300 * time.Millisecond)
	if _, found, err := c.Get(ctx, key); err != nil || found {
		t.Errorf("entry survived its TTL: found=%v err=%v", found, err)
	}

	// Invalidation is per workspace, so a neighbour's cache must survive it.
	other := c.GenerateKey("ws-neighbour", "prod", "SELECT 1", QueryCacheScopeFixture{})
	if err := c.Set(ctx, other, resultWith([]map[string]any{{"a": 1}}), time.Minute); err != nil {
		t.Fatalf("seed neighbour: %v", err)
	}
	if err := c.InvalidateByWorkspace(ctx, ws); err != nil {
		t.Fatalf("invalidate workspace: %v", err)
	}
	if _, found, err := c.Get(ctx, key); err != nil || found {
		t.Errorf("workspace invalidation left a key behind: found=%v err=%v", found, err)
	}
	if _, found, _ := c.Get(ctx, other); !found {
		t.Error("workspace invalidation must not touch another workspace")
	}
}

func TestQueryCacheRedisErrorIsNotCached(t *testing.T) {
	c := newRedisCache(t)
	ctx := context.Background()

	calls := 0
	boom := errors.New("duckdb session died")
	queryFunc := func(context.Context) (*duckdb.QueryResult, error) {
		calls++
		return nil, boom
	}

	key := c.GenerateKey("ws-err", "prod", "SELECT 1", QueryCacheScopeFixture{})
	if _, _, err := c.ExecuteCachedOrLive(ctx, key, time.Minute, false, queryFunc); !errors.Is(err, boom) {
		t.Fatalf("expected the query error to surface, got %v", err)
	}
	if _, found, err := c.Get(ctx, key); err != nil || found {
		t.Errorf("a failed query must not be cached: found=%v err=%v", found, err)
	}

	// The next attempt retries instead of replaying the failure.
	if _, _, err := c.ExecuteCachedOrLive(ctx, key, time.Minute, false,
		func(context.Context) (*duckdb.QueryResult, error) { calls++; return resultWith(nil), nil }); err != nil {
		t.Fatalf("retry: %v", err)
	}
	if calls != 2 {
		t.Errorf("calls = %d, want 2", calls)
	}
}
