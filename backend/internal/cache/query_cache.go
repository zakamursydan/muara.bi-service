package cache

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/redis/go-redis/v9"
	"muara.bi/core/internal/duckdb"
)

// QueryCache wraps Redis client for analytical query results caching
type QueryCache struct {
	client     *redis.Client
	defaultTTL time.Duration
}

// NewQueryCache creates a query cache instance
func NewQueryCache(client *redis.Client, defaultTTL time.Duration) *QueryCache {
	if defaultTTL <= 0 {
		defaultTTL = 300 * time.Second
	}
	return &QueryCache{
		client:     client,
		defaultTTL: defaultTTL,
	}
}

// GenerateKey produces a deterministic cache key
// Pattern: cache:query:<ws_id>:<env_tag>:<sha256(raw_sql + serialized_filters)>
func (c *QueryCache) GenerateKey(workspaceID, envTag, rawSQL string, filters any) string {
	filterBytes, _ := json.Marshal(filters)
	hasher := sha256.New()
	hasher.Write([]byte(strings.TrimSpace(rawSQL)))
	hasher.Write([]byte(":"))
	hasher.Write(filterBytes)
	hashSum := hex.EncodeToString(hasher.Sum(nil))

	cleanEnv := strings.TrimSpace(envTag)
	if cleanEnv == "" {
		cleanEnv = "default"
	}

	return fmt.Sprintf("cache:query:%s:%s:%s", workspaceID, cleanEnv, hashSum)
}

// Get retrieves cached QueryResult from Redis
func (c *QueryCache) Get(ctx context.Context, key string) (*duckdb.QueryResult, bool, error) {
	if c.client == nil {
		return nil, false, nil
	}

	val, err := c.client.Get(ctx, key).Bytes()
	if err != nil {
		if err == redis.Nil {
			return nil, false, nil
		}
		return nil, false, fmt.Errorf("failed to read from redis cache: %w", err)
	}

	var res duckdb.QueryResult
	if err := json.Unmarshal(val, &res); err != nil {
		return nil, false, fmt.Errorf("failed to unmarshal cached query result: %w", err)
	}

	return &res, true, nil
}

// Set saves a QueryResult into Redis with the specified TTL
func (c *QueryCache) Set(ctx context.Context, key string, result *duckdb.QueryResult, ttl time.Duration) error {
	if c.client == nil || result == nil {
		return nil
	}

	if ttl <= 0 {
		ttl = c.defaultTTL
	}

	data, err := json.Marshal(result)
	if err != nil {
		return fmt.Errorf("failed to marshal query result for cache: %w", err)
	}

	if err := c.client.Set(ctx, key, data, ttl).Err(); err != nil {
		return fmt.Errorf("failed to set redis cache key: %w", err)
	}

	return nil
}

// Invalidate removes a specific cache key
func (c *QueryCache) Invalidate(ctx context.Context, key string) error {
	if c.client == nil {
		return nil
	}
	return c.client.Del(ctx, key).Err()
}

// InvalidateByWorkspace scans and removes all query cache keys for a given workspace
func (c *QueryCache) InvalidateByWorkspace(ctx context.Context, workspaceID string) error {
	if c.client == nil {
		return nil
	}

	pattern := fmt.Sprintf("cache:query:%s:*", workspaceID)
	var cursor uint64
	for {
		keys, nextCursor, err := c.client.Scan(ctx, cursor, pattern, 100).Result()
		if err != nil {
			return fmt.Errorf("failed to scan cache keys for workspace %s: %w", workspaceID, err)
		}

		if len(keys) > 0 {
			if err := c.client.Del(ctx, keys...).Err(); err != nil {
				return fmt.Errorf("failed to delete cache keys: %w", err)
			}
		}

		cursor = nextCursor
		if cursor == 0 {
			break
		}
	}

	return nil
}

// ExecuteCachedOrLive provides a high-level wrapper honoring forceRefresh and TTL
func (c *QueryCache) ExecuteCachedOrLive(
	ctx context.Context,
	key string,
	ttl time.Duration,
	forceRefresh bool,
	queryFunc func(ctx context.Context) (*duckdb.QueryResult, error),
) (*duckdb.QueryResult, bool, error) {
	if !forceRefresh {
		if cached, found, err := c.Get(ctx, key); err == nil && found {
			return cached, true, nil
		}
	} else {
		_ = c.Invalidate(ctx, key)
	}

	// Live execution
	result, err := queryFunc(ctx)
	if err != nil {
		return nil, false, err
	}

	// Cache in background
	if ttl > 0 {
		_ = c.Set(ctx, key, result, ttl)
	}

	return result, false, nil
}
