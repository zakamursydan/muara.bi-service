package pgsql

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"muara.bi/core/internal/domain/entity"
)

type ConnectionRepositoryProvider interface {
	CreateConnection(ctx context.Context, conn *entity.DataConnection) error
	GetConnectionByID(ctx context.Context, workspaceID, id string) (*entity.DataConnection, error)
	GetConnectionByAlias(ctx context.Context, workspaceID, alias string) (*entity.DataConnection, error)
	ListConnections(ctx context.Context, workspaceID string) ([]entity.DataConnection, error)
	DeleteConnection(ctx context.Context, workspaceID, id string) error

	UpsertEnvironment(ctx context.Context, env *entity.ConnectionEnvironment) error
	GetEnvironment(ctx context.Context, connectionID, envTag string) (*entity.ConnectionEnvironment, error)
	ListEnvironments(ctx context.Context, connectionID string) ([]entity.ConnectionEnvironment, error)
	UpdateCachedSchema(ctx context.Context, connectionID, envTag string, schemaJSON []byte) error
}

type ConnectionRepository struct {
	db *sql.DB
}

func NewConnectionRepository(db *sql.DB) *ConnectionRepository {
	return &ConnectionRepository{db: db}
}

func (r *ConnectionRepository) CreateConnection(ctx context.Context, conn *entity.DataConnection) error {
	query := `
		INSERT INTO data_connections (workspace_id, alias_name, category, driver_type)
		VALUES ($1, $2, $3, $4)
		RETURNING id, created_at`

	return r.db.QueryRowContext(
		ctx, query,
		conn.WorkspaceID, conn.AliasName, conn.Category, conn.DriverType,
	).Scan(&conn.ID, &conn.CreatedAt)
}

func (r *ConnectionRepository) GetConnectionByID(ctx context.Context, workspaceID, id string) (*entity.DataConnection, error) {
	query := `
		SELECT id, workspace_id, alias_name, category, driver_type, created_at
		FROM data_connections
		WHERE workspace_id = $1 AND id = $2`

	var conn entity.DataConnection
	err := r.db.QueryRowContext(ctx, query, workspaceID, id).Scan(
		&conn.ID, &conn.WorkspaceID, &conn.AliasName, &conn.Category, &conn.DriverType, &conn.CreatedAt,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to get connection by id: %w", err)
	}
	return &conn, nil
}

func (r *ConnectionRepository) GetConnectionByAlias(ctx context.Context, workspaceID, alias string) (*entity.DataConnection, error) {
	query := `
		SELECT id, workspace_id, alias_name, category, driver_type, created_at
		FROM data_connections
		WHERE workspace_id = $1 AND alias_name = $2`

	var conn entity.DataConnection
	err := r.db.QueryRowContext(ctx, query, workspaceID, alias).Scan(
		&conn.ID, &conn.WorkspaceID, &conn.AliasName, &conn.Category, &conn.DriverType, &conn.CreatedAt,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to get connection by alias: %w", err)
	}
	return &conn, nil
}

func (r *ConnectionRepository) ListConnections(ctx context.Context, workspaceID string) ([]entity.DataConnection, error) {
	query := `
		SELECT id, workspace_id, alias_name, category, driver_type, created_at
		FROM data_connections
		WHERE workspace_id = $1
		ORDER BY alias_name ASC`

	rows, err := r.db.QueryContext(ctx, query, workspaceID)
	if err != nil {
		return nil, fmt.Errorf("failed to list connections: %w", err)
	}
	defer rows.Close()

	var connections []entity.DataConnection
	for rows.Next() {
		var c entity.DataConnection
		if err := rows.Scan(&c.ID, &c.WorkspaceID, &c.AliasName, &c.Category, &c.DriverType, &c.CreatedAt); err != nil {
			return nil, fmt.Errorf("failed to scan connection: %w", err)
		}
		connections = append(connections, c)
	}

	return connections, nil
}

