package worker

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/hibiken/asynq"
	"muara.bi/core/internal/cache"
	"muara.bi/core/internal/domain/entity"
	"muara.bi/core/internal/service"
	"muara.bi/core/pkg/logging"
)

// QueryWorker handles asynchronous heavy query executions.
type QueryWorker struct {
	preparer *service.SessionPreparer
	access   service.AccessServiceProvider
	cache    *cache.QueryCache
	logger   *logging.Logger
}

func NewQueryWorker(
	preparer *service.SessionPreparer,
	access service.AccessServiceProvider,
	cache *cache.QueryCache,
	logger *logging.Logger,
) *QueryWorker {
	return &QueryWorker{preparer: preparer, access: access, cache: cache, logger: logger}
}

func (w *QueryWorker) ProcessTask(ctx context.Context, t *asynq.Task) error {
	var payload QueryExecutePayload
	if err := json.Unmarshal(t.Payload(), &payload); err != nil {
		return fmt.Errorf("failed to unmarshal query payload: %w", err)
	}

	access, err := w.access.Resolve(ctx, service.Scope{
		WorkspaceID: payload.WorkspaceID,
		ProjectID:   payload.ProjectID,
		UserID:      payload.UserID,
		Attributes:  payload.UserAttributes,
	})
	if err != nil {
		return err
	}
	if !access.Can(entity.PermQueryExecute) {
		return fmt.Errorf("user %s may not run queries in project %s", payload.UserID, payload.ProjectID)
	}

	envTag := service.NormalizeEnvTag(payload.EnvTag)
	w.logger.Info(fmt.Sprintf("processing async query for workspace=%s env=%s rls_rules=%d",
		payload.WorkspaceID, envTag, len(access.RLSRules)))

	result, err := w.preparer.Execute(ctx, service.PreparedQuery{
		WorkspaceID: payload.WorkspaceID,
		ProjectID:   payload.ProjectID,
		EnvTag:      envTag,
		RawSQL:      payload.RawSQL,
		Parameters:  payload.Parameters,
		MaxRows:     payload.MaxRows,
		RLSRules:    access.RLSRules,
	})
	if err != nil {
		return err
	}

	if payload.CacheTTL > 0 && w.cache != nil {
		cacheKey := w.cache.GenerateKey(payload.WorkspaceID, envTag, payload.RawSQL, service.QueryCacheScope{
			Parameters: payload.Parameters,
			RLS:        access.RLSRules,
		})
		if err := w.cache.Set(ctx, cacheKey, result, payload.CacheTTL); err != nil {
			w.logger.Warn(fmt.Sprintf("failed to persist async query cache: %v", err))
		}
	}

	w.logger.Info(fmt.Sprintf("async query finished: row_count=%d duration_ms=%d", result.RowCount, result.DurationMs))
	return nil
}
