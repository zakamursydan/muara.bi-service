package main

import (
	"context"
	"os"
	"os/signal"
	"syscall"

	"muara.bi/core/internal/app/server"
	"muara.bi/core/internal/config"
	"muara.bi/core/internal/worker"
	"muara.bi/core/pkg/logging"
)

func main() {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	cfg, err := config.GetConfig()
	if err != nil {
		panic("failed to load configuration: " + err.Error())
	}
	logger := logging.CreateDefaultLogger(cfg.GetLogLevel().String())

	logger.Info("initializing muara.bi background worker pool...")

	conn, err := server.NewInternalConnection(ctx, logger, cfg)
	if err != nil {
		logger.Fatal("failed to connect to internal databases: " + err.Error())
	}
	defer conn.Close()

	// Reuse the API wiring so background jobs share identical repositories,
	// crypto, SSH pool and analytical engine configuration.
	repo := server.NewRepository(conn)
	svc := server.NewService(repo, conn, cfg, logger)
	defer func() {
		if err := svc.Close(); err != nil {
			logger.Error("failed to close analytical engine: " + err.Error())
		}
	}()

	workerServer := worker.NewWorkerServer(
		cfg,
		conn.DB,
		repo.DashboardRepo,
		repo.WidgetRepo,
		svc.Preparer,
		svc.AccessService,
		svc.QueryCache,
		logger,
	)

	// Start active alert & digest cron scheduler
	scheduler := worker.NewScheduler(repo.AlertRepo, repo.DashboardRepo, repo.WidgetRepo, svc.AsynqClient, logger)
	scheduler.Start(ctx)

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)

	go func() {
		sig := <-sigChan
		logger.Info("received termination signal: " + sig.String())
		workerServer.Shutdown()
		cancel()
	}()

	if err := workerServer.Run(); err != nil {
		logger.Fatal("worker server exited with error: " + err.Error())
	}

	<-ctx.Done()
	logger.Info("worker server shutdown completed")
}
