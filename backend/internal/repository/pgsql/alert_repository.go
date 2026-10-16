package pgsql

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"muara.bi/core/internal/domain/entity"
)

type AlertRepositoryProvider interface {
	CreateAlert(ctx context.Context, alert *entity.DashboardAlert) error
	GetAlertByID(ctx context.Context, wsID, alertID string) (*entity.DashboardAlert, error)
	ListAlertsByDashboard(ctx context.Context, wsID, dashID string) ([]entity.DashboardAlert, error)
	ListActiveAlerts(ctx context.Context) ([]entity.DashboardAlert, error)
	UpdateAlert(ctx context.Context, alert *entity.DashboardAlert) error
	DeleteAlert(ctx context.Context, wsID, alertID string) error
	UpdateLastTriggeredAt(ctx context.Context, alertID string) error
}

type AlertRepository struct {
	db *sql.DB
}

func NewAlertRepository(db *sql.DB) *AlertRepository {
	return &AlertRepository{db: db}
}

func (r *AlertRepository) CreateAlert(ctx context.Context, alert *entity.DashboardAlert) error {
	query := `
		INSERT INTO dashboard_alerts (
			workspace_id, dashboard_id, widget_id, title, alert_type,
			cron_expression, condition_operator, threshold_value, channel,
			recipient_config, env_tag, is_active, created_by
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13)
		RETURNING id, created_at`

	return r.db.QueryRowContext(
		ctx, query,
		alert.WorkspaceID,
		alert.DashboardID,
		alert.WidgetID,
		alert.Title,
		alert.AlertType,
		alert.CronExpression,
		alert.ConditionOperator,
		alert.ThresholdValue,
		alert.Channel,
		string(alert.RecipientConfig),
		alert.EnvTag,
		alert.IsActive,
		alert.CreatedBy,
	).Scan(&alert.ID, &alert.CreatedAt)
}

func (r *AlertRepository) GetAlertByID(ctx context.Context, wsID, alertID string) (*entity.DashboardAlert, error) {
	query := `
		SELECT id, workspace_id, dashboard_id, widget_id, title, alert_type,
		       cron_expression, condition_operator, threshold_value, channel,
		       recipient_config, env_tag, is_active, last_triggered_at, created_by, created_at
		FROM dashboard_alerts
		WHERE workspace_id = $1 AND id = $2`

	var a entity.DashboardAlert
	var recipientStr string
	err := r.db.QueryRowContext(ctx, query, wsID, alertID).Scan(
		&a.ID,
		&a.WorkspaceID,
		&a.DashboardID,
		&a.WidgetID,
		&a.Title,
		&a.AlertType,
		&a.CronExpression,
		&a.ConditionOperator,
		&a.ThresholdValue,
		&a.Channel,
		&recipientStr,
		&a.EnvTag,
		&a.IsActive,
		&a.LastTriggeredAt,
		&a.CreatedBy,
		&a.CreatedAt,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to query alert by id: %w", err)
	}

	a.RecipientConfig = []byte(recipientStr)
	return &a, nil
}

func (r *AlertRepository) ListAlertsByDashboard(ctx context.Context, wsID, dashID string) ([]entity.DashboardAlert, error) {
	query := `
		SELECT id, workspace_id, dashboard_id, widget_id, title, alert_type,
		       cron_expression, condition_operator, threshold_value, channel,
		       recipient_config, env_tag, is_active, last_triggered_at, created_by, created_at
		FROM dashboard_alerts
		WHERE workspace_id = $1 AND dashboard_id = $2
		ORDER BY created_at DESC`

	rows, err := r.db.QueryContext(ctx, query, wsID, dashID)
	if err != nil {
		return nil, fmt.Errorf("failed to list alerts by dashboard: %w", err)
	}
	defer rows.Close()

	var alerts []entity.DashboardAlert
	for rows.Next() {
		var a entity.DashboardAlert
		var recipientStr string
		if err := rows.Scan(
			&a.ID,
			&a.WorkspaceID,
			&a.DashboardID,
			&a.WidgetID,
			&a.Title,
			&a.AlertType,
			&a.CronExpression,
			&a.ConditionOperator,
			&a.ThresholdValue,
			&a.Channel,
			&recipientStr,
			&a.EnvTag,
			&a.IsActive,
			&a.LastTriggeredAt,
			&a.CreatedBy,
			&a.CreatedAt,
		); err != nil {
			return nil, fmt.Errorf("failed to scan alert row: %w", err)
		}
		a.RecipientConfig = []byte(recipientStr)
		alerts = append(alerts, a)
	}
	return alerts, nil
}

