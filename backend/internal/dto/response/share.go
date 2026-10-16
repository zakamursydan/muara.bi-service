package response

import (
	"encoding/json"
	"time"
)

// ShareResponse is the management view of a share. ShareURL and EmbedToken are
// only populated on creation, since the embed token is a bearer credential.
type ShareResponse struct {
	ID                string     `json:"id"`
	DashboardID       string     `json:"dashboard_id"`
	ShareToken        string     `json:"share_token"`
	AccessMode        string     `json:"access_mode"`
	EnvTag            string     `json:"env_tag"`
	PasswordProtected bool       `json:"password_protected"`
	AllowedDomains    []string   `json:"allowed_domains"`
	IsActive          bool       `json:"is_active"`
	ExpiresAt         *time.Time `json:"expires_at,omitempty"`
	CreatedAt         time.Time  `json:"created_at"`
	ShareURL          string     `json:"share_url,omitempty"`
	EmbedToken        string     `json:"embed_token,omitempty"`
}

// PublicWidgetResponse is the render-only view of a widget. RawSQL and
// BuilderState are deliberately omitted so a public link never discloses the
// query or the underlying schema.
type PublicWidgetResponse struct {
	ID            string          `json:"id"`
	Title         string          `json:"title"`
	ChartType     string          `json:"chart_type"`
	EchartsConfig json.RawMessage `json:"echarts_config"`
	MaxRows       int             `json:"max_rows"`
}

// PublicDashboardResponse is the unauthenticated canvas payload. When
// PasswordRequired is true the dashboard and widget fields stay empty until the
// viewer posts the password.
type PublicDashboardResponse struct {
	ShareToken       string                 `json:"share_token"`
	AccessMode       string                 `json:"access_mode"`
	PasswordRequired bool                   `json:"password_required"`
	FrameAncestors   []string               `json:"frame_ancestors"`
	ExpiresAt        *time.Time             `json:"expires_at,omitempty"`
	Title            string                 `json:"title,omitempty"`
	Description      string                 `json:"description,omitempty"`
	LayoutConfig     json.RawMessage        `json:"layout_config,omitempty"`
	GlobalFilters    json.RawMessage        `json:"global_filters,omitempty"`
	Widgets          []PublicWidgetResponse `json:"widgets,omitempty"`
}
