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
	AppPublicBaseURL       string `env:"APP_PUBLIC_BASE_URL" default:"http://localhost:3000"`                  // web origin used to build share links
	// WorkerConcurrency is how many Asynq tasks one worker process runs at once.
	// Tasks that reach DuckDB additionally queue on the shared analytical budget,
	// so this may safely exceed DUCKDB_MAX_CONCURRENCY.
	WorkerConcurrency int `env:"WORKER_CONCURRENCY" default:"4"`
	Database          DatabaseConfig
	Redis             RedisConfig
	DuckDB            DuckDBConfig
	Otel              telemetry.OtelConfig
}

// DuckDBConfig tunes the analytical engine. The extension directory points at
// pre-bundled .duckdb_extension files so a container with no network egress can
// still ATTACH external databases.
type DuckDBConfig struct {
	ExtensionDirectory string `env:"DUCKDB_EXTENSION_DIRECTORY" default:""`

	// MaxMemory is the DuckDB memory budget of a single session, e.g. "2GiB".
	// It must carry a unit and must be greater than zero; the engine rejects
	// either mistake at startup.
	MaxMemory string `env:"DUCKDB_MAX_MEMORY" default:"2GiB"`

	// MaxConcurrentExecutions is how many DuckDB sessions may run at once across
	// every API and worker process sharing this Redis. A session may grow to
	// DUCKDB_MAX_MEMORY before it spills to its temp directory, so size the two
	// together rather than assuming the ceiling is committed RAM.
	MaxConcurrentExecutions int `env:"DUCKDB_MAX_CONCURRENCY" default:"2"`
	// SlotLeaseSeconds is how long an abandoned slot is tolerated before it is
	// reclaimed. It must exceed the longest guarded execution, otherwise a slow but
	// live query loses its budget to a newcomer.
	SlotLeaseSeconds int `env:"DUCKDB_SLOT_LEASE_SECONDS" default:"120"`
	// QueueTimeoutSeconds is how long a caller waits for a free slot before it
	// gets a 429.
	QueueTimeoutSeconds int `env:"DUCKDB_QUEUE_TIMEOUT_SECONDS" default:"30"`
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
