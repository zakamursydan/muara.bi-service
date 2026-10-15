package server

import (
	"context"
	"fmt"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"muara.bi/core/internal/config"
	"muara.bi/core/internal/handler"
	middlewarePkg "muara.bi/core/internal/middleware"
	"muara.bi/core/pkg/logging"
)

type Server struct {
	srv    *http.Server
	router chi.Router
	cfg    *config.Config
	logger *logging.Logger

	InternalConnection *InternalConnection
}

func NewAppServer(cfg *config.Config, logger *logging.Logger) *Server {
	router := chi.NewMux()

	// Global Middlewares
	router.Use(middleware.RequestID)
	router.Use(middleware.RealIP)
	router.Use(middleware.Logger)
	router.Use(middleware.Recoverer)

	srv := &http.Server{
		Addr:    fmt.Sprintf(":%s", cfg.AppHTTPPort),
		Handler: router,
	}

	return &Server{
		cfg:    cfg,
		router: router,
		srv:    srv,
		logger: logger,
	}
}

func (s *Server) BeforeStart(ctx context.Context) error {
	internalClient, err := NewInternalConnection(ctx, s.logger, s.cfg)
	if err != nil {
		return err
	}

	repository := NewRepository(internalClient)
	svc := NewService(repository, internalClient, s.cfg, s.logger)

	// Health check endpoint
	s.router.Get("/health", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"status":"ok","app":"muara.bi"}`))
	})

	// Public Handlers (Auth)
	authHandler := handler.NewAuthHandler(svc.AuthService, s.cfg.AppSecretKey, s.logger)
	authHandler.RegisterRoutes(s.router)

	// Protected Routes (JWT Required)
	connHandler := handler.NewConnectionHandler(svc.ConnectionService)
	queryHandler := handler.NewQueryHandler(svc.QueryService)
	semanticHandler := handler.NewSemanticHandler(svc.SemanticService)

	s.router.Group(func(protected chi.Router) {
		protected.Use(middlewarePkg.JWTMiddleware(s.cfg.AppSecretKey))
		connHandler.RegisterRoutes(protected)
		queryHandler.RegisterRoutes(protected)
		semanticHandler.RegisterRoutes(protected)
	})

	s.InternalConnection = &internalClient
	return nil
}

func (s *Server) AfterStart(ctx context.Context) error {
	s.logger.Info("AppServer AfterStart completed.")
	return nil
}

func (s *Server) Shutdown(ctx context.Context) error {
	if s.InternalConnection != nil {
		if err := s.InternalConnection.Close(); err != nil {
			s.logger.Error(fmt.Sprintf("Failed to close DB/Redis connection: %s", err.Error()))
			return err
		}
		s.logger.Info("DB and Redis connections closed.")
	}
	return nil
}

func (s *Server) HTTPServer() *http.Server {
	return s.srv
}
