package service

import (
	"context"
	"time"

	"muara.bi/core/internal/cache"
	"muara.bi/core/internal/domain/entity"
	"muara.bi/core/internal/dto/request"
	"muara.bi/core/internal/dto/response"
	"muara.bi/core/internal/duckdb"
	"muara.bi/core/pkg/customerror"
	"muara.bi/core/pkg/logging"
)

type QueryServiceProvider interface {
	ExecuteQuery(ctx context.Context, wsID, userID string, req request.ExecuteQueryRequest, userAttributes map[string]any) (*response.ExecuteQueryResponse, error)
}

type QueryService struct {
	preparer *SessionPreparer
	access   AccessServiceProvider
	cache    *cache.QueryCache
	logger   *logging.Logger
}

func NewQueryService(
	preparer *SessionPreparer,
	access AccessServiceProvider,
	cache *cache.QueryCache,
	logger *logging.Logger,
) *QueryService {
	return &QueryService{
		preparer: preparer,
		access:   access,
		cache:    cache,
		logger:   logger,
	}
}

// QueryCacheScope is folded into the cache key so two callers with different
// row-level filters can never be served each other's result set. Both the
// synchronous API and the async workers must build it identically.
type QueryCacheScope struct {
	Parameters map[string]any   `json:"parameters"`
	RLS        []duckdb.RLSRule `json:"rls"`
}

func (s *QueryService) ExecuteQuery(
	ctx context.Context,
	wsID string,
	userID string,
	req request.ExecuteQueryRequest,
	userAttributes map[string]any,
) (*response.ExecuteQueryResponse, error) {
	access, err := s.access.Resolve(ctx, Scope{
		WorkspaceID: wsID,
		ProjectID:   req.ProjectID,
		UserID:      userID,
		Attributes:  userAttributes,
	})
	if err != nil {
		return nil, err
	}
	if !access.Can(entity.PermQueryExecute) {
		return nil, customerror.NewForbiddenError("you are not allowed to run queries in this project")
	}

	execCtx, cancel := duckdb.WithGuardrailTimeout(ctx, 30*time.Second)
	defer cancel()

	envTag := NormalizeEnvTag(req.EnvTag)
	maxRows := req.MaxRows
	if maxRows <= 0 {
		maxRows = 10000
	}

	cacheKey := s.cache.GenerateKey(wsID, envTag, req.RawSQL, QueryCacheScope{
		Parameters: req.Parameters,
		RLS:        access.RLSRules,
	})
	ttl := time.Duration(req.CacheTTL) * time.Second
	if ttl <= 0 {
		ttl = 300 * time.Second
	}

	queryFunc := func(c context.Context) (*duckdb.QueryResult, error) {
		return s.preparer.Execute(c, PreparedQuery{
			WorkspaceID: wsID,
			ProjectID:   req.ProjectID,
			EnvTag:      envTag,
			RawSQL:      req.RawSQL,
			Parameters:  req.Parameters,
			MaxRows:     maxRows,
			RLSRules:    access.RLSRules,
		})
	}

	result, cached, err := s.cache.ExecuteCachedOrLive(execCtx, cacheKey, ttl, req.ForceRefresh, queryFunc)
	if err != nil {
		return nil, err
	}

	return &response.ExecuteQueryResponse{
		Columns:    result.Columns,
		Rows:       result.Rows,
		RowCount:   result.RowCount,
		DurationMs: result.DurationMs,
		Cached:     cached,
		EnvTag:     envTag,
	}, nil
}
