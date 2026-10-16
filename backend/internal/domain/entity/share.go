package entity

import "time"

// Share access modes (dashboard_shares.access_mode).
const (
	AccessModePublicLink  = "PUBLIC_LINK"
	AccessModeEmbedIFrame = "EMBED_IFRAME"
)

// DashboardShare is a revocable public grant to read one dashboard.
// PasswordHash is empty when the share is unprotected. WorkspaceID is not a
// column on dashboard_shares; it is populated by lookups that join dashboards.
type DashboardShare struct {
	ID             string
	WorkspaceID    string
	DashboardID    string
	ShareToken     string
	AccessMode     string
	EnvTag         string
	PasswordHash   string
	AllowedDomains []string
	IsActive       bool
	ExpiresAt      *time.Time
	CreatedBy      *string
	CreatedAt      time.Time
}

// IsExpired reports whether the optional expiry timestamp has passed.
func (s *DashboardShare) IsExpired(now time.Time) bool {
	return s.ExpiresAt != nil && now.After(*s.ExpiresAt)
}

// IsAccessible reports whether the share can still serve a dashboard.
func (s *DashboardShare) IsAccessible(now time.Time) bool {
	return s.IsActive && !s.IsExpired(now)
}
