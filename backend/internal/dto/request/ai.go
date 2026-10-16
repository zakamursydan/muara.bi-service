package request

type UpdateAISettingsRequest struct {
	Provider           string   `json:"provider"`
	APIKey             string   `json:"api_key,omitempty"` // Write-only
	BaseURL            string   `json:"base_url,omitempty"`
	DefaultModel       string   `json:"default_model"`
	FastModel          string   `json:"fast_model"`
	Temperature        *float64 `json:"temperature,omitempty"`
	MaxTokensBudget    *int     `json:"max_tokens_budget,omitempty"`
	CustomInstructions string   `json:"custom_instructions,omitempty"`
	SchemaMasking      []string `json:"schema_masking"`
}

type GenerateSQLRequest struct {
	ProjectID       string `json:"project_id,omitempty"`
	EnvTag          string `json:"env_tag,omitempty"`
	Prompt          string `json:"prompt"`
	ModelPreference string `json:"model_preference,omitempty"` // "default" or "fast"
}

type GenerateDashboardRequest struct {
	ProjectID string `json:"project_id,omitempty"`
	EnvTag    string `json:"env_tag,omitempty"`
	Prompt    string `json:"prompt"`
	Title     string `json:"title,omitempty"`
}
