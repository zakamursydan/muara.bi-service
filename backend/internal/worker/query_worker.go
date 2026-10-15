package worker

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/hibiken/asynq"
	"muara.bi/core/internal/cache"
	"muara.bi/core/internal/duckdb"
	"muara.bi/core/pkg/logging"
)

// QueryWorker handles asynchronous query executions
type QueryWorker struct {
	engine duckdb.Engine
	cache  *cache.QueryCache
	logger *logging.Logger
}

func NewQueryWorker(engine duckdb.Engine, cache *cache.QueryCache, logger *logging.Logger) *QueryWorker {
	return &QueryWorker{
		engine: engine,
		cache:  cache,
		logger: logger,
	}
}

func (w *QueryWorker) ProcessTask(ctx context.Context, t *asynq.Task) error {
	var payload QueryExecutePayload
	if err := json.Unmarshal(t.Payload(), &payload); err != nil {
		return fmt.Errorf("failed to unmarshal query payload: %w", err)
	}

	w.logger.Info(fmt.Sprintf("processing async query for workspace=%s env=%s", payload.WorkspaceID, payload.EnvTag))

	// 1. Guardrail validation
	if err := duckdb.ValidateQuery(payload.RawSQL); err != nil {
		w.logger.Error(fmt.Sprintf("query rejected by guardrail: %v", err))
		return err
	}

	// 2. Mustache parameter transformation
	paramResult, err := duckdb.TransformParameters(payload.RawSQL, payload.Parameters)
	if err != nil {
		return fmt.Errorf("parameter transformation failed: %w", err)
	}

	// 3. Auto LIMIT
	guardedSQL := duckdb.ApplyLimit(paramResult.ParameterizedSQL, 10000)

	// 4. Session allocation & execution
	session, err := w.engine.NewSession(ctx)
	if err != nil {
		return fmt.Errorf("failed to create duckdb session: %w", err)
	}
	defer session.Close()

	result, err := session.Query(ctx, guardedSQL, paramResult.OrderedArgs...)
	if err != nil {
		return fmt.Errorf("execution error: %w", err)
	}

	// 5. Cache result if requested
	if payload.CacheTTL > 0 && w.cache != nil {
		cacheKey := w.cache.GenerateKey(payload.WorkspaceID, payload.EnvTag, payload.RawSQL, payload.Parameters)
		_ = w.cache.Set(ctx, cacheKey, result, payload.CacheTTL)
	}

	w.logger.Info(fmt.Sprintf("async query finished: row_count=%d duration_ms=%d", result.RowCount, result.DurationMs))
	return nil
}
