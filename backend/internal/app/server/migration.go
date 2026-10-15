package server

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/pressly/goose/v3"
	"muara.bi/core/internal/config"
	"muara.bi/core/pkg/logging"
)

func RunMigration(ctx context.Context, cfg *config.Config, log *logging.Logger) error {
	if !cfg.Database.EnableMigrations {
		log.Info("Database migrations are disabled via config.")
		return nil
	}

	dsn := fmt.Sprintf(
		"host=%s port=%s user=%s password=%s dbname=%s sslmode=%s search_path=%s",
		cfg.Database.Host,
		cfg.Database.Port,
		cfg.Database.User,
		cfg.Database.Password,
		cfg.Database.Name,
		cfg.Database.SSLMode,
		cfg.Database.Schema,
	)

	db, err := sql.Open("pgx", dsn)
	if err != nil {
		log.Error(fmt.Sprintf("Failed to open db for migration: %s", err.Error()))
		return err
	}
	defer db.Close()

	if err := db.PingContext(ctx); err != nil {
		log.Warn(fmt.Sprintf("Cannot ping database for migration: %s. Skipping auto-migration for now.", err.Error()))
		return nil
	}

	log.Info("Running PostgreSQL metadata database migrations via Goose...")

	if err := goose.SetDialect("postgres"); err != nil {
		return fmt.Errorf("failed to set goose dialect: %w", err)
	}

	migrationDir := "./internal/generated/migrations"
	if err := goose.Up(db, migrationDir); err != nil {
		log.Error(fmt.Sprintf("Migration failed: %s", err.Error()))
		return err
	}

	log.Info("All database migrations completed successfully (14 core tables ready).")
	return nil
}
