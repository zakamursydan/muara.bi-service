package request

import "time"

// CreateShareRequest opens a dashboard to anonymous viewers. AllowedDomains is
// required for EMBED_IFRAME and ignored for PUBLIC_LINK.
type CreateShareRequest struct {
	AccessMode     string     `json:"access_mode"`
	EnvTag         string     `json:"env_tag"`
	Password       string     `json:"password"`
	AllowedDomains []string   `json:"allowed_domains"`
	ExpiresAt      *time.Time `json:"expires_at"`
}

// PublicWidgetQueryRequest carries only filter values for a public viewer. The
// SQL itself always comes from the stored widget row, never from the request.
type PublicWidgetQueryRequest struct {
	Parameters map[string]any `json:"parameters"`
}
