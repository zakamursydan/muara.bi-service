package server

import (
	"time"

	"muara.bi/core/internal/cache"
	"muara.bi/core/internal/config"
	"muara.bi/core/internal/duckdb"
	"muara.bi/core/internal/helper/cryptohelper"
	"muara.bi/core/internal/service"
	"muara.bi/core/internal/ssh"
	"muara.bi/core/pkg/logging"
)

type Service struct {
	AuthService       service.AuthServiceProvider
	ConnectionService service.ConnectionServiceProvider
	QueryService      service.QueryServiceProvider
	SemanticService   service.SemanticServiceProvider
	ProjectService    service.ProjectServiceProvider
	DashboardService  service.DashboardServiceProvider
	WidgetService     service.WidgetServiceProvider
	ShareService      service.ShareServiceProvider
	AccessService     service.AccessServiceProvider

	// Preparer is reused by background workers so async jobs enforce the same
	// attach, guardrail and RLS rules as the synchronous query API.
	Preparer   *service.SessionPreparer
	Engine     duckdb.Engine
	QueryCache *cache.QueryCache
}

func NewService(
	repository Repository,
	internalConnection InternalConnection,
	cfg *config.Config,
	logger *logging.Logger,
) Service {
	cryptoHelper, err := cryptohelper.NewCryptoHelper(cfg.AppEncryptionMasterKey)
	if err != nil {
		logger.Fatal("failed to initialize crypto helper: " + err.Error())
	}

	sshPool := ssh.NewTunnelPool()

	duckdbEngine, err := duckdb.NewEngine(duckdb.DefaultGuardrailConfig())
	if err != nil {
		logger.Fatal("failed to initialize duckdb engine: " + err.Error())
	}

	queryCache := cache.NewQueryCache(internalConnection.Redis, 300*time.Second)

	connSvc := service.NewConnectionService(repository.ConnectionRepo, cryptoHelper, sshPool, logger)
	preparer := service.NewSessionPreparer(duckdbEngine, repository.ConnectionRepo, repository.SemanticRepo, connSvc, logger)
	accessSvc := service.NewAccessService(
		repository.UserRepo,
		repository.ProjectRepo,
		repository.DashboardRepo,
		repository.WidgetRepo,
	)

	return Service{
		AuthService:       service.NewAuthService(repository.UserRepo, cfg, logger),
		ConnectionService: connSvc,
		QueryService:      service.NewQueryService(preparer, accessSvc, queryCache, logger),
		SemanticService:   service.NewSemanticService(repository.SemanticRepo),
		ProjectService:    service.NewProjectService(repository.ProjectRepo),
		DashboardService:  service.NewDashboardService(repository.DashboardRepo, repository.WidgetRepo, repository.ProjectRepo),
		WidgetService:     service.NewWidgetService(repository.WidgetRepo, repository.DashboardRepo),
		ShareService: service.NewShareService(
			repository.ShareRepo,
			repository.DashboardRepo,
			repository.WidgetRepo,
			preparer,
			cfg,
			logger,
		),
		AccessService: accessSvc,
		Preparer:      preparer,
		Engine:        duckdbEngine,
		QueryCache:    queryCache,
	}
}

// Close releases the in-process analytical engine.
func (s Service) Close() error {
	if s.Engine == nil {
		return nil
	}
	return s.Engine.Close()
}
