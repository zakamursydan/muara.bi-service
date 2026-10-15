package request

// CreateConnectionRequest holds parameters for creating a logical connection alias
type CreateConnectionRequest struct {
	AliasName  string `json:"alias_name"`
	Category   string `json:"category"`    // SQL, NOSQL, FILE, CLOUD_DW
	DriverType string `json:"driver_type"` // postgres, mysql, mongodb, sqlite
}

// UpsertEnvironmentRequest holds parameters for direct database connection credentials
type UpsertEnvironmentRequest struct {
	EnvTag      string `json:"env_tag"` // production, staging, sandbox, uat, etc.
	DisplayName string `json:"display_name"`
	BadgeColor  string `json:"badge_color"` // blue, green, red, yellow, purple
	IsDefault   bool   `json:"is_default"`

	// Direct Connect Fields (Default UI form)
	Host     string `json:"host"`
	Port     int    `json:"port"`
	Database string `json:"database"`
	User     string `json:"user"`
	Password string `json:"password"`
	SSLMode  string `json:"ssl_mode"`
	URI      string `json:"uri"`

	// Collapsible Optional SSH Bastion Jump Host
	SSHTunnelEnabled    bool   `json:"ssh_tunnel_enabled"`
	SSHHost             string `json:"ssh_host"`
	SSHPort             int    `json:"ssh_port"`
	SSHUser             string `json:"ssh_user"`
	SSHPassword         string `json:"ssh_password"`
	SSHPrivateKey       string `json:"ssh_private_key"`
}