func (r *AlertRepository) ListActiveAlerts(ctx context.Context) ([]entity.DashboardAlert, error) {
	query := `
		SELECT id, workspace_id, dashboard_id, widget_id, title, alert_type,
		       cron_expression, condition_operator, threshold_value, channel,
		       recipient_config, env_tag, is_active, last_triggered_at, created_by, created_at
		FROM dashboard_alerts
		WHERE is_active = true
		ORDER BY created_at ASC`

	rows, err := r.db.QueryContext(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("failed to list active alerts: %w", err)
	}
	defer rows.Close()

	var alerts []entity.DashboardAlert
	for rows.Next() {
		var a entity.DashboardAlert
		var recipientStr string
		if err := rows.Scan(
			&a.ID,
			&a.WorkspaceID,
			&a.DashboardID,
			&a.WidgetID,
			&a.Title,
			&a.AlertType,
			&a.CronExpression,
			&a.ConditionOperator,
			&a.ThresholdValue,
			&a.Channel,
			&recipientStr,
			&a.EnvTag,
			&a.IsActive,
			&a.LastTriggeredAt,
			&a.CreatedBy,
			&a.CreatedAt,
		); err != nil {
			return nil, fmt.Errorf("failed to scan active alert row: %w", err)
		}
		a.RecipientConfig = []byte(recipientStr)
		alerts = append(alerts, a)
	}
	return alerts, nil
}

func (r *AlertRepository) UpdateAlert(ctx context.Context, alert *entity.DashboardAlert) error {
	query := `
		UPDATE dashboard_alerts
		SET widget_id = $1, title = $2, cron_expression = $3,
		    condition_operator = $4, threshold_value = $5, channel = $6,
		    recipient_config = $7, env_tag = $8, is_active = $9
		WHERE workspace_id = $10 AND id = $11`

	res, err := r.db.ExecContext(
		ctx, query,
		alert.WidgetID,
		alert.Title,
		alert.CronExpression,
		alert.ConditionOperator,
		alert.ThresholdValue,
		alert.Channel,
		string(alert.RecipientConfig),
		alert.EnvTag,
		alert.IsActive,
		alert.WorkspaceID,
		alert.ID,
	)
	if err != nil {
		return fmt.Errorf("failed to update alert: %w", err)
	}
	rowsAffected, _ := res.RowsAffected()
	if rowsAffected == 0 {
		return sql.ErrNoRows
	}
	return nil
}

func (r *AlertRepository) DeleteAlert(ctx context.Context, wsID, alertID string) error {
	query := `DELETE FROM dashboard_alerts WHERE workspace_id = $1 AND id = $2`
	res, err := r.db.ExecContext(ctx, query, wsID, alertID)
	if err != nil {
		return fmt.Errorf("failed to delete alert: %w", err)
	}
	rowsAffected, _ := res.RowsAffected()
	if rowsAffected == 0 {
		return sql.ErrNoRows
	}
	return nil
}

func (r *AlertRepository) UpdateLastTriggeredAt(ctx context.Context, alertID string) error {
	query := `UPDATE dashboard_alerts SET last_triggered_at = NOW() WHERE id = $1`
	_, err := r.db.ExecContext(ctx, query, alertID)
	return err
}
