package pgsql

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"muara.bi/core/internal/domain/entity"
)

type UserRepositoryProvider interface {
	CreateUser(ctx context.Context, u *entity.User) error
	GetUserByEmail(ctx context.Context, email string) (*entity.User, error)
	GetUserByID(ctx context.Context, id string) (*entity.User, error)
	CreateWorkspace(ctx context.Context, ws *entity.Workspace) error
	AddWorkspaceMember(ctx context.Context, m *entity.WorkspaceMember) error
	GetUserWorkspaces(ctx context.Context, userID string) ([]entity.Workspace, error)
	GetWorkspaceRole(ctx context.Context, workspaceID, userID string) (string, error)
}

type UserRepository struct {
	db *sql.DB
}

func NewUserRepository(db *sql.DB) UserRepositoryProvider {
	return &UserRepository{db: db}
}

func (r *UserRepository) CreateUser(ctx context.Context, u *entity.User) error {
	query := `
		INSERT INTO users (email, password_hash, full_name, avatar_url, attributes)
		VALUES ($1, $2, $3, $4, $5)
		RETURNING id, created_at, updated_at
	`
	attrs := u.Attributes
	if len(attrs) == 0 {
		attrs = []byte("{}")
	}

	return r.db.QueryRowContext(ctx, query,
		u.Email,
		u.PasswordHash,
		u.FullName,
		u.AvatarURL,
		attrs,
	).Scan(&u.ID, &u.CreatedAt, &u.UpdatedAt)
}

func (r *UserRepository) GetUserByEmail(ctx context.Context, email string) (*entity.User, error) {
	query := `
		SELECT id, email, password_hash, full_name, avatar_url, attributes, created_at, updated_at
		FROM users
		WHERE email = $1
	`
	var u entity.User
	var rawAttrs []byte

	err := r.db.QueryRowContext(ctx, query, email).Scan(
		&u.ID,
		&u.Email,
		&u.PasswordHash,
		&u.FullName,
		&u.AvatarURL,
		&rawAttrs,
		&u.CreatedAt,
		&u.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, fmt.Errorf("error querying user by email: %w", err)
	}

	u.Attributes = rawAttrs
	return &u, nil
}

func (r *UserRepository) GetUserByID(ctx context.Context, id string) (*entity.User, error) {
	query := `
		SELECT id, email, password_hash, full_name, avatar_url, attributes, created_at, updated_at
		FROM users
		WHERE id = $1
	`
	var u entity.User
	var rawAttrs []byte

	err := r.db.QueryRowContext(ctx, query, id).Scan(
		&u.ID,
		&u.Email,
		&u.PasswordHash,
		&u.FullName,
		&u.AvatarURL,
		&rawAttrs,
		&u.CreatedAt,
		&u.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, fmt.Errorf("error querying user by id: %w", err)
	}

	u.Attributes = rawAttrs
	return &u, nil
}

func (r *UserRepository) CreateWorkspace(ctx context.Context, ws *entity.Workspace) error {
	query := `
		INSERT INTO workspaces (name, slug, plan_tier)
		VALUES ($1, $2, $3)
		RETURNING id, created_at
	`
	plan := ws.PlanTier
	if plan == "" {
		plan = "pro"
	}
	return r.db.QueryRowContext(ctx, query, ws.Name, ws.Slug, plan).Scan(&ws.ID, &ws.CreatedAt)
}

func (r *UserRepository) AddWorkspaceMember(ctx context.Context, m *entity.WorkspaceMember) error {
	query := `
		INSERT INTO workspace_members (workspace_id, user_id, role)
		VALUES ($1, $2, $3)
		ON CONFLICT (workspace_id, user_id) DO UPDATE SET role = EXCLUDED.role
		RETURNING created_at
	`
	return r.db.QueryRowContext(ctx, query, m.WorkspaceID, m.UserID, m.Role).Scan(&m.CreatedAt)
}

// GetWorkspaceRole returns the caller's workspace_members.role, or "" when they
// are not a member. Callers must treat "" as "no permissions".
func (r *UserRepository) GetWorkspaceRole(ctx context.Context, workspaceID, userID string) (string, error) {
	query := `
		SELECT role FROM workspace_members
		WHERE workspace_id = $1 AND user_id = $2
	`

	var role string
	err := r.db.QueryRowContext(ctx, query, workspaceID, userID).Scan(&role)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return "", nil
		}
		return "", fmt.Errorf("failed to resolve workspace role: %w", err)
	}
	return role, nil
}

func (r *UserRepository) GetUserWorkspaces(ctx context.Context, userID string) ([]entity.Workspace, error) {
	query := `
		SELECT w.id, w.name, w.slug, w.plan_tier, w.created_at
		FROM workspaces w
		JOIN workspace_members wm ON w.id = wm.workspace_id
		WHERE wm.user_id = $1
		ORDER BY w.created_at ASC
	`
	rows, err := r.db.QueryContext(ctx, query, userID)
	if err != nil {
		return nil, fmt.Errorf("error querying user workspaces: %w", err)
	}
	defer rows.Close()

	var workspaces []entity.Workspace
	for rows.Next() {
		var ws entity.Workspace
		if err := rows.Scan(&ws.ID, &ws.Name, &ws.Slug, &ws.PlanTier, &ws.CreatedAt); err != nil {
			return nil, err
		}
		workspaces = append(workspaces, ws)
	}
	return workspaces, nil
}
