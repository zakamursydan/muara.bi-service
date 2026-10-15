package main

import (
	"context"
	"os"
	"os/signal"
	"syscall"
	"time"

	"muara.bi/core/internal/app/server"
	"muara.bi/core/internal/cache"
	"muara.bi/core/internal/config"
	"muara.bi/core/internal/duckdb"
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

	// Connect to PostgreSQL and Redis
	conn, err := server.NewInternalConnection(ctx, logger, cfg)
	if err != nil {
		logger.Fatal("failed to connect to internal databases: " + err.Error())
	}
	defer func() {
		if conn.DB != nil {
			_ = conn.DB.Close()
		}
		if conn.Redis != nil {
			_ = conn.Redis.Close()
		}
	}()

	// Analytical engine
	engine, err := duckdb.NewEngine(duckdb.DefaultGuardrailConfig())
	if err != nil {
		logger.Fatal("failed to initialize duckdb engine: " + err.Error())
	}
	defer engine.Close()

	// Query cache
	queryCache := cache.NewQueryCache(conn.Redis, 300*time.Second)

	// Create worker server
	workerServer := worker.NewWorkerServer(cfg, conn.DB, engine, queryCache, logger)

	// Signal handling for graceful shutdown

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
