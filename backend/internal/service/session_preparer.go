package service

import (
	"context"
	"fmt"

	"muara.bi/core/internal/domain/entity"
	"muara.bi/core/internal/duckdb"
	"muara.bi/core/internal/repository/pgsql"
	"muara.bi/core/pkg/customerror"
	"muara.bi/core/pkg/logging"
)

const DefaultEnvTag = "default"

// NormalizeEnvTag maps an empty environment tag to the neutral default used for
// cache keys and connection resolution.
func NormalizeEnvTag(envTag string) string {
	if envTag == "" {
		return DefaultEnvTag
	}
	return envTag
}

// PreparedQuery describes one guardrailed analytical execution.
type PreparedQuery struct {
	WorkspaceID string
	ProjectID   string
	EnvTag      string
	RawSQL      string
	Parameters  map[string]any
	MaxRows     int
	RLSRules    []duckdb.RLSRule
}

// SessionPreparer builds tenant-scoped ephemeral DuckDB sessions and executes
// guardrailed queries inside them. It is shared by the synchronous query API
// and the background workers so both enforce identical attach, RLS and limit
// rules.
type SessionPreparer struct {
	engine       duckdb.Engine
	connRepo     pgsql.ConnectionRepositoryProvider
	semanticRepo pgsql.SemanticRepositoryProvider
	connService  ConnectionServiceProvider
	logger       *logging.Logger
}

func NewSessionPreparer(
	engine duckdb.Engine,
	connRepo pgsql.ConnectionRepositoryProvider,
	semanticRepo pgsql.SemanticRepositoryProvider,
	connService ConnectionServiceProvider,
	logger *logging.Logger,
) *SessionPreparer {
	return &SessionPreparer{
		engine:       engine,
		connRepo:     connRepo,
		semanticRepo: semanticRepo,
		connService:  connService,
		logger:       logger,
	}
}

// Execute validates, parameterizes, row-limits and runs the query inside a
// freshly allocated in-memory session that is destroyed on return.
func (p *SessionPreparer) Execute(ctx context.Context, q PreparedQuery) (*duckdb.QueryResult, error) {
	if err := duckdb.ValidateQuery(q.RawSQL); err != nil {
		return nil, customerror.NewBadRequestError("query rejected by guardrails", err)
	}

	binding, err := duckdb.TransformParameters(q.RawSQL, q.Parameters)
	if err != nil {
		return nil, customerror.NewBadRequestError("failed to bind query parameters", err)
	}

	sqlText := binding.ParameterizedSQL
	args := binding.OrderedArgs
	if len(q.RLSRules) > 0 {
		scopedSQL, rlsArgs, err := duckdb.ApplyRLS(sqlText, q.RLSRules, len(args)+1)
		if err != nil {
			return nil, customerror.NewBadRequestError("row-level security could not be applied", err)
		}
		sqlText = scopedSQL
		args = append(args, rlsArgs...)
	}

	maxRows := q.MaxRows
	if maxRows <= 0 {
		maxRows = 10000
	}
	sqlText = duckdb.ApplyLimit(sqlText, maxRows)

	session, err := p.engine.NewSession(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to create analytical session: %w", err)
	}
	defer session.Close()

	if err := p.mountSemanticModels(ctx, session, q); err != nil {
		return nil, err
	}
	p.attachConnections(ctx, session, q.WorkspaceID, NormalizeEnvTag(q.EnvTag))

	return session.Query(ctx, sqlText, args...)
}

func (p *SessionPreparer) mountSemanticModels(ctx context.Context, session duckdb.Session, q PreparedQuery) error {
	if q.ProjectID == "" {
		return nil
	}

	models, err := p.semanticRepo.ListSemanticModelsByProject(ctx, q.WorkspaceID, q.ProjectID)
	if err != nil {
		return fmt.Errorf("failed to load semantic models: %w", err)
	}
	if len(models) == 0 {
		return nil
	}

	smList := make([]duckdb.SemanticModel, 0, len(models))
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

	if err := duckdb.MountSemanticModels(ctx, session, smList); err != nil {
		return fmt.Errorf("failed to mount semantic models: %w", err)
	}
	return nil
}

// attachConnections binds every workspace alias that has a credential profile
// for the active environment tag. Aliases without a matching profile are
// skipped so one unconfigured environment cannot break unrelated queries.
func (p *SessionPreparer) attachConnections(ctx context.Context, session duckdb.Session, wsID, envTag string) {
	conns, err := p.connRepo.ListConnections(ctx, wsID)
	if err != nil {
		p.logger.Warn(fmt.Sprintf("failed to list connections for workspace %s: %v", wsID, err))
		return
	}

	for _, conn := range conns {
		cfg, _, err := p.connService.GetDecryptedConfig(ctx, wsID, conn.ID, envTag)
		if err != nil || cfg == nil {
			continue
		}
		if err := attachConnection(ctx, session, conn, cfg); err != nil {
			p.logger.Warn(fmt.Sprintf("failed to attach alias %s (env=%s): %v", conn.AliasName, envTag, err))
		}
	}
}

func attachConnection(ctx context.Context, session duckdb.Session, conn entity.DataConnection, cfg *entity.ConnectionConfig) error {
	switch conn.DriverType {
	case "postgres":
		sslMode := cfg.SSLMode
		if sslMode == "" {
			sslMode = "disable"
		}
		dsn := fmt.Sprintf("host=%s port=%d user=%s password=%s dbname=%s sslmode=%s",
			cfg.Host, cfg.Port, cfg.User, cfg.Password, cfg.Database, sslMode)
		return session.AttachPostgres(ctx, conn.AliasName, dsn)
	case "mysql":
		dsn := fmt.Sprintf("%s:%s@tcp(%s:%d)/%s", cfg.User, cfg.Password, cfg.Host, cfg.Port, cfg.Database)
		return session.AttachMySQL(ctx, conn.AliasName, dsn)
	case "sqlite":
		return session.AttachSQLite(ctx, conn.AliasName, cfg.Database)
	default:
		return fmt.Errorf("no duckdb attach strategy for driver %q", conn.DriverType)
	}
}
