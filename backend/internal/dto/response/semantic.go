package response

import (
	"encoding/json"
	"time"
)

type SemanticModelResponse struct {
	ID           string          `json:"id"`
	WorkspaceID  string          `json:"workspace_id"`
	ProjectID    string          `json:"project_id"`
	Name         string          `json:"name"`
	Description  string          `json:"description"`
	RawSQL       string          `json:"raw_sql"`
	CachedSchema json.RawMessage `json:"cached_schema,omitempty"`
	CreatedAt    time.Time       `json:"created_at"`
	UpdatedAt    time.Time       `json:"updated_at"`
}
