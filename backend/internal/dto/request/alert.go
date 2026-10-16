package request

import "encoding/json"

type CreateAlertRequest struct {
	WidgetID          *string         `json:"widget_id,omitempty"`
	Title             string          `json:"title"`
	AlertType         string          `json:"alert_type"` // SCHEDULED_DIGEST or THRESHOLD_ALERT
	CronExpression    string          `json:"cron_expression"`
	ConditionOperator *string         `json:"condition_operator,omitempty"` // >, <, >=, <=, ==, !=
	ThresholdValue    *float64        `json:"threshold_value,omitempty"`
	Channel           string          `json:"channel"` // TELEGRAM, SLACK, EMAIL, WEBHOOK
	RecipientConfig   json.RawMessage `json:"recipient_config"`
	EnvTag            string          `json:"env_tag"`
	IsActive          *bool           `json:"is_active,omitempty"`
}

type UpdateAlertRequest struct {
	WidgetID          *string         `json:"widget_id,omitempty"`
	Title             string          `json:"title"`
	CronExpression    string          `json:"cron_expression"`
	ConditionOperator *string         `json:"condition_operator,omitempty"`
	ThresholdValue    *float64        `json:"threshold_value,omitempty"`
	Channel           string          `json:"channel"`
	RecipientConfig   json.RawMessage `json:"recipient_config"`
	EnvTag            string          `json:"env_tag"`
	IsActive          bool            `json:"is_active"`
}
