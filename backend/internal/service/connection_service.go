package service

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"net"
	"time"

	"muara.bi/core/internal/domain/entity"
	"muara.bi/core/internal/dto/request"
	"muara.bi/core/internal/dto/response"
	"muara.bi/core/internal/helper/cryptohelper"
	"muara.bi/core/internal/repository/pgsql"
	"muara.bi/core/internal/ssh"
	"muara.bi/core/pkg/logging"
)

type ConnectionServiceProvider interface {
	CreateConnection(ctx context.Context, wsID string, req request.CreateConnectionRequest) (*response.ConnectionResponse, error)
	ListConnections(ctx context.Context, wsID string) ([]response.ConnectionResponse, error)
	UpsertEnvironment(ctx context.Context, wsID, connID string, req request.UpsertEnvironmentRequest) (*response.EnvironmentInfoResponse, error)
	TestConnection(ctx context.Context, wsID, connID, envTag string) (*response.TestConnectionResponse, error)
	SyncSchema(ctx context.Context, wsID, connID, envTag string) error
	GetDecryptedConfig(ctx context.Context, wsID, connID, envTag string) (*entity.ConnectionConfig, *entity.DataConnection, error)
}

type ConnectionService struct {
	connRepo pgsql.ConnectionRepositoryProvider
	crypto   *cryptohelper.CryptoHelper
	sshPool  *ssh.TunnelPool
	logger   *logging.Logger
}

func NewConnectionService(
	connRepo pgsql.ConnectionRepositoryProvider,
	crypto *cryptohelper.CryptoHelper,
	sshPool *ssh.TunnelPool,
	logger *logging.Logger,
) *ConnectionService {
	return &ConnectionService{
		connRepo: connRepo,
		crypto:   crypto,
		sshPool:  sshPool,
		logger:   logger,
	}
}

func (s *ConnectionService) CreateConnection(ctx context.Context, wsID string, req request.CreateConnectionRequest) (*response.ConnectionResponse, error) {
	if req.AliasName == "" {
		return nil, errors.New("alias_name is required")
	}
	if req.DriverType == "" {
		req.DriverType = "postgres"
	}
	if req.Category == "" {
		req.Category = "SQL"
	}

	conn := &entity.DataConnection{
		WorkspaceID: wsID,
		AliasName:   req.AliasName,
		Category:    req.Category,
		DriverType:  req.DriverType,
	}

	if err := s.connRepo.CreateConnection(ctx, conn); err != nil {
		return nil, fmt.Errorf("failed to create connection: %w", err)
	}

	return &response.ConnectionResponse{
		ID:          conn.ID,
		WorkspaceID: conn.WorkspaceID,
		AliasName:   conn.AliasName,
		Category:    conn.Category,
		DriverType:  conn.DriverType,
		CreatedAt:   conn.CreatedAt,
	}, nil
}

func (s *ConnectionService) ListConnections(ctx context.Context, wsID string) ([]response.ConnectionResponse, error) {
	conns, err := s.connRepo.ListConnections(ctx, wsID)
	if err != nil {
		return nil, err
	}

	var results []response.ConnectionResponse
	for _, c := range conns {
		envs, err := s.connRepo.ListEnvironments(ctx, c.ID)
		if err != nil {
			return nil, err
		}

		var envResponses []response.EnvironmentInfoResponse
		for _, e := range envs {
			// Decrypt config safely to surface non-sensitive host/port info
			var cfg entity.ConnectionConfig
			_ = s.crypto.DecryptJSON(e.EncryptedConfig, &cfg)

			sshEnabled := false
			if cfg.SSHTunnel != nil && cfg.SSHTunnel.Enabled {
				sshEnabled = true
			}

			envResponses = append(envResponses, response.EnvironmentInfoResponse{
				ID:               e.ID,
				EnvTag:           e.EnvTag,
				DisplayName:      e.DisplayName,
				BadgeColor:       e.BadgeColor,
				IsDefault:        e.IsDefault,
				SSHTunnelEnabled: sshEnabled,
				Host:             cfg.Host,
				Port:             cfg.Port,
				Database:         cfg.Database,
				User:             cfg.User,
				CachedSchema:     e.CachedSchema,
				LastSyncedAt:     e.LastSyncedAt,
			})
		}

		results = append(results, response.ConnectionResponse{
			ID:           c.ID,
			WorkspaceID:  c.WorkspaceID,
			AliasName:    c.AliasName,
			Category:     c.Category,
			DriverType:   c.DriverType,
			Environments: envResponses,
			CreatedAt:    c.CreatedAt,
		})
	}

	return results, nil
}

