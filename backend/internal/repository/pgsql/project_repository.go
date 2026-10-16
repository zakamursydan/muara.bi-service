package pgsql

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"muara.bi/core/internal/domain/entity"
)

type ProjectRepositoryProvider interface {
	CreateProject(ctx context.Context, project *entity.Project) error
	GetProjectByID(ctx context.Context, workspaceID, id string) (*entity.Project, error)
	GetProjectBySlug(ctx context.Context, workspaceID, slug string) (*entity.Project, error)
	ListProjects(ctx context.Context, workspaceID string) ([]entity.Project, error)

	UpsertProjectMember(ctx context.Context, member *entity.ProjectMember) error
	ListProjectMembers(ctx context.Context, projectID string) ([]entity.ProjectMember, error)
	GetProjectMember(ctx context.Context, projectID, userID string) (*entity.ProjectMember, error)
}

type ProjectRepository struct {
	db *sql.DB
}

func NewProjectRepository(db *sql.DB) *ProjectRepository {
	return &ProjectRepository{db: db}
}

func (r *ProjectRepository) CreateProject(ctx context.Context, project *entity.Project) error {
	query := `
		INSERT INTO projects (workspace_id, name, slug, description, icon)
		VALUES ($1, $2, $3, $4, $5)
		RETURNING id, created_at`

	icon := project.Icon
	if icon == "" {
		icon = "folder"
	}

	return r.db.QueryRowContext(ctx, query,
		project.WorkspaceID, project.Name, project.Slug, project.Description, icon,
	).Scan(&project.ID, &project.CreatedAt)
}

func (r *ProjectRepository) GetProjectByID(ctx context.Context, workspaceID, id string) (*entity.Project, error) {
	query := `
		SELECT id, workspace_id, name, slug, description, icon, created_at
		FROM projects
		WHERE workspace_id = $1 AND id = $2`

	var p entity.Project
	err := r.db.QueryRowContext(ctx, query, workspaceID, id).Scan(
		&p.ID, &p.WorkspaceID, &p.Name, &p.Slug, &p.Description, &p.Icon, &p.CreatedAt,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to get project: %w", err)
	}
	return &p, nil
}

func (r *ProjectRepository) GetProjectBySlug(ctx context.Context, workspaceID, slug string) (*entity.Project, error) {
	query := `
		SELECT id, workspace_id, name, slug, description, icon, created_at
		FROM projects
		WHERE workspace_id = $1 AND slug = $2`

	var p entity.Project
	err := r.db.QueryRowContext(ctx, query, workspaceID, slug).Scan(
		&p.ID, &p.WorkspaceID, &p.Name, &p.Slug, &p.Description, &p.Icon, &p.CreatedAt,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to get project by slug: %w", err)
	}
	return &p, nil
}

func (r *ProjectRepository) ListProjects(ctx context.Context, workspaceID string) ([]entity.Project, error) {
	query := `
		SELECT id, workspace_id, name, slug, description, icon, created_at
		FROM projects
		WHERE workspace_id = $1
		ORDER BY name ASC`

	rows, err := r.db.QueryContext(ctx, query, workspaceID)
	if err != nil {
		return nil, fmt.Errorf("failed to list projects: %w", err)
	}
	defer rows.Close()

	var projects []entity.Project
	for rows.Next() {
		var p entity.Project
		if err := rows.Scan(&p.ID, &p.WorkspaceID, &p.Name, &p.Slug, &p.Description, &p.Icon, &p.CreatedAt); err != nil {
			return nil, fmt.Errorf("failed to scan project: %w", err)
		}
		projects = append(projects, p)
	}
	return projects, rows.Err()
}

func (r *ProjectRepository) UpsertProjectMember(ctx context.Context, member *entity.ProjectMember) error {
	query := `
		INSERT INTO project_members (project_id, user_id, role, rls_rules)
		VALUES ($1, $2, $3, $4)
		ON CONFLICT (project_id, user_id) DO UPDATE
			SET role = EXCLUDED.role, rls_rules = EXCLUDED.rls_rules
		RETURNING id, created_at`

	rules := member.RLSRules
	if len(rules) == 0 {
		rules = []byte("[]")
	}

	return r.db.QueryRowContext(ctx, query,
		member.ProjectID, member.UserID, member.Role, rules,
	).Scan(&member.ID, &member.CreatedAt)
}

func (r *ProjectRepository) ListProjectMembers(ctx context.Context, projectID string) ([]entity.ProjectMember, error) {
	query := `
		SELECT pm.id, pm.project_id, pm.user_id, pm.role, pm.rls_rules, pm.created_at,
		       u.email, u.full_name
		FROM project_members pm
		JOIN users u ON u.id = pm.user_id
		WHERE pm.project_id = $1
		ORDER BY u.full_name ASC`

	rows, err := r.db.QueryContext(ctx, query, projectID)
	if err != nil {
		return nil, fmt.Errorf("failed to list project members: %w", err)
	}
	defer rows.Close()

	var members []entity.ProjectMember
	for rows.Next() {
		m, err := scanProjectMember(rows)
		if err != nil {
			return nil, err
		}
		members = append(members, *m)
	}
	return members, rows.Err()
}

func (r *ProjectRepository) GetProjectMember(ctx context.Context, projectID, userID string) (*entity.ProjectMember, error) {
	query := `
		SELECT pm.id, pm.project_id, pm.user_id, pm.role, pm.rls_rules, pm.created_at,
		       u.email, u.full_name
		FROM project_members pm
		JOIN users u ON u.id = pm.user_id
		WHERE pm.project_id = $1 AND pm.user_id = $2`

	m, err := scanProjectMember(r.db.QueryRowContext(ctx, query, projectID, userID))
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	return m, nil
}

type rowScanner interface {
	Scan(dest ...any) error
}

func scanProjectMember(s rowScanner) (*entity.ProjectMember, error) {
	var m entity.ProjectMember
	var rules []byte
	if err := s.Scan(
		&m.ID, &m.ProjectID, &m.UserID, &m.Role, &rules, &m.CreatedAt,
		&m.UserEmail, &m.UserFullName,
	); err != nil {
		return nil, fmt.Errorf("failed to scan project member: %w", err)
	}
	if len(rules) > 0 {
		m.RLSRules = rules
	} else {
		m.RLSRules = []byte("[]")
	}
	return &m, nil
}
