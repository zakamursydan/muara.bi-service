package entity

import (
	"encoding/json"
	"time"
)

const (
	AIProviderAnthropic = "anthropic"
	AIProviderOpenAI    = "openai"
	AIProviderGemini    = "gemini"
	AIProviderAzure     = "azure_openai"
	AIProviderOllama    = "ollama"
)

// AISettings stores workspace-level LLM preferences and encrypted API credentials
type AISettings struct {
	ID                 string          `json:"id"`
	WorkspaceID        string          `json:"workspace_id"`
	Provider           string          `json:"provider"`
	EncryptedAPIKey    []byte          `json:"-"`
	BaseURL            string          `json:"base_url,omitempty"`
	DefaultModel       string          `json:"default_model"`
	FastModel          string          `json:"fast_model"`
	Temperature        float64         `json:"temperature"`
	MaxTokensBudget    int             `json:"max_tokens_budget"`
	CustomInstructions string          `json:"custom_instructions,omitempty"`
	SchemaMasking      json.RawMessage `json:"schema_masking"` // Array of column names to mask: ["phone", "salary"]
	CreatedAt          time.Time       `json:"created_at"`
	UpdatedAt          time.Time       `json:"updated_at"`
}