func (s *ConnectionService) UpsertEnvironment(ctx context.Context, wsID, connID string, req request.UpsertEnvironmentRequest) (*response.EnvironmentInfoResponse, error) {
	conn, err := s.connRepo.GetConnectionByID(ctx, wsID, connID)
	if err != nil || conn == nil {
		return nil, errors.New("connection not found in this workspace")
	}

	if req.EnvTag == "" {
		return nil, errors.New("env_tag is required (e.g. production, staging, sandbox)")
	}

	// Prepare credential payload
	cfgPayload := entity.ConnectionConfig{
		Host:     req.Host,
		Port:     req.Port,
		Database: req.Database,
		User:     req.User,
		Password: req.Password,
		SSLMode:  req.SSLMode,
		URI:      req.URI,
	}

	if req.SSHTunnelEnabled {
		cfgPayload.SSHTunnel = &entity.SSHConfigJSON{
			Enabled:    true,
			Host:       req.SSHHost,
			Port:       req.SSHPort,
			User:       req.SSHUser,
			Password:   req.SSHPassword,
			PrivateKey: req.SSHPrivateKey,
		}
	}

	// Encrypt using master AES-256-GCM key
	encryptedBytes, err := s.crypto.EncryptJSON(cfgPayload)
	if err != nil {
		return nil, fmt.Errorf("failed to encrypt connection credentials: %w", err)
	}

	env := &entity.ConnectionEnvironment{
		ConnectionID:    connID,
		EnvTag:          req.EnvTag,
		DisplayName:     req.DisplayName,
		BadgeColor:      req.BadgeColor,
		EncryptedConfig: encryptedBytes,
		IsDefault:       req.IsDefault,
	}

	if err := s.connRepo.UpsertEnvironment(ctx, env); err != nil {
		return nil, fmt.Errorf("failed to upsert connection environment: %w", err)
	}

	return &response.EnvironmentInfoResponse{
		ID:               env.ID,
		EnvTag:           env.EnvTag,
		DisplayName:      env.DisplayName,
		BadgeColor:       env.BadgeColor,
		IsDefault:        env.IsDefault,
		SSHTunnelEnabled: req.SSHTunnelEnabled,
		Host:             req.Host,
		Port:             req.Port,
		Database:         req.Database,
		User:             req.User,
	}, nil
}

