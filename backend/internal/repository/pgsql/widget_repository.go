package pgsql

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"muara.bi/core/internal/domain/entity"
)

type WidgetRepositoryProvider interface {
	CreateWidget(ctx context.Context, widget *entity.Widget) error
	GetWidgetByID(ctx context.Context, workspaceID, widgetID string) (*entity.Widget, error)
	ListWidgetsByDashboard(ctx context.Context, dashboardID string) ([]entity.Widget, error)
	UpdateWidget(ctx context.Context, workspaceID string, widget *entity.Widget) error
	DeleteWidget(ctx context.Context, workspaceID, widgetID string) error
	GetProjectIDByWidget(ctx context.Context, workspaceID, widgetID string) (string, error)
}

type WidgetRepository struct {
	db *sql.DB
}

func NewWidgetRepository(db *sql.DB) *WidgetRepository {
	return &WidgetRepository{db: db}
}

const widgetColumns = `w.id, w.dashboard_id, w.title, w.chart_type, w.query_mode,
	w.builder_state, w.raw_sql, w.echarts_config, w.cache_ttl_seconds, w.max_rows, w.created_at`

func (r *WidgetRepository) CreateWidget(ctx context.Context, w *entity.Widget) error {
	const query = `
		INSERT INTO widgets (dashboard_id, title, chart_type, query_mode, builder_state, raw_sql, echarts_config, cache_ttl_seconds, max_rows)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
		RETURNING id, created_at`

	builderState := nullableJSON(w.BuilderState)
	echartsConfig := jsonOrEmptyObject(w.EchartsConfig)
	cacheTTL := w.CacheTTLSeconds
	maxRows := w.MaxRows

	return r.db.QueryRowContext(ctx, query,
		w.DashboardID, w.Title, w.ChartType, w.QueryMode, builderState, w.RawSQL,
		echartsConfig, cacheTTL, maxRows,
	).Scan(&w.ID, &w.CreatedAt)
}

func (r *WidgetRepository) GetWidgetByID(ctx context.Context, workspaceID, widgetID string) (*entity.Widget, error) {
	query := fmt.Sprintf(`
		SELECT %s
		FROM widgets w
		JOIN dashboards d ON d.id = w.dashboard_id
		WHERE d.workspace_id = $1 AND w.id = $2`, widgetColumns)

	w, err := scanWidget(r.db.QueryRowContext(ctx, query, workspaceID, widgetID))
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	return w, nil
}

func (r *WidgetRepository) ListWidgetsByDashboard(ctx context.Context, dashboardID string) ([]entity.Widget, error) {
	query := fmt.Sprintf(`
		SELECT %s
		FROM widgets w
		WHERE w.dashboard_id = $1
		ORDER BY w.created_at ASC`, widgetColumns)

	rows, err := r.db.QueryContext(ctx, query, dashboardID)
	if err != nil {
		return nil, fmt.Errorf("failed to list widgets: %w", err)
	}
	defer rows.Close()

	var widgets []entity.Widget
	for rows.Next() {
		w, err := scanWidget(rows)
		if err != nil {
			return nil, err
		}
		widgets = append(widgets, *w)
	}
	return widgets, rows.Err()
}

func (r *WidgetRepository) UpdateWidget(ctx context.Context, workspaceID string, w *entity.Widget) error {
	const query = `
		UPDATE widgets
		SET title = $1, chart_type = $2, query_mode = $3, builder_state = $4,
		    raw_sql = $5, echarts_config = $6, cache_ttl_seconds = $7, max_rows = $8
		FROM dashboards d
		WHERE d.id = widgets.dashboard_id AND d.workspace_id = $9 AND widgets.id = $10
		RETURNING widgets.id`

	var id string
	err := r.db.QueryRowContext(ctx, query,
		w.Title, w.ChartType, w.QueryMode, nullableJSON(w.BuilderState), w.RawSQL,
		jsonOrEmptyObject(w.EchartsConfig), w.CacheTTLSeconds, w.MaxRows, workspaceID, w.ID,
	).Scan(&id)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return sql.ErrNoRows
		}
		return fmt.Errorf("failed to update widget: %w", err)
	}
	return nil
}

func (r *WidgetRepository) DeleteWidget(ctx context.Context, workspaceID, widgetID string) error {
	const query = `
		DELETE FROM widgets
		USING dashboards d
		WHERE d.id = widgets.dashboard_id AND d.workspace_id = $1 AND widgets.id = $2`

	res, err := r.db.ExecContext(ctx, query, workspaceID, widgetID)
	if err != nil {
		return fmt.Errorf("failed to delete widget: %w", err)
	}
	if affected, err := res.RowsAffected(); err == nil && affected == 0 {
		return sql.ErrNoRows
	}
	return nil
}

// GetProjectIDByWidget resolves the owning project through the parent dashboard
// so widget routes can enforce project-scoped RBAC and RLS. Returns "" when the
// widget does not exist in the given workspace.
func (r *WidgetRepository) GetProjectIDByWidget(ctx context.Context, workspaceID, widgetID string) (string, error) {
	const query = `
		SELECT d.project_id
		FROM widgets w
		JOIN dashboards d ON d.id = w.dashboard_id
		WHERE w.id = $1 AND d.workspace_id = $2`

	var projID string
	err := r.db.QueryRowContext(ctx, query, widgetID, workspaceID).Scan(&projID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return "", nil
		}
		return "", fmt.Errorf("failed to resolve widget project: %w", err)
	}
	return projID, nil
}

func scanWidget(s rowScanner) (*entity.Widget, error) {
	var w entity.Widget
	var builderState, echartsConfig []byte

	if err := s.Scan(
		&w.ID, &w.DashboardID, &w.Title, &w.ChartType, &w.QueryMode,
		&builderState, &w.RawSQL, &echartsConfig, &w.CacheTTLSeconds, &w.MaxRows, &w.CreatedAt,
	); err != nil {
		return nil, fmt.Errorf("failed to scan widget: %w", err)
	}

	w.BuilderState = builderState
	w.EchartsConfig = echartsConfig
	return &w, nil
}

func nullableJSON(raw []byte) any {
	if len(raw) == 0 {
		return nil
	}
	return raw
}

func jsonOrEmptyObject(raw []byte) []byte {
	if len(raw) == 0 {
		return []byte("{}")
	}
	return raw
}
