package duckdb

import (
	"context"
	"fmt"
	"strings"

	"muara.bi/core/internal/helper/sqlident"
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

		// RegisterTempView interpolates the name into DDL, so only a plain
		// identifier may reach it.
		if !sqlident.IsPlain(cleanName) {
			return fmt.Errorf("invalid semantic model name %q: use letters, digits and underscores", cleanName)
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
