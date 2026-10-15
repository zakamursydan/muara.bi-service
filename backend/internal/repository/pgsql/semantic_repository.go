package pgsql

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"muara.bi/core/internal/domain/entity"
)

type SemanticRepositoryProvider interface {
	CreateSemanticModel(ctx context.Context, model *entity.SemanticModel) error
	GetSemanticModelByID(ctx context.Context, workspaceID, id string) (*entity.SemanticModel, error)
	ListSemanticModelsByProject(ctx context.Context, workspaceID, projectID string) ([]entity.SemanticModel, error)
	UpdateSemanticModel(ctx context.Context, model *entity.SemanticModel) error
	DeleteSemanticModel(ctx context.Context, workspaceID, id string) error
}

type SemanticRepository struct {
	db *sql.DB
}

func NewSemanticRepository(db *sql.DB) *SemanticRepository {
	return &SemanticRepository{db: db}
}

func (r *SemanticRepository) CreateSemanticModel(ctx context.Context, model *entity.SemanticModel) error {
	query := `
		INSERT INTO semantic_models (workspace_id, project_id, name, description, raw_sql, created_by)
		VALUES ($1, $2, $3, $4, $5, $6)
		RETURNING id, created_at, updated_at`

	return r.db.QueryRowContext(
		ctx, query,
		model.WorkspaceID, model.ProjectID, model.Name, model.Description, model.RawSQL, model.CreatedBy,
	).Scan(&model.ID, &model.CreatedAt, &model.UpdatedAt)
}

func (r *SemanticRepository) GetSemanticModelByID(ctx context.Context, workspaceID, id string) (*entity.SemanticModel, error) {
	query := `
		SELECT id, workspace_id, project_id, name, description, raw_sql, cached_schema, created_by, created_at, updated_at
		FROM semantic_models
		WHERE workspace_id = $1 AND id = $2`

	var m entity.SemanticModel
	var cachedSchema sql.NullString
	err := r.db.QueryRowContext(ctx, query, workspaceID, id).Scan(
		&m.ID, &m.WorkspaceID, &m.ProjectID, &m.Name, &m.Description, &m.RawSQL,
		&cachedSchema, &m.CreatedBy, &m.CreatedAt, &m.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to get semantic model: %w", err)
	}

	if cachedSchema.Valid {
		m.CachedSchema = []byte(cachedSchema.String)
	}
	return &m, nil
}

func (r *SemanticRepository) ListSemanticModelsByProject(ctx context.Context, workspaceID, projectID string) ([]entity.SemanticModel, error) {
	query := `
		SELECT id, workspace_id, project_id, name, description, raw_sql, cached_schema, created_by, created_at, updated_at
		FROM semantic_models
		WHERE workspace_id = $1 AND project_id = $2
		ORDER BY name ASC`

	rows, err := r.db.QueryContext(ctx, query, workspaceID, projectID)
	if err != nil {
		return nil, fmt.Errorf("failed to list semantic models: %w", err)
	}
	defer rows.Close()

	var models []entity.SemanticModel
	for rows.Next() {
		var m entity.SemanticModel
		var cachedSchema sql.NullString
		if err := rows.Scan(
			&m.ID, &m.WorkspaceID, &m.ProjectID, &m.Name, &m.Description, &m.RawSQL,
			&cachedSchema, &m.CreatedBy, &m.CreatedAt, &m.UpdatedAt,
		); err != nil {
			return nil, fmt.Errorf("failed to scan semantic model: %w", err)
		}
		if cachedSchema.Valid {
			m.CachedSchema = []byte(cachedSchema.String)
		}
		models = append(models, m)
	}
	return models, nil
}

func (r *SemanticRepository) UpdateSemanticModel(ctx context.Context, model *entity.SemanticModel) error {
	query := `
		UPDATE semantic_models
		SET name = $1, description = $2, raw_sql = $3, updated_at = NOW()
		WHERE workspace_id = $4 AND id = $5`

	_, err := r.db.ExecContext(ctx, query, model.Name, model.Description, model.RawSQL, model.WorkspaceID, model.ID)
	return err
}

func (r *SemanticRepository) DeleteSemanticModel(ctx context.Context, workspaceID, id string) error {
	query := `DELETE FROM semantic_models WHERE workspace_id = $1 AND id = $2`
	_, err := r.db.ExecContext(ctx, query, workspaceID, id)
	return err
}
