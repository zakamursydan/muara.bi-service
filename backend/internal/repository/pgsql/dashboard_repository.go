package pgsql

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"muara.bi/core/internal/domain/entity"
)

type DashboardRepositoryProvider interface {
	CreateDashboard(ctx context.Context, dashboard *entity.Dashboard) error
	GetDashboardByID(ctx context.Context, workspaceID, id string) (*entity.Dashboard, error)
	ListDashboardsByProject(ctx context.Context, workspaceID, projectID string) ([]entity.Dashboard, error)
	UpdateDashboard(ctx context.Context, dashboard *entity.Dashboard) error
	GetWorkspaceIDByDashboard(ctx context.Context, dashboardID string) (string, error)
	GetProjectIDByDashboard(ctx context.Context, workspaceID, dashboardID string) (string, error)
}

type DashboardRepository struct {
	db *sql.DB
}

func NewDashboardRepository(db *sql.DB) *DashboardRepository {
	return &DashboardRepository{db: db}
}

const dashboardColumns = `id, workspace_id, project_id, title, description,
	layout_config, global_filters, cache_ttl_seconds, created_by, updated_at`

func (r *DashboardRepository) CreateDashboard(ctx context.Context, d *entity.Dashboard) error {
	const query = `
		INSERT INTO dashboards (workspace_id, project_id, title, description, layout_config, global_filters, cache_ttl_seconds, created_by)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
		RETURNING id, updated_at`

	layout := jsonOrEmptyArray(d.LayoutConfig)
	filters := jsonOrEmptyArray(d.GlobalFilters)

	return r.db.QueryRowContext(ctx, query,
		d.WorkspaceID, d.ProjectID, d.Title, d.Description, layout, filters, d.CacheTTLSeconds, d.CreatedBy,
	).Scan(&d.ID, &d.UpdatedAt)
}

func (r *DashboardRepository) GetDashboardByID(ctx context.Context, workspaceID, id string) (*entity.Dashboard, error) {
	query := fmt.Sprintf(`SELECT %s FROM dashboards WHERE workspace_id = $1 AND id = $2`, dashboardColumns)

	d, err := scanDashboard(r.db.QueryRowContext(ctx, query, workspaceID, id))
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	return d, nil
}

func (r *DashboardRepository) ListDashboardsByProject(ctx context.Context, workspaceID, projectID string) ([]entity.Dashboard, error) {
	query := fmt.Sprintf(`
		SELECT %s FROM dashboards
		WHERE workspace_id = $1 AND project_id = $2
		ORDER BY title ASC`, dashboardColumns)

	rows, err := r.db.QueryContext(ctx, query, workspaceID, projectID)
	if err != nil {
		return nil, fmt.Errorf("failed to list dashboards: %w", err)
	}
	defer rows.Close()

	var dashboards []entity.Dashboard
	for rows.Next() {
		d, err := scanDashboard(rows)
		if err != nil {
			return nil, err
		}
		dashboards = append(dashboards, *d)
	}
	return dashboards, rows.Err()
}

func (r *DashboardRepository) UpdateDashboard(ctx context.Context, d *entity.Dashboard) error {
	const query = `
		UPDATE dashboards
		SET title = $1, description = $2, layout_config = $3, global_filters = $4,
		    cache_ttl_seconds = $5, updated_at = NOW()
		WHERE workspace_id = $6 AND id = $7
		RETURNING updated_at`

	return r.db.QueryRowContext(ctx, query,
		d.Title, d.Description, jsonOrEmptyArray(d.LayoutConfig), jsonOrEmptyArray(d.GlobalFilters),
		d.CacheTTLSeconds, d.WorkspaceID, d.ID,
	).Scan(&d.UpdatedAt)
}

// GetWorkspaceIDByDashboard resolves the owning tenant for routes that only
// carry a dashboard identifier, such as public share lookups.
func (r *DashboardRepository) GetWorkspaceIDByDashboard(ctx context.Context, dashboardID string) (string, error) {
	var wsID string
	err := r.db.QueryRowContext(ctx, `SELECT workspace_id FROM dashboards WHERE id = $1`, dashboardID).Scan(&wsID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return "", nil
		}
		return "", fmt.Errorf("failed to resolve dashboard workspace: %w", err)
	}
	return wsID, nil
}

// GetProjectIDByDashboard resolves the project a dashboard belongs to so
// project-scoped RBAC and RLS rules can be applied to dashboard routes. Returns
// "" when the dashboard does not exist in the given workspace.
func (r *DashboardRepository) GetProjectIDByDashboard(ctx context.Context, workspaceID, dashboardID string) (string, error) {
	var projID string
	err := r.db.QueryRowContext(ctx,
		`SELECT project_id FROM dashboards WHERE id = $1 AND workspace_id = $2`,
		dashboardID, workspaceID,
	).Scan(&projID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return "", nil
		}
		return "", fmt.Errorf("failed to resolve dashboard project: %w", err)
	}
	return projID, nil
}

func scanDashboard(s rowScanner) (*entity.Dashboard, error) {
	var d entity.Dashboard
	var description sql.NullString
	var layout, filters []byte

	if err := s.Scan(
		&d.ID, &d.WorkspaceID, &d.ProjectID, &d.Title, &description,
		&layout, &filters, &d.CacheTTLSeconds, &d.CreatedBy, &d.UpdatedAt,
	); err != nil {
		return nil, fmt.Errorf("failed to scan dashboard: %w", err)
	}

	d.Description = description.String
	d.LayoutConfig = layout
	d.GlobalFilters = filters
	return &d, nil
}

func jsonOrEmptyArray(raw []byte) []byte {
	if len(raw) == 0 {
		return []byte("[]")
	}
	return raw
}
