package entity

import (
	"encoding/json"
	"time"
)

// Workspace roles (workspace_members.role)
const (
	RoleOwner    = "OWNER"
	RoleAdmin    = "ADMIN"
	RoleEngineer = "ENGINEER"
	RoleEditor   = "EDITOR"
	RoleViewer   = "VIEWER"
)

// Project roles (project_members.role)
const (
	ProjectRoleAdmin  = "ADMIN"
	ProjectRoleEditor = "EDITOR"
	ProjectRoleViewer = "VIEWER"
)

// Widget query modes
const (
	QueryModeSynced    = "SYNCED"
	QueryModeCustomSQL = "CUSTOM_SQL"
)

// Project is an app-scoped analytical container inside a workspace.
type Project struct {
	ID          string    `json:"id"`
	WorkspaceID string    `json:"workspace_id"`
	Name        string    `json:"name"`
	Slug        string    `json:"slug"`
	Description string    `json:"description"`
	Icon        string    `json:"icon"`
	CreatedAt   time.Time `json:"created_at"`
}

// ProjectMember carries a project-scoped role plus its Row-Level Security rules.
// UserEmail and UserFullName are only populated by listing queries that join users.
type ProjectMember struct {
	ID           string          `json:"id"`
	ProjectID    string          `json:"project_id"`
	UserID       string          `json:"user_id"`
	Role         string          `json:"role"`
	RLSRules     json.RawMessage `json:"rls_rules"`
	CreatedAt    time.Time       `json:"created_at"`
	UserEmail    string          `json:"user_email,omitempty"`
	UserFullName string          `json:"user_full_name,omitempty"`
}

// RLSRuleSpec is one entry of project_members.rls_rules. ValueFromUser names an
// attribute key resolved from the requesting user's users.attributes payload.
type RLSRuleSpec struct {
	Column        string `json:"column"`
	Operator      string `json:"operator"`
	ValueFromUser string `json:"value_from_user,omitempty"`
	Value         any    `json:"value,omitempty"`
}

// Dashboard is a 12-column grid of widgets scoped to a project.
type Dashboard struct {
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
	Widgets         []Widget        `json:"widgets,omitempty"`
}

// Widget is a single visual card. RawSQL is authoritative for CUSTOM_SQL mode,
// while BuilderState is regenerated into RawSQL for SYNCED mode.
type Widget struct {
	ID              string          `json:"id"`
	DashboardID     string          `json:"dashboard_id"`
	Title           string          `json:"title"`
	ChartType       string          `json:"chart_type"`
	QueryMode       string          `json:"query_mode"`
	BuilderState    json.RawMessage `json:"builder_state"`
	RawSQL          string          `json:"raw_sql"`
	EchartsConfig   json.RawMessage `json:"echarts_config"`
	CacheTTLSeconds int             `json:"cache_ttl_seconds"`
	MaxRows         int             `json:"max_rows"`
	CreatedAt       time.Time       `json:"created_at"`
}