func (r *ConnectionRepository) DeleteConnection(ctx context.Context, workspaceID, id string) error {
	query := `DELETE FROM data_connections WHERE workspace_id = $1 AND id = $2`
	res, err := r.db.ExecContext(ctx, query, workspaceID, id)
	if err != nil {
		return fmt.Errorf("failed to delete connection: %w", err)
	}
	rowsAffected, _ := res.RowsAffected()
	if rowsAffected == 0 {
		return errors.New("connection not found or unauthorized")
	}
	return nil
}

func (r *ConnectionRepository) UpsertEnvironment(ctx context.Context, env *entity.ConnectionEnvironment) error {
	query := `
		INSERT INTO connection_environments (
			connection_id, env_tag, display_name, badge_color, encrypted_config, is_default
		)
		VALUES ($1, $2, $3, $4, $5, $6)
		ON CONFLICT (connection_id, env_tag) DO UPDATE
		SET display_name = EXCLUDED.display_name,
		    badge_color = EXCLUDED.badge_color,
		    encrypted_config = EXCLUDED.encrypted_config,
		    is_default = EXCLUDED.is_default
		RETURNING id`

	return r.db.QueryRowContext(
		ctx, query,
		env.ConnectionID, env.EnvTag, env.DisplayName, env.BadgeColor, env.EncryptedConfig, env.IsDefault,
	).Scan(&env.ID)
}

func (r *ConnectionRepository) GetEnvironment(ctx context.Context, connectionID, envTag string) (*entity.ConnectionEnvironment, error) {
	query := `
		SELECT id, connection_id, env_tag, display_name, badge_color, encrypted_config, cached_schema, is_default, last_synced_at
		FROM connection_environments
		WHERE connection_id = $1 AND env_tag = $2`

	var env entity.ConnectionEnvironment
	var cachedSchema sql.NullString
	err := r.db.QueryRowContext(ctx, query, connectionID, envTag).Scan(
		&env.ID, &env.ConnectionID, &env.EnvTag, &env.DisplayName, &env.BadgeColor,
		&env.EncryptedConfig, &cachedSchema, &env.IsDefault, &env.LastSyncedAt,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to get connection environment: %w", err)
	}

	if cachedSchema.Valid {
		env.CachedSchema = []byte(cachedSchema.String)
	}

	return &env, nil
}

func (r *ConnectionRepository) ListEnvironments(ctx context.Context, connectionID string) ([]entity.ConnectionEnvironment, error) {
	query := `
		SELECT id, connection_id, env_tag, display_name, badge_color, encrypted_config, cached_schema, is_default, last_synced_at
		FROM connection_environments
		WHERE connection_id = $1
		ORDER BY is_default DESC, env_tag ASC`

	rows, err := r.db.QueryContext(ctx, query, connectionID)
	if err != nil {
		return nil, fmt.Errorf("failed to list environments: %w", err)
	}
	defer rows.Close()

	var envs []entity.ConnectionEnvironment
	for rows.Next() {
		var env entity.ConnectionEnvironment
		var cachedSchema sql.NullString
		if err := rows.Scan(
			&env.ID, &env.ConnectionID, &env.EnvTag, &env.DisplayName, &env.BadgeColor,
			&env.EncryptedConfig, &cachedSchema, &env.IsDefault, &env.LastSyncedAt,
		); err != nil {
			return nil, fmt.Errorf("failed to scan environment: %w", err)
		}
		if cachedSchema.Valid {
			env.CachedSchema = []byte(cachedSchema.String)
		}
		envs = append(envs, env)
	}

	return envs, nil
}

func (r *ConnectionRepository) UpdateCachedSchema(ctx context.Context, connectionID, envTag string, schemaJSON []byte) error {
	query := `
		UPDATE connection_environments
		SET cached_schema = $1, last_synced_at = NOW()
		WHERE connection_id = $2 AND env_tag = $3`

	_, err := r.db.ExecContext(ctx, query, string(schemaJSON), connectionID, envTag)
	return err
}
