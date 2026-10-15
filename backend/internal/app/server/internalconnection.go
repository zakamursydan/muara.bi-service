package server

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/redis/go-redis/v9"
	"muara.bi/core/internal/config"
	"muara.bi/core/pkg/logging"
)

type InternalConnection struct {
	DB    *sql.DB
	Redis *redis.Client
}

func NewInternalConnection(ctx context.Context, log *logging.Logger, cfg *config.Config) (InternalConnection, error) {
	db, err := NewDbConnection(ctx, log, &cfg.Database)
	if err != nil {
		return InternalConnection{}, err
	}

	redisClient := NewRedisConnection(ctx, log, &cfg.Redis)

	return InternalConnection{
		DB:    db,
		Redis: redisClient,
	}, nil
}

func NewDbConnection(ctx context.Context, log *logging.Logger, cfg *config.DatabaseConfig) (*sql.DB, error) {
	dsn := fmt.Sprintf(
		"host=%s port=%s user=%s password=%s dbname=%s sslmode=%s search_path=%s",
		cfg.Host,
		cfg.Port,
		cfg.User,
		cfg.Password,
		cfg.Name,
		cfg.SSLMode,
		cfg.Schema,
	)

	db, err := sql.Open("pgx", dsn)
	if err != nil {
		log.Error(fmt.Sprintf("Failed to open database connection: %s", err.Error()))
		return nil, err
	}

	db.SetConnMaxLifetime(5 * time.Minute)
	db.SetMaxIdleConns(cfg.MaxIdleConns)
	db.SetMaxOpenConns(cfg.MaxOpenConns)

	if err := db.PingContext(ctx); err != nil {
		log.Warn(fmt.Sprintf("Database ping failed (check connection/credentials): %s", err.Error()))
		// We return db even if ping fails so migrations or retry loops can handle it gracefully
	} else {
		log.Info(fmt.Sprintf("Connected successfully to PostgreSQL metadata database: %s:%s/%s", cfg.Host, cfg.Port, cfg.Name))
	}

	return db, nil
}

func NewRedisConnection(ctx context.Context, log *logging.Logger, cfg *config.RedisConfig) *redis.Client {
	rdb := redis.NewClient(&redis.Options{
		Addr:     fmt.Sprintf("%s:%s", cfg.Host, cfg.Port),
		Password: cfg.Password,
		DB:       cfg.DB,
	})

	if err := rdb.Ping(ctx).Err(); err != nil {
		log.Warn(fmt.Sprintf("Redis ping failed (caching/queue will reconnect when available): %s", err.Error()))
	} else {
		log.Info(fmt.Sprintf("Connected successfully to Redis instance: %s:%s", cfg.Host, cfg.Port))
	}

	return rdb
}

func (ic *InternalConnection) Close() error {
	var errs []error
	if ic.DB != nil {
		if err := ic.DB.Close(); err != nil {
			errs = append(errs, err)
		}
	}
	if ic.Redis != nil {
		if err := ic.Redis.Close(); err != nil {
			errs = append(errs, err)
		}
	}
	if len(errs) > 0 {
		return fmt.Errorf("errors closing internal connections: %v", errs)
	}
	return nil
}
