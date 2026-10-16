package pgsql

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"muara.bi/core/internal/domain/entity"
)

type AIRepositoryProvider interface {
	GetAISettingsByWorkspace(ctx context.Context, wsID string) (*entity.AISettings, error)
	UpsertAISettings(ctx context.Context, settings *entity.AISettings) error
}

type AIRepository struct {
	db *sql.DB
}

func NewAIRepository(db *sql.DB) *AIRepository {
	return &AIRepository{db: db}
}

func (r *AIRepository) GetAISettingsByWorkspace(ctx context.Context, wsID string) (*entity.AISettings, error) {
	query := `
		SELECT id, workspace_id, provider, encrypted_api_key, base_url,
		       default_model, fast_model, temperature, max_tokens_budget,
		       custom_instructions, schema_masking, created_at, updated_at
		FROM ai_settings
		WHERE workspace_id = $1`

	var s entity.AISettings
	var encryptedKey []byte
	var baseURL, customInstructions sql.NullString
	var schemaMaskingStr string

	err := r.db.QueryRowContext(ctx, query, wsID).Scan(
		&s.ID,
		&s.WorkspaceID,
		&s.Provider,
		&encryptedKey,
		&baseURL,
		&s.DefaultModel,
		&s.FastModel,
		&s.Temperature,
		&s.MaxTokensBudget,
		&customInstructions,
		&schemaMaskingStr,
		&s.CreatedAt,
		&s.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to query ai settings: %w", err)
	}

	s.EncryptedAPIKey = encryptedKey
	if baseURL.Valid {
		s.BaseURL = baseURL.String
	}
	if customInstructions.Valid {
		s.CustomInstructions = customInstructions.String
	}
	s.SchemaMasking = []byte(schemaMaskingStr)

	return &s, nil
}

func (r *AIRepository) UpsertAISettings(ctx context.Context, s *entity.AISettings) error {
	query := `
		INSERT INTO ai_settings (
			workspace_id, provider, encrypted_api_key, base_url,
			default_model, fast_model, temperature, max_tokens_budget,
			custom_instructions, schema_masking, updated_at
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, NOW())
		ON CONFLICT (workspace_id) DO UPDATE SET
			provider = EXCLUDED.provider,
			encrypted_api_key = COALESCE(NULLIF(EXCLUDED.encrypted_api_key, ''::bytea), ai_settings.encrypted_api_key),
			base_url = EXCLUDED.base_url,
			default_model = EXCLUDED.default_model,
			fast_model = EXCLUDED.fast_model,
			temperature = EXCLUDED.temperature,
			max_tokens_budget = EXCLUDED.max_tokens_budget,
			custom_instructions = EXCLUDED.custom_instructions,
			schema_masking = EXCLUDED.schema_masking,
			updated_at = NOW()
		RETURNING id, created_at, updated_at`

	return r.db.QueryRowContext(
		ctx, query,
		s.WorkspaceID,
		s.Provider,
		s.EncryptedAPIKey,
		s.BaseURL,
		s.DefaultModel,
		s.FastModel,
		s.Temperature,
		s.MaxTokensBudget,
		s.CustomInstructions,
		string(s.SchemaMasking),
	).Scan(&s.ID, &s.CreatedAt, &s.UpdatedAt)
}
