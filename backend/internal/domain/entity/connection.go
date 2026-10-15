package entity

import (
	"encoding/json"
	"time"
)

// DataConnection represents a logical connection alias (e.g., db_transaksi)
type DataConnection struct {
	ID          string    `json:"id"`
	WorkspaceID string    `json:"workspace_id"`
	AliasName   string    `json:"alias_name"`
	Category    string    `json:"category"`    // SQL, NOSQL, FILE, CLOUD_DW
	DriverType  string    `json:"driver_type"` // postgres, mysql, mongodb, sqlite, etc.
	CreatedAt   time.Time `json:"created_at"`
}

// ConnectionConfig is the decrypted database credential payload
type ConnectionConfig struct {
	Host       string         `json:"host"`
	Port       int            `json:"port"`
	Database   string         `json:"database"`
	User       string         `json:"user"`
	Password   string         `json:"password"`
	SSLMode    string         `json:"ssl_mode,omitempty"`
	URI        string         `json:"uri,omitempty"` // For MongoDB / Cloud connections
	SSHTunnel  *SSHConfigJSON `json:"ssh_tunnel,omitempty"`
}

type SSHConfigJSON struct {
	Enabled    bool   `json:"enabled"`
	Host       string `json:"host"`
	Port       int    `json:"port"`
	User       string `json:"user"`
	Password   string `json:"password,omitempty"`
	PrivateKey string `json:"private_key,omitempty"`
}

// ConnectionEnvironment represents a custom environment profile (production, staging, sandbox, etc.)
type ConnectionEnvironment struct {
	ID              string          `json:"id"`
	ConnectionID    string          `json:"connection_id"`
	EnvTag          string          `json:"env_tag"`
	DisplayName     string          `json:"display_name"`
	BadgeColor      string          `json:"badge_color"`
	EncryptedConfig []byte          `json:"-"`
	CachedSchema    json.RawMessage `json:"cached_schema"`
	IsDefault       bool            `json:"is_default"`
	LastSyncedAt    *time.Time      `json:"last_synced_at,omitempty"`
}

// SemanticModel represents a reusable virtual dataset / virtual view
type SemanticModel struct {
	ID           string          `json:"id"`
	WorkspaceID  string          `json:"workspace_id"`
	ProjectID    string          `json:"project_id"`
	Name         string          `json:"name"`
	Description  string          `json:"description"`
	RawSQL       string          `json:"raw_sql"`
	CachedSchema json.RawMessage `json:"cached_schema"`
	CreatedBy    *string         `json:"created_by,omitempty"`
	CreatedAt    time.Time       `json:"created_at"`
	UpdatedAt    time.Time       `json:"updated_at"`
}
