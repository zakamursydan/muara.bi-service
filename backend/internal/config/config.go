package config

import (
	"strings"

	"muara.bi/core/pkg/envparser"
	"muara.bi/core/pkg/logging"
	"muara.bi/core/pkg/telemetry"
)

type Config struct {
	LogLevel               string `env:"LOG_LEVEL" default:"info"`
	AppHTTPPort            string `env:"APP_HTTP_PORT" default:"8080"`
	AppSecretKey           string `env:"APP_SECRET_KEY" default:"muara-bi-super-secure-jwt-signing-secret-key-32b"`
	AppEncryptionMasterKey string `env:"APP_ENCRYPTION_MASTER_KEY" default:"0123456789abcdef0123456789abcdef"` // 32 bytes for AES-256-GCM
	Database               DatabaseConfig
	Redis                  RedisConfig
	Otel                   telemetry.OtelConfig
}

type DatabaseConfig struct {
	Host             string `env:"DB_HOST" default:"127.0.0.1"`
	Port             string `env:"DB_PORT" default:"5432"`
	User             string `env:"DB_USERNAME" default:"postgres"`
	Password         string `env:"DB_PASSWORD" default:"postgres"`
	Name             string `env:"DB_NAME" default:"muara_bi_db"`
	Schema           string `env:"DB_SCHEMA" default:"public"`
	SSLMode          string `env:"DB_SSL_MODE" default:"disable"`
	MaxIdleConns     int    `env:"DB_MAX_IDLE_CONNS" default:"5"`
	MaxOpenConns     int    `env:"DB_MAX_OPEN_CONNS" default:"20"`
	EnableMigrations bool   `env:"ENABLE_MIGRATIONS" default:"true"`
}

type RedisConfig struct {
	Host     string `env:"REDIS_HOST" default:"127.0.0.1"`
	Port     string `env:"REDIS_PORT" default:"6379"`
	Password string `env:"REDIS_PASSWORD" default:""`
	DB       int    `env:"REDIS_DB" default:"0"`
}

func (c *Config) GetLogLevel() logging.Level {
	switch strings.ToLower(c.LogLevel) {
	case "debug":
		return logging.LevelDebug
	case "info":
		return logging.LevelInfo
	case "warn":
		return logging.LevelWarn
	case "error":
		return logging.LevelError
	case "fatal":
		return logging.LevelFatal
	default:
		return logging.LevelInfo
	}
}

func GetConfig() (*Config, error) {
	cfg := Config{}
	err := envparser.LoadEnv(&cfg)
	if err != nil {
		return nil, err
	}
	return &cfg, nil
}
