package response

import (
	"encoding/json"
	"time"
)

type ConnectionResponse struct {
	ID           string                    `json:"id"`
	WorkspaceID  string                    `json:"workspace_id"`
	AliasName    string                    `json:"alias_name"`
	Category     string                    `json:"category"`
	DriverType   string                    `json:"driver_type"`
	Environments []EnvironmentInfoResponse `json:"environments"`
	CreatedAt    time.Time                 `json:"created_at"`
}

type EnvironmentInfoResponse struct {
	ID               string `json:"id"`
	EnvTag           string `json:"env_tag"`
	DisplayName      string `json:"display_name"`
	BadgeColor       string `json:"badge_color"`
	IsDefault        bool   `json:"is_default"`
	SSHTunnelEnabled bool   `json:"ssh_tunnel_enabled"`
	Host             string `json:"host"`
	Port             int    `json:"port"`
	Database         string `json:"database"`
	User             string `json:"user"`
	// ssl_mode and root_cert are returned so the form can show what is actually
	// stored. Without them it falls back to its initial "prefer"/empty values and
	// the next save silently rewrites both. uri and password stay write-only: a
	// connection string can carry credentials.
	SSLMode      string          `json:"ssl_mode"`
	RootCert     string          `json:"root_cert"`
	CachedSchema json.RawMessage `json:"cached_schema,omitempty"`
	LastSyncedAt *time.Time      `json:"last_synced_at,omitempty"`
}

type TestConnectionResponse struct {
	Success   bool   `json:"success"`
	Message   string `json:"message"`
	LatencyMs int64  `json:"latency_ms"`
}
