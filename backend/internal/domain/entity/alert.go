package entity

import (
	"encoding/json"
	"time"
)

const (
	AlertTypeScheduledDigest = "SCHEDULED_DIGEST"
	AlertTypeThresholdAlert  = "THRESHOLD_ALERT"

	ChannelTelegram = "TELEGRAM"
	ChannelSlack    = "SLACK"
	ChannelEmail    = "EMAIL"
	ChannelWebhook  = "WEBHOOK"
)

// DashboardAlert represents a scheduled report or threshold anomaly monitor
type DashboardAlert struct {
	ID                string          `json:"id"`
	WorkspaceID       string          `json:"workspace_id"`
	DashboardID       string          `json:"dashboard_id"`
	WidgetID          *string         `json:"widget_id,omitempty"`
	Title             string          `json:"title"`
	AlertType         string          `json:"alert_type"` // SCHEDULED_DIGEST or THRESHOLD_ALERT
	CronExpression    string          `json:"cron_expression"`
	ConditionOperator *string         `json:"condition_operator,omitempty"` // >, <, >=, <=, ==, !=
	ThresholdValue    *float64        `json:"threshold_value,omitempty"`
	Channel           string          `json:"channel"` // TELEGRAM, SLACK, EMAIL, WEBHOOK
	RecipientConfig   json.RawMessage `json:"recipient_config"`
	EnvTag            string          `json:"env_tag"`
	IsActive          bool            `json:"is_active"`
	LastTriggeredAt   *time.Time      `json:"last_triggered_at,omitempty"`
	CreatedBy         *string         `json:"created_by,omitempty"`
	CreatedAt         time.Time       `json:"created_at"`
}
