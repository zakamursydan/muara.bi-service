package server

import (
	"muara.bi/core/internal/config"
	"muara.bi/core/internal/service"
	"muara.bi/core/pkg/logging"
)

type Service struct {
	AuthService service.AuthServiceProvider
}

func NewService(
	repository Repository,
	cfg *config.Config,
	logger *logging.Logger,
) Service {
	return Service{
		AuthService: service.NewAuthService(repository.UserRepo, cfg, logger),
	}
}
