package service

import (
	"context"
	"fmt"
	"time"

	"muara.bi/core/internal/cache"
	"muara.bi/core/internal/domain/entity"
	"muara.bi/core/internal/dto/request"
	"muara.bi/core/internal/dto/response"
	"muara.bi/core/internal/duckdb"
	"muara.bi/core/internal/repository/pgsql"
	"muara.bi/core/pkg/logging"
)

type QueryServiceProvider interface {
	ExecuteQuery(ctx context.Context, wsID string, req request.ExecuteQueryRequest, userAttributes map[string]any) (*response.ExecuteQueryResponse, error)
}

type QueryService struct {
	engine     duckdb.Engine
	connRepo   pgsql.ConnectionRepositoryProvider
	semanticRepo pgsql.SemanticRepositoryProvider
	connService ConnectionServiceProvider
	cache      *cache.QueryCache
	logger     *logging.Logger
}

func NewQueryService(
	engine duckdb.Engine,
	connRepo pgsql.ConnectionRepositoryProvider,
	semanticRepo pgsql.SemanticRepositoryProvider,
	connService ConnectionServiceProvider,
	cache *cache.QueryCache,
	logger *logging.Logger,
) *QueryService {
	return &QueryService{
		engine:       engine,
		connRepo:     connRepo,
		semanticRepo: semanticRepo,
		connService:  connService,
		cache:        cache,
		logger:       logger,
	}
}

func (s *QueryService) ExecuteQuery(
	ctx context.Context,
	wsID string,
	req request.ExecuteQueryRequest,
	userAttributes map[string]any,
) (*response.ExecuteQueryResponse, error) {
	// 1. Guardrail validation
	if err := duckdb.ValidateQuery(req.RawSQL); err != nil {
		return nil, fmt.Errorf("query guardrail violation: %w", err)
	}

	// 2. Bound execution timeout
	execCtx, cancel := duckdb.WithGuardrailTimeout(ctx, 30*time.Second)
	defer cancel()

	// 3. Transform parameters: {{variable}} -> $1, $2
	paramResult, err := duckdb.TransformParameters(req.RawSQL, req.Parameters)
	if err != nil {
		return nil, fmt.Errorf("parameter transformation error: %w", err)
	}

	// 4. Apply Auto LIMIT
	maxRows := req.MaxRows
	if maxRows <= 0 {
		maxRows = 10000
	}
	guardedSQL := duckdb.ApplyLimit(paramResult.ParameterizedSQL, maxRows)

	// 5. Smart Query Cache check
	envTag := req.EnvTag
	if envTag == "" {
		envTag = "default"
	}
	cacheKey := s.cache.GenerateKey(wsID, envTag, guardedSQL, req.Parameters)
	ttl := time.Duration(req.CacheTTL) * time.Second
	if ttl <= 0 {
		ttl = 300 * time.Second
	}

	queryFunc := func(c context.Context) (*duckdb.QueryResult, error) {
		session, err := s.engine.NewSession(c)
		if err != nil {
			return nil, fmt.Errorf("failed to create analytical session: %w", err)
		}
		defer session.Close()

		// Mount semantic models if project specified
		if req.ProjectID != "" {
			models, err := s.semanticRepo.ListSemanticModelsByProject(c, wsID, req.ProjectID)
			if err == nil && len(models) > 0 {
				var smList []duckdb.SemanticModel
				for _, m := range models {
					smList = append(smList, duckdb.SemanticModel{
						ID:          m.ID,
						WorkspaceID: m.WorkspaceID,
						ProjectID:   m.ProjectID,
						Name:        m.Name,
						Description: m.Description,
						RawSQL:      m.RawSQL,
					})
				}
				_ = duckdb.MountSemanticModels(c, session, smList)
			}
		}

		// Attach data connections for this workspace and active environment tag
		conns, err := s.connRepo.ListConnections(c, wsID)
		if err == nil {
			for _, conn := range conns {
				cfg, _, err := s.connService.GetDecryptedConfig(c, wsID, conn.ID, envTag)
				if err == nil && cfg != nil {
					s.attachConnectionToSession(c, session, conn, cfg)
				}
			}
		}

		// Execute guarded parameterized query
		return session.Query(c, guardedSQL, paramResult.OrderedArgs...)
	}

	result, cached, err := s.cache.ExecuteCachedOrLive(execCtx, cacheKey, ttl, req.ForceRefresh, queryFunc)
	if err != nil {
		return nil, fmt.Errorf("query execution failed: %w", err)
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

func (s *QueryService) attachConnectionToSession(ctx context.Context, session duckdb.Session, conn entity.DataConnection, cfg *entity.ConnectionConfig) {
	switch conn.DriverType {
	case "postgres":
		dsn := fmt.Sprintf("host=%s port=%d user=%s password=%s dbname=%s sslmode=%s",
			cfg.Host, cfg.Port, cfg.User, cfg.Password, cfg.Database, cfg.SSLMode)
		if cfg.SSLMode == "" {
			dsn += " sslmode=disable"
		}
		_ = session.AttachPostgres(ctx, conn.AliasName, dsn)
	case "mysql":
		dsn := fmt.Sprintf("%s:%s@tcp(%s:%d)/%s", cfg.User, cfg.Password, cfg.Host, cfg.Port, cfg.Database)
		_ = session.AttachMySQL(ctx, conn.AliasName, dsn)
	case "sqlite":
		_ = session.AttachSQLite(ctx, conn.AliasName, cfg.Database)
	}
}
