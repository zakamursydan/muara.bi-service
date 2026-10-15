package cache

import (
	"strings"
	"testing"
	"time"
)

func TestQueryCache_GenerateKey(t *testing.T) {
	cache := NewQueryCache(nil, 300*time.Second)

	wsID := "11111111-2222-3333-4444-555555555555"
	envTag := "production"
	sql := "SELECT * FROM orders WHERE status = 'COMPLETED'"
	filters := map[string]any{"date_range": "30d"}

	k1 := cache.GenerateKey(wsID, envTag, sql, filters)
	k2 := cache.GenerateKey(wsID, envTag, sql, filters)

	if k1 != k2 {
		t.Errorf("expected deterministic keys, got %q vs %q", k1, k2)
	}

	if !strings.HasPrefix(k1, "cache:query:11111111-2222-3333-4444-555555555555:production:") {
		t.Errorf("key did not match expected prefix, got: %s", k1)
	}

	// Change env tag
	k3 := cache.GenerateKey(wsID, "staging", sql, filters)
	if k1 == k3 {
		t.Errorf("keys should differ across environments, got identical key: %s", k1)
	}
}
