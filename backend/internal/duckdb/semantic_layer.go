package duckdb

import (
	"context"
	"fmt"
	"strings"
)

// SemanticModel represents a reusable virtual view/saved dataset
type SemanticModel struct {
	ID          string `json:"id"`
	WorkspaceID string `json:"workspace_id"`
	ProjectID   string `json:"project_id"`
	Name        string `json:"name"`
	Description string `json:"description"`
	RawSQL      string `json:"raw_sql"`
}

// MountSemanticModels registers a list of semantic models into the DuckDB session as temporary views
func MountSemanticModels(ctx context.Context, session Session, models []SemanticModel) error {
	for _, model := range models {
		cleanName := strings.TrimSpace(model.Name)
		if cleanName == "" {
			continue
		}

		// Ensure the model SQL is safe
		if err := ValidateQuery(model.RawSQL); err != nil {
			return fmt.Errorf("invalid semantic model %q: %w", cleanName, err)
		}

		cleanSQL := strings.TrimSuffix(strings.TrimSpace(model.RawSQL), ";")
		if err := session.RegisterTempView(ctx, cleanName, cleanSQL); err != nil {
			return fmt.Errorf("failed to mount semantic model %q: %w", cleanName, err)
		}
	}
	return nil
}
