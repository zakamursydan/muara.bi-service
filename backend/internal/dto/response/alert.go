package response

import (
	"encoding/json"
	"time"
)

type AlertResponse struct {
	ID                string          `json:"id"`
	WorkspaceID       string          `json:"workspace_id"`
	DashboardID       string          `json:"dashboard_id"`
	WidgetID          *string         `json:"widget_id,omitempty"`
	Title             string          `json:"title"`
	AlertType         string          `json:"alert_type"`
	CronExpression    string          `json:"cron_expression"`
	ConditionOperator *string         `json:"condition_operator,omitempty"`
	ThresholdValue    *float64        `json:"threshold_value,omitempty"`
	Channel           string          `json:"channel"`
	RecipientConfig   json.RawMessage `json:"recipient_config"`
	EnvTag            string          `json:"env_tag"`
	IsActive          bool            `json:"is_active"`
	LastTriggeredAt   *time.Time      `json:"last_triggered_at,omitempty"`
	CreatedAt         time.Time       `json:"created_at"`
}
