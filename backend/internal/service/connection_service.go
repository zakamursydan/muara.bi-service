package service

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/url"
	"strings"
	"time"

	"muara.bi/core/internal/domain/entity"
	"muara.bi/core/internal/dto/request"
	"muara.bi/core/internal/dto/response"
	"muara.bi/core/internal/repository/pgsql"
	"muara.bi/core/pkg/customerror"
	"muara.bi/core/pkg/logging"
	"muara.bi/engine/cryptohelper"
	"muara.bi/engine/duckdb"
	"muara.bi/engine/sqlident"
	"muara.bi/engine/ssh"
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
	// engine reads the catalogue of sources this build has no Go driver for, so
	// a schema cache describes exactly what a query session can mount.
	engine duckdb.Engine
	logger *logging.Logger
}

func NewConnectionService(
	connRepo pgsql.ConnectionRepositoryProvider,
	crypto *cryptohelper.CryptoHelper,
	sshPool *ssh.TunnelPool,
	engine duckdb.Engine,
	logger *logging.Logger,
) *ConnectionService {
	return &ConnectionService{
		connRepo: connRepo,
		crypto:   crypto,
		sshPool:  sshPool,
		engine:   engine,
		logger:   logger,
	}
}

func (s *ConnectionService) CreateConnection(ctx context.Context, wsID string, req request.CreateConnectionRequest) (*response.ConnectionResponse, error) {
	if req.AliasName == "" {
		return nil, customerror.NewBadRequestError("alias_name is required", nil)
	}
	// The alias becomes the DuckDB catalog name, so it has to survive both the
	// ATTACH statement and an unquoted `alias.table` reference in user SQL.
	if !sqlident.IsPlain(req.AliasName) {
		return nil, customerror.NewBadRequestError(
			fmt.Sprintf("alias_name %q must be a plain SQL identifier: letters, digits and underscores, starting with a letter or underscore", req.AliasName),
			nil,
		)
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

	results := make([]response.ConnectionResponse, 0)
	for _, c := range conns {
		envs, err := s.connRepo.ListEnvironments(ctx, c.ID)
		if err != nil {
			return nil, err
		}

		// Initialised as a slice so an unconfigured connection serialises as [] rather than null.
		envResponses := make([]response.EnvironmentInfoResponse, 0)
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
				SSLMode:          cfg.SSLMode,
				RootCert:         cfg.RootCert,
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

	// Refuse an unrecognised ssl_mode here: it would otherwise be encrypted and
	// stored, then surface much later as a TLS failure during connect or ATTACH.
	sslMode, err := normalizeSSLMode(req.SSLMode)
	if err != nil {
		return nil, customerror.NewBadRequestError(err.Error(), err)
	}
	req.SSLMode = sslMode

	// A CA path is checked against the connector and the mode that would read it,
	// at the same point: an unusable combination saved here would only appear
	// later as a TLS error at ATTACH time, in a different process, with no hint
	// that the profile itself was the problem.
	rootCert, err := normalizeStoredRootCert(conn.DriverType, req.RootCert, sslMode)
	if err != nil {
		return nil, customerror.NewBadRequestError(err.Error(), err)
	}
	req.RootCert = rootCert

	// Prepare credential payload
	cfgPayload := entity.ConnectionConfig{
		Host:     req.Host,
		Port:     req.Port,
		Database: req.Database,
		User:     req.User,
		Password: req.Password,
		SSLMode:  req.SSLMode,
		RootCert: req.RootCert,
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
		SSLMode:          req.SSLMode,
		RootCert:         req.RootCert,
	}, nil
}

func (s *ConnectionService) TestConnection(ctx context.Context, wsID, connID, envTag string) (*response.TestConnectionResponse, error) {
	cfg, conn, err := s.GetDecryptedConfig(ctx, wsID, connID, envTag)
	if err != nil {
		return &response.TestConnectionResponse{Success: false, Message: err.Error()}, nil
	}

	start := time.Now()

	// A MongoDB source keeps its target in a connection string, or in the host
	// fields when the form filled those instead. Either way the generic
	// host:port dial below would dial ":0", and a TCP connect would also pass
	// for a server that then rejects the credentials — so ping with the driver.
	if conn.DriverType == "mongodb" {
		uri, err := mongoURI(cfg)
		if err != nil {
			return &response.TestConnectionResponse{
				Success:   false,
				Message:   redactCredentials(err.Error(), cfg),
				LatencyMs: time.Since(start).Milliseconds(),
			}, nil
		}
		if uri == "" {
			return &response.TestConnectionResponse{
				Success:   false,
				Message:   "mongodb needs either a connection string or a host and port",
				LatencyMs: time.Since(start).Milliseconds(),
			}, nil
		}
		if err := duckdb.PingMongo(ctx, uri, 5); err != nil {
			return &response.TestConnectionResponse{
				Success:   false,
				Message:   redactCredentials(err.Error(), cfg),
				LatencyMs: time.Since(start).Milliseconds(),
			}, nil
		}
		return &response.TestConnectionResponse{
			Success:   true,
			Message:   "MongoDB ping succeeded",
			LatencyMs: time.Since(start).Milliseconds(),
		}, nil
	}

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

// redactCredentials removes the secret half of a connection string from the
// provider text that a connection test returns to the UI. Drivers wrap the URI
// they failed on into the error.
func redactCredentials(msg string, cfg *entity.ConnectionConfig) string {
	if cfg.Password != "" {
		msg = strings.ReplaceAll(msg, cfg.Password, "<redacted>")
	}
	if uri, err := mongoURI(cfg); err == nil && uri != "" {
		parsed, err := url.Parse(uri)
		if err == nil && parsed.User != nil {
			msg = strings.ReplaceAll(msg, parsed.User.String(), "<credentials>")
			if name := parsed.User.Username(); name != "" {
				msg = strings.ReplaceAll(msg, name+"@", "<credentials>@")
			}
		}
	}
	return msg
}

// cachedColumn is one entry of cached_schema. That JSONB column is served verbatim
// to the schema tree, the Monaco completion source and the AI prompt, so this
// shape is the contract every driver has to answer to — including the ones added
// after Postgres.
type cachedColumn struct {
	Name string `json:"name"`
	Type string `json:"type"`
}

func (s *ConnectionService) SyncSchema(ctx context.Context, wsID, connID, envTag string) error {
	cfg, conn, err := s.GetDecryptedConfig(ctx, wsID, connID, envTag)
	if err != nil {
		return err
	}

	s.logger.Info(fmt.Sprintf("syncing schema for conn=%s alias=%s env=%s", connID, conn.AliasName, envTag))

	tables, err := s.discoverSchema(ctx, conn, cfg)
	if err != nil {
		// Attaching a source makes its credentials part of the statement, and the
		// scanner echoes that statement back in its error. AppError.Error() appends
		// its cause, so both halves have to be the redacted text — keeping the raw
		// error as the cause would put the password in every log line that prints it.
		msg := redactCredentials(err.Error(), cfg)
		return customerror.NewBadRequestError(msg, errors.New(msg))
	}
	if len(tables) == 0 {
		// Storing an empty map would wipe a cache that works and then answer the AI
		// prompt with "this source has no tables", so seeing nothing is a failure.
		return customerror.NewBadRequestError(fmt.Sprintf(
			"source %s exposes no queryable tables in environment %q", conn.AliasName, envTag), nil)
	}

	// cached_schema is a JSONB column and the API serves it verbatim to the
	// builder and autocomplete, so it is stored as plain metadata, never encrypted.
	schemaJSON, err := json.Marshal(tables)
	if err != nil {
		return fmt.Errorf("failed to encode discovered schema: %w", err)
	}
	if err := s.connRepo.UpdateCachedSchema(ctx, connID, envTag, schemaJSON); err != nil {
		return fmt.Errorf("failed to persist cached schema: %w", err)
	}

	return nil
}

// discoverSchema reads the table and column metadata of one source. Each branch
// uses the reader that will run the eventual query: Postgres through its own Go
// driver, MySQL and SQLite through the DuckDB scanner because this build carries
// no driver for them, MongoDB not at all.
func (s *ConnectionService) discoverSchema(ctx context.Context, conn *entity.DataConnection, cfg *entity.ConnectionConfig) (map[string][]cachedColumn, error) {
	switch conn.DriverType {
	case "postgres":
		return s.discoverPostgres(ctx, cfg)
	case "mysql", "sqlite":
		return s.discoverThroughEngine(ctx, conn, cfg)
	case "mongodb":
		// A schemaless source has no catalogue to read: its fields only become
		// visible by streaming documents, and putting that into a cache that is
		// handed to the AI prompt would move source data into metadata. Queries
		// still work because the collections are mounted per execution, under the
		// row cap.
		return nil, fmt.Errorf("mongodb has no schema to sync: collection fields can only be inferred by reading documents, which the metadata-only rule forbids")
	default:
		return nil, fmt.Errorf("no schema discovery for driver %q", conn.DriverType)
	}
}

func (s *ConnectionService) discoverPostgres(ctx context.Context, cfg *entity.ConnectionConfig) (map[string][]cachedColumn, error) {
	dsn, err := postgresDSN(cfg)
	if err != nil {
		return nil, err
	}

	targetDB, err := sql.Open("pgx", dsn)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to source db for schema discovery: %w", err)
	}
	defer targetDB.Close()

	rows, err := targetDB.QueryContext(ctx, `
		SELECT table_name, column_name, data_type 
		FROM information_schema.columns 
		WHERE table_schema NOT IN ('information_schema', 'pg_catalog')
		ORDER BY table_name, ordinal_position`)
	if err != nil {
		return nil, fmt.Errorf("schema query failed: %w", err)
	}
	defer rows.Close()

	tableMap := make(map[string][]cachedColumn)
	for rows.Next() {
		var tbl, col, dt string
		if err := rows.Scan(&tbl, &col, &dt); err == nil {
			tableMap[tbl] = append(tableMap[tbl], cachedColumn{Name: col, Type: dt})
		}
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("schema query failed while reading rows: %w", err)
	}

	return tableMap, nil
}

// discoverThroughEngine attaches the source in a throwaway session and reads the
// catalogue DuckDB built from it. Discovery is metadata only: no row of source
// data is selected, and the session is closed before the caller sees a result.
func (s *ConnectionService) discoverThroughEngine(ctx context.Context, conn *entity.DataConnection, cfg *entity.ConnectionConfig) (map[string][]cachedColumn, error) {
	if s.engine == nil {
		return nil, fmt.Errorf("schema discovery for %s needs the analytical engine, which is unavailable in this build", conn.DriverType)
	}

	session, err := s.engine.NewSession(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to open an analytical session for schema discovery: %w", err)
	}
	defer session.Close()

	if _, err := attachConnection(ctx, session, *conn, cfg); err != nil {
		return nil, err
	}

	tables, err := session.DescribeSource(ctx, conn.AliasName)
	if err != nil {
		return nil, err
	}

	tableMap := make(map[string][]cachedColumn, len(tables))
	for _, table := range tables {
		columns := make([]cachedColumn, 0, len(table.Columns))
		for _, column := range table.Columns {
			columns = append(columns, cachedColumn{Name: column.Name, Type: column.Type})
		}
		tableMap[table.Name] = columns
	}
	return tableMap, nil
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
