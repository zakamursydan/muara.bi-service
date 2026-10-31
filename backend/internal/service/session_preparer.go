package service

import (
	"context"
	"errors"
	"fmt"
	"net/url"

	"muara.bi/core/internal/domain/entity"
	"muara.bi/core/internal/limiter"
	"muara.bi/core/internal/repository/pgsql"
	"muara.bi/core/pkg/customerror"
	"muara.bi/core/pkg/logging"
	"muara.bi/engine/duckdb"
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
	slots        *limiter.Semaphore
	logger       *logging.Logger
}

func NewSessionPreparer(
	engine duckdb.Engine,
	connRepo pgsql.ConnectionRepositoryProvider,
	semanticRepo pgsql.SemanticRepositoryProvider,
	connService ConnectionServiceProvider,
	slots *limiter.Semaphore,
	logger *logging.Logger,
) *SessionPreparer {
	return &SessionPreparer{
		engine:       engine,
		connRepo:     connRepo,
		semanticRepo: semanticRepo,
		connService:  connService,
		slots:        slots,
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
		maxRows = duckdb.DefaultMaxRows
	}

	// The budget is taken after the cheap validation and SQL rewrite, so a query
	// that could never run does not consume a slot or make a valid one wait.
	release, err := p.slots.Acquire(ctx)
	if err != nil {
		if errors.Is(err, context.Canceled) {
			return nil, err
		}
		return nil, customerror.NewTooManyRequestsError(
			"the analytical engine is at capacity, try again shortly", err)
	}
	defer release()

	session, err := p.engine.NewSession(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to create analytical session: %w", err)
	}
	defer session.Close()

	if err := p.mountSemanticModels(ctx, session, q); err != nil {
		return nil, err
	}
	cappedSources, err := p.attachConnections(ctx, session, q.WorkspaceID, NormalizeEnvTag(q.EnvTag))
	if err != nil {
		return nil, err
	}

	// The row cap is enforced by the engine, which also reports honestly whether
	// it cut the answer short (BRD §7.3).
	return session.Query(ctx, sqlText, duckdb.QueryOptions{
		MaxRows:       maxRows,
		CappedSources: cappedSources,
	}, args...)
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
// It returns the mounted views whose source read was capped, which the executed
// query has to declare as truncation.
func (p *SessionPreparer) attachConnections(ctx context.Context, session duckdb.Session, wsID, envTag string) ([]string, error) {
	conns, err := p.connRepo.ListConnections(ctx, wsID)
	if err != nil {
		return nil, fmt.Errorf("failed to list workspace connections: %w", err)
	}

	attempted, failed := 0, 0
	capped := make([]string, 0)
	var lastErr error
	for _, conn := range conns {
		cfg, _, err := p.connService.GetDecryptedConfig(ctx, wsID, conn.ID, envTag)
		if err != nil || cfg == nil {
			continue
		}
		attempted++
		viewCapped, err := attachConnection(ctx, session, conn, cfg)
		if err != nil {
			failed++
			lastErr = err
			p.logger.Warn(fmt.Sprintf("failed to attach alias %s (env=%s): %v", conn.AliasName, envTag, err))
			continue
		}
		capped = append(capped, viewCapped...)
	}

	// When every source configured for this environment failed, the query would
	// only die later on an unrelated "table does not exist" catalog error.
	if attempted > 0 && failed == attempted {
		return nil, customerror.NewBadRequestError(
			fmt.Sprintf("no data source could be attached for environment %q", envTag),
			lastErr,
		)
	}
	return capped, nil
}

func attachConnection(ctx context.Context, session duckdb.Session, conn entity.DataConnection, cfg *entity.ConnectionConfig) ([]string, error) {
	switch conn.DriverType {
	case "postgres":
		sslMode := cfg.SSLMode
		if sslMode == "" {
			sslMode = "disable"
		}
		dsn := fmt.Sprintf("host=%s port=%d user=%s password=%s dbname=%s sslmode=%s",
			cfg.Host, cfg.Port, cfg.User, cfg.Password, cfg.Database, sslMode)
		return nil, session.AttachPostgres(ctx, conn.AliasName, dsn)
	case "mysql":
		dsn := fmt.Sprintf("%s:%s@tcp(%s:%d)/%s", cfg.User, cfg.Password, cfg.Host, cfg.Port, cfg.Database)
		return nil, session.AttachMySQL(ctx, conn.AliasName, dsn)
	case "sqlite":
		return nil, session.AttachSQLite(ctx, conn.AliasName, cfg.Database)
	case "mongodb":
		mounts, err := duckdb.StreamMongoCollections(ctx, session, duckdb.MongoCollectionStream{
			URI:      mongoURI(cfg),
			Database: cfg.Database,
			Alias:    conn.AliasName,
		})
		if err != nil {
			return nil, err
		}
		// A database with no collections mounts nothing, which would otherwise look
		// like a successful attach and then fail on an unknown table.
		if len(mounts) == 0 {
			return nil, fmt.Errorf("mongodb database %q has no collections to mount", cfg.Database)
		}
		return duckdb.CappedMongoViews(mounts), nil
	default:
		return nil, fmt.Errorf("no duckdb attach strategy for driver %q", conn.DriverType)
	}
}

// mongoURI prefers the stored connection string and falls back to the host fields.
// The root MongoDB user lives in `admin`, so authentication is pointed there unless
// the URI already says otherwise.
func mongoURI(cfg *entity.ConnectionConfig) string {
	if cfg.URI != "" {
		return cfg.URI
	}
	if cfg.Host == "" || cfg.Port == 0 {
		return ""
	}

	uri := url.URL{
		Scheme: "mongodb",
		Host:   fmt.Sprintf("%s:%d", cfg.Host, cfg.Port),
		RawQuery: url.Values{
			"authSource":       []string{"admin"},
			"directConnection": []string{"true"},
		}.Encode(),
	}
	if cfg.User != "" {
		uri.User = url.UserPassword(cfg.User, cfg.Password)
	}
	return uri.String()
}
