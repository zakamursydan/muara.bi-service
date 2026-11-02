package server

import (
	"fmt"
	"time"

	"github.com/hibiken/asynq"
	"muara.bi/core/internal/cache"
	"muara.bi/core/internal/config"
	"muara.bi/core/internal/limiter"
	"muara.bi/core/internal/service"
	"muara.bi/core/pkg/logging"
	"muara.bi/engine/cryptohelper"
	"muara.bi/engine/duckdb"
	"muara.bi/engine/ssh"
)

// AnalyticalSlotKey is the Redis member set every process contends for. It is
// deliberately a single global budget: DuckDB sessions are bounded by host
// memory, not by workspace.
const AnalyticalSlotKey = "muara:duckdb:slots"

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
	AlertService      service.AlertServiceProvider
	AIService         service.AIServiceProvider

	// Preparer is reused by background workers so async jobs enforce the same
	// attach, guardrail and RLS rules as the synchronous query API.
	Preparer    *service.SessionPreparer
	Engine      duckdb.Engine
	QueryCache  *cache.QueryCache
	AsynqClient *asynq.Client
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

	guardrails := duckdb.DefaultGuardrailConfig()
	guardrails.ExtensionDirectory = cfg.DuckDB.ExtensionDirectory
	guardrails.MaxMemory = cfg.DuckDB.MaxMemory
	guardrails.Threads = cfg.DuckDB.Threads

	duckdbEngine, err := duckdb.NewEngine(guardrails)
	if err != nil {
		logger.Fatal("failed to initialize duckdb engine: " + err.Error())
	}

	queryCache := cache.NewQueryCache(internalConnection.Redis, 300*time.Second)

	asynqClient := asynq.NewClient(asynq.RedisClientOpt{
		Addr:     fmt.Sprintf("%s:%s", cfg.Redis.Host, cfg.Redis.Port),
		Password: cfg.Redis.Password,
		DB:       cfg.Redis.DB,
	})

	connSvc := service.NewConnectionService(repository.ConnectionRepo, cryptoHelper, sshPool, duckdbEngine, logger)
	// One Redis-backed budget spans the API and every worker replica, because a
	// DuckDB session is the resource that actually runs out of memory.
	analyticalSlots := limiter.NewSemaphore(
		internalConnection.Redis,
		AnalyticalSlotKey,
		cfg.DuckDB.MaxConcurrentExecutions,
		time.Duration(cfg.DuckDB.SlotLeaseSeconds)*time.Second,
		time.Duration(cfg.DuckDB.QueueTimeoutSeconds)*time.Second,
		logger,
	)
	preparer := service.NewSessionPreparer(duckdbEngine, repository.ConnectionRepo, repository.SemanticRepo, connSvc, analyticalSlots, logger)
	accessSvc := service.NewAccessService(
		repository.UserRepo,
		repository.ProjectRepo,
		repository.DashboardRepo,
		repository.WidgetRepo,
	)
	alertSvc := service.NewAlertService(
		repository.AlertRepo,
		repository.DashboardRepo,
		repository.WidgetRepo,
		asynqClient,
		logger,
	)
	aiSvc := service.NewAIService(
		repository.AIRepo,
		repository.ConnectionRepo,
		repository.SemanticRepo,
		cryptoHelper,
		cfg,
		logger,
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
		AlertService:  alertSvc,
		AIService:     aiSvc,
		Preparer:      preparer,
		Engine:        duckdbEngine,
		QueryCache:    queryCache,
		AsynqClient:   asynqClient,
	}
}

// Close releases the in-process analytical engine and asynq client.
func (s Service) Close() error {
	if s.AsynqClient != nil {
		_ = s.AsynqClient.Close()
	}
	if s.Engine == nil {
		return nil
	}
	return s.Engine.Close()
}
