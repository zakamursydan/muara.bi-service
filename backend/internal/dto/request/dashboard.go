package request

import (
	"encoding/json"

	"muara.bi/core/internal/domain/entity"
)

// CreateProjectRequest creates an app-scoped analytical container
// (e.g. Apps Omnichannel, Apps Chatbot, Retail ERP).
type CreateProjectRequest struct {
	Name        string `json:"name"`
	Slug        string `json:"slug"`
	Description string `json:"description"`
	Icon        string `json:"icon"`
}

// AssignProjectMemberRequest grants a project role together with its
// Row-Level Security rules.
type AssignProjectMemberRequest struct {
	UserID   string               `json:"user_id"`
	Role     string               `json:"role"`
	RLSRules []entity.RLSRuleSpec `json:"rls_rules"`
}

// CreateDashboardRequest supports blank, template and AI generated canvases.
type CreateDashboardRequest struct {
	Title           string          `json:"title"`
	Description     string          `json:"description"`
	LayoutConfig    json.RawMessage `json:"layout_config"`
	GlobalFilters   json.RawMessage `json:"global_filters"`
	CacheTTLSeconds *int            `json:"cache_ttl_seconds"`
}

// UpdateDashboardRequest carries the debounced auto-save payload from the canvas.
type UpdateDashboardRequest struct {
	Title           string          `json:"title"`
	Description     string          `json:"description"`
	LayoutConfig    json.RawMessage `json:"layout_config"`
	GlobalFilters   json.RawMessage `json:"global_filters"`
	CacheTTLSeconds *int            `json:"cache_ttl_seconds"`
}

// CreateWidgetRequest adds a card to a dashboard canvas.
type CreateWidgetRequest struct {
	Title           string          `json:"title"`
	ChartType       string          `json:"chart_type"`
	QueryMode       string          `json:"query_mode"`
	BuilderState    json.RawMessage `json:"builder_state"`
	RawSQL          string          `json:"raw_sql"`
	EchartsConfig   json.RawMessage `json:"echarts_config"`
	CacheTTLSeconds *int            `json:"cache_ttl_seconds"`
	MaxRows         *int            `json:"max_rows"`
}

// UpdateWidgetRequest edits a card's visual mapping or its custom SQL.
type UpdateWidgetRequest struct {
	Title           string          `json:"title"`
	ChartType       string          `json:"chart_type"`
	QueryMode       string          `json:"query_mode"`
	BuilderState    json.RawMessage `json:"builder_state"`
	RawSQL          string          `json:"raw_sql"`
	EchartsConfig   json.RawMessage `json:"echarts_config"`
	CacheTTLSeconds *int            `json:"cache_ttl_seconds"`
	MaxRows         *int            `json:"max_rows"`
}
