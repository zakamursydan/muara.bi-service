package main

import (
	"context"
	"fmt"
	"net/http"

	"muara.bi/core/internal/app/server"
	"muara.bi/core/internal/config"
	"muara.bi/core/pkg/container"
	"muara.bi/core/pkg/logging"
	"muara.bi/core/pkg/telemetry"
)

func main() {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	cfg, err := config.GetConfig()
	if err != nil {
		panic(fmt.Sprintf("Failed to load configuration: %s", err.Error()))
	}

	logger := logging.CreateDefaultLogger(cfg.GetLogLevel().String())

	// Initialize OpenTelemetry
	shutdownOTel, err := telemetry.InitProvider(ctx, &cfg.Otel)
	if err != nil {
		logger.Error(fmt.Sprintf("Failed to initialize OpenTelemetry: %s", err.Error()))
	} else {
		defer func() {
			if err := shutdownOTel(context.Background()); err != nil {
				logger.Error(fmt.Sprintf("Failed to shutdown OpenTelemetry: %s", err.Error()))
			}
		}()
	}

	// Run Database Migrations
	err = server.RunMigration(ctx, cfg, logger)
	if err != nil {
		logger.Warn(fmt.Sprintf("Database migration notice: %s", err.Error()))
	}

	logger.Info(fmt.Sprintf("[API] Starting muara.bi core API server at port :%s...", cfg.AppHTTPPort))
	apiServer := server.NewAppServer(cfg, logger)
	servers := []*http.Server{apiServer.HTTPServer()}

	appContainer := container.New(
		apiServer,
		container.WithHTTPServer(servers),
	)

	err = appContainer.Start(ctx)
	if err != nil {
		logger.Error(fmt.Sprintf("Server error: %s", err.Error()))
		cancel()
	} else {
		logger.Info("[API] muara.bi API server shut down gracefully.")
	}
}
