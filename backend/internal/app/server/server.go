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
	service            Service
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
	s.service = svc

	// Health check endpoint
	s.router.Get("/health", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"status":"ok","app":"muara.bi"}`))
	})

	// Public Handlers (Auth)
	authHandler := handler.NewAuthHandler(svc.AuthService, s.cfg.AppSecretKey, s.logger)
	authHandler.RegisterRoutes(s.router)

	// Tenant-scoped routes (JWT required). A single subrouter owns the
	// /api/v1/workspaces/{wsId} prefix so that handler routes registered by
	// different domains can never collide on the same chi mount point.
	guard := middlewarePkg.NewGuard(svc.AccessService)
	s.router.Route("/api/v1/workspaces/{wsId}", func(ws chi.Router) {
		ws.Use(middlewarePkg.JWTMiddleware(s.cfg.AppSecretKey))

		handler.NewConnectionHandler(svc.ConnectionService, guard).RegisterRoutes(ws)
		handler.NewQueryHandler(svc.QueryService, guard).RegisterRoutes(ws)
		handler.NewSemanticHandler(svc.SemanticService, guard).RegisterRoutes(ws)
		handler.NewProjectHandler(svc.ProjectService, guard).RegisterRoutes(ws)
		handler.NewDashboardHandler(svc.DashboardService, guard).RegisterRoutes(ws)
		handler.NewWidgetHandler(svc.WidgetService, guard).RegisterRoutes(ws)
		handler.NewShareHandler(svc.ShareService, guard).RegisterRoutes(ws)
		handler.NewAlertHandler(svc.AlertService, guard).RegisterRoutes(ws)
		handler.NewAIHandler(svc.AIService, guard).RegisterRoutes(ws)
	})

	// Public share surface. No JWT: the share token (or a signed embed token) is
	// the credential, and the routes accept no SQL, env tag or tenant identifier.
	s.router.Route("/api/v1/public", func(pub chi.Router) {
		handler.NewPublicHandler(svc.ShareService, s.cfg.AppPublicBaseURL).RegisterRoutes(pub)
	})

	s.InternalConnection = &internalClient
	return nil
}

func (s *Server) AfterStart(ctx context.Context) error {
	s.logger.Info("AppServer AfterStart completed.")
	return nil
}

func (s *Server) Shutdown(ctx context.Context) error {
	if err := s.service.Close(); err != nil {
		s.logger.Error(fmt.Sprintf("Failed to close analytical engine: %s", err.Error()))
	}

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
