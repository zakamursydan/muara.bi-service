package response

// AccessResponse is the caller's resolved authorization in one workspace, and
// optionally one project inside it. The frontend uses it to hide actions the
// server would reject anyway.
type AccessResponse struct {
	WorkspaceID   string   `json:"workspace_id"`
	ProjectID     string   `json:"project_id,omitempty"`
	WorkspaceRole string   `json:"workspace_role"`
	ProjectRole   string   `json:"project_role,omitempty"`
	Permissions   []string `json:"permissions"`
	RLSColumns    []string `json:"rls_columns"`
}
