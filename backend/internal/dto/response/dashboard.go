package response

import (
	"encoding/json"
	"time"
)

// ProjectResponse is a catalog entry in the workspace project switcher.
type ProjectResponse struct {
	ID          string    `json:"id"`
	WorkspaceID string    `json:"workspace_id"`
	Name        string    `json:"name"`
	Slug        string    `json:"slug"`
	Description string    `json:"description"`
	Icon        string    `json:"icon"`
	CreatedAt   time.Time `json:"created_at"`
}

// ProjectMemberResponse exposes the granular role and RLS rules of a member.
type ProjectMemberResponse struct {
	ID           string          `json:"id"`
	ProjectID    string          `json:"project_id"`
	UserID       string          `json:"user_id"`
	UserEmail    string          `json:"user_email"`
	UserFullName string          `json:"user_full_name"`
	Role         string          `json:"role"`
	RLSRules     json.RawMessage `json:"rls_rules"`
	CreatedAt    time.Time       `json:"created_at"`
}

// WidgetResponse is one canvas card.
type WidgetResponse struct {
	ID              string          `json:"id"`
	DashboardID     string          `json:"dashboard_id"`
	Title           string          `json:"title"`
	ChartType       string          `json:"chart_type"`
	QueryMode       string          `json:"query_mode"`
	BuilderState    json.RawMessage `json:"builder_state,omitempty"`
	RawSQL          string          `json:"raw_sql"`
	EchartsConfig   json.RawMessage `json:"echarts_config"`
	CacheTTLSeconds int             `json:"cache_ttl_seconds"`
	MaxRows         int             `json:"max_rows"`
	CreatedAt       time.Time       `json:"created_at"`
}

// DashboardSummaryResponse is the catalog card without nested widgets.
type DashboardSummaryResponse struct {
	ID              string          `json:"id"`
	WorkspaceID     string          `json:"workspace_id"`
	ProjectID       string          `json:"project_id"`
	Title           string          `json:"title"`
	Description     string          `json:"description"`
	LayoutConfig    json.RawMessage `json:"layout_config"`
	GlobalFilters   json.RawMessage `json:"global_filters"`
	CacheTTLSeconds int             `json:"cache_ttl_seconds"`
	CreatedBy       *string         `json:"created_by,omitempty"`
	UpdatedAt       time.Time       `json:"updated_at"`
}

// DashboardDetailResponse is the full canvas payload including every widget.
type DashboardDetailResponse struct {
	DashboardSummaryResponse
	Widgets []WidgetResponse `json:"widgets"`
}
