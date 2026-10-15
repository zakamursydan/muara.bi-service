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
}

func NewService(
	repository Repository,
	internalConnection InternalConnection,
	cfg *config.Config,
	logger *logging.Logger,
) Service {
	// Crypto helper for master key encryption
	cryptoHelper, err := cryptohelper.NewCryptoHelper(cfg.AppEncryptionMasterKey)
	if err != nil {
		logger.Fatal("failed to initialize crypto helper: " + err.Error())
	}

	// SSH tunnel pool
	sshPool := ssh.NewTunnelPool()

	// DuckDB Analytical Engine
	duckdbEngine, err := duckdb.NewEngine(duckdb.DefaultGuardrailConfig())
	if err != nil {
		logger.Fatal("failed to initialize duckdb engine: " + err.Error())
	}

	// Smart query cache
	queryCache := cache.NewQueryCache(internalConnection.Redis, 300*time.Second)

	// Services
	authSvc := service.NewAuthService(repository.UserRepo, cfg, logger)
	connSvc := service.NewConnectionService(repository.ConnectionRepo, cryptoHelper, sshPool, logger)
	semanticSvc := service.NewSemanticService(repository.SemanticRepo)
	querySvc := service.NewQueryService(duckdbEngine, repository.ConnectionRepo, repository.SemanticRepo, connSvc, queryCache, logger)

	return Service{
		AuthService:       authSvc,
		ConnectionService: connSvc,
		QueryService:      querySvc,
		SemanticService:   semanticSvc,
	}
}