func (s *ConnectionService) TestConnection(ctx context.Context, wsID, connID, envTag string) (*response.TestConnectionResponse, error) {
	cfg, _, err := s.GetDecryptedConfig(ctx, wsID, connID, envTag)
	if err != nil {
		return &response.TestConnectionResponse{Success: false, Message: err.Error()}, nil
	}

	start := time.Now()

	// Check if SSH tunnel is requested
	if cfg.SSHTunnel != nil && cfg.SSHTunnel.Enabled {
		bastionCfg := ssh.BastionConfig{
			Host:       cfg.SSHTunnel.Host,
			Port:       cfg.SSHTunnel.Port,
			User:       cfg.SSHTunnel.User,
			Password:   cfg.SSHTunnel.Password,
			PrivateKey: cfg.SSHTunnel.PrivateKey,
			Timeout:    5,
		}

		conn, err := s.sshPool.DialContext(ctx, bastionCfg, cfg.Host, cfg.Port)
		if err != nil {
			return &response.TestConnectionResponse{
				Success:   false,
				Message:   fmt.Sprintf("SSH tunnel dial failed: %v", err),
				LatencyMs: time.Since(start).Milliseconds(),
			}, nil
		}
		_ = conn.Close()
	} else {
		// Direct TCP dial test
		targetAddr := fmt.Sprintf("%s:%d", cfg.Host, cfg.Port)
		d := net.Dialer{Timeout: 5 * time.Second}
		conn, err := d.DialContext(ctx, "tcp", targetAddr)
		if err != nil {
			return &response.TestConnectionResponse{
				Success:   false,
				Message:   fmt.Sprintf("Direct connection failed: %v", err),
				LatencyMs: time.Since(start).Milliseconds(),
			}, nil
		}
		_ = conn.Close()
	}

	return &response.TestConnectionResponse{
		Success:   true,
		Message:   "Connection successful",
		LatencyMs: time.Since(start).Milliseconds(),
	}, nil
}

func (s *ConnectionService) SyncSchema(ctx context.Context, wsID, connID, envTag string) error {
	cfg, conn, err := s.GetDecryptedConfig(ctx, wsID, connID, envTag)
	if err != nil {
		return err
	}

	s.logger.Info(fmt.Sprintf("syncing schema for conn=%s alias=%s env=%s", connID, conn.AliasName, envTag))

	// For Postgres: query information_schema.columns
	if conn.DriverType == "postgres" {
		dsn := fmt.Sprintf("host=%s port=%d user=%s password=%s dbname=%s sslmode=%s",
			cfg.Host, cfg.Port, cfg.User, cfg.Password, cfg.Database, cfg.SSLMode)
		if cfg.SSLMode == "" {
			dsn += " sslmode=disable"
		}

		targetDB, err := sql.Open("pgx", dsn)
		if err != nil {
			return fmt.Errorf("failed to connect to source db for schema discovery: %w", err)
		}
		defer targetDB.Close()

		rows, err := targetDB.QueryContext(ctx, `
			SELECT table_name, column_name, data_type 
			FROM information_schema.columns 
			WHERE table_schema NOT IN ('information_schema', 'pg_catalog')
			ORDER BY table_name, ordinal_position`)
		if err != nil {
			return fmt.Errorf("schema query failed: %w", err)
		}
		defer rows.Close()

		type ColumnDef struct {
			Name string `json:"name"`
			Type string `json:"type"`
		}
		tableMap := make(map[string][]ColumnDef)
		for rows.Next() {
			var tbl, col, dt string
			if err := rows.Scan(&tbl, &col, &dt); err == nil {
				tableMap[tbl] = append(tableMap[tbl], ColumnDef{Name: col, Type: dt})
			}
		}

		schemaJSON, _ := s.crypto.EncryptJSON(tableMap) // reuse json helper
		_ = s.connRepo.UpdateCachedSchema(ctx, connID, envTag, schemaJSON)
	}

	return nil
}

func (s *ConnectionService) GetDecryptedConfig(ctx context.Context, wsID, connID, envTag string) (*entity.ConnectionConfig, *entity.DataConnection, error) {
	conn, err := s.connRepo.GetConnectionByID(ctx, wsID, connID)
	if err != nil || conn == nil {
		return nil, nil, errors.New("connection not found")
	}

	env, err := s.connRepo.GetEnvironment(ctx, connID, envTag)
	if err != nil || env == nil {
		return nil, nil, fmt.Errorf("environment profile %q not found for this connection", envTag)
	}

	var cfg entity.ConnectionConfig
	if err := s.crypto.DecryptJSON(env.EncryptedConfig, &cfg); err != nil {
		return nil, nil, fmt.Errorf("failed to decrypt connection configuration: %w", err)
	}

	return &cfg, conn, nil
}
