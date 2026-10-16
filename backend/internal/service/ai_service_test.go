package service

import (
	"context"
	"testing"

	"muara.bi/core/internal/config"
	"muara.bi/core/internal/domain/entity"
	"muara.bi/core/internal/dto/request"
	"muara.bi/core/internal/helper/cryptohelper"
	"muara.bi/core/pkg/customerror"
	"muara.bi/core/pkg/logging"
)

type mockAIRepo struct {
	settings map[string]*entity.AISettings
}

func (m *mockAIRepo) GetAISettingsByWorkspace(ctx context.Context, wsID string) (*entity.AISettings, error) {
	if s, ok := m.settings[wsID]; ok {
		return s, nil
	}
	return nil, nil
}

func (m *mockAIRepo) UpsertAISettings(ctx context.Context, settings *entity.AISettings) error {
	if m.settings == nil {
		m.settings = make(map[string]*entity.AISettings)
	}
	m.settings[settings.WorkspaceID] = settings
	return nil
}

type mockConnRepoForAI struct {
	connections []entity.Connection
}

func (m *mockConnRepoForAI) CreateConnection(ctx context.Context, c *entity.Connection) error {
	return nil
}
func (m *mockConnRepoForAI) GetConnectionByID(ctx context.Context, wsID, id string) (*entity.Connection, error) {
	return nil, nil
}
func (m *mockConnRepoForAI) ListConnections(ctx context.Context, wsID string) ([]entity.Connection, error) {
	return m.connections, nil
}
func (m *mockConnRepoForAI) UpdateConnection(ctx context.Context, c *entity.Connection) error {
	return nil
}
func (m *mockConnRepoForAI) DeleteConnection(ctx context.Context, wsID, id string) error {
	return nil
}
func (m *mockConnRepoForAI) UpsertEnvironment(ctx context.Context, env *entity.ConnectionEnvironment) error {
	return nil
}
func (m *mockConnRepoForAI) GetEnvironment(ctx context.Context, connID, envTag string) (*entity.ConnectionEnvironment, error) {
	return nil, nil
}
func (m *mockConnRepoForAI) ListEnvironments(ctx context.Context, connID string) ([]entity.ConnectionEnvironment, error) {
	return nil, nil
}
func (m *mockConnRepoForAI) UpdateCachedSchema(ctx context.Context, envID string, schemaJSON []byte) error {
	return nil
}

type mockSemanticRepoForAI struct{}

func (m *mockSemanticRepoForAI) CreateSemanticModel(ctx context.Context, sm *entity.SemanticModel) error {
	return nil
}
func (m *mockSemanticRepoForAI) GetSemanticModelByID(ctx context.Context, wsID, id string) (*entity.SemanticModel, error) {
	return nil, nil
}
func (m *mockSemanticRepoForAI) ListSemanticModelsByProject(ctx context.Context, wsID, projID string) ([]entity.SemanticModel, error) {
	return nil, nil
}
func (m *mockSemanticRepoForAI) UpdateSemanticModel(ctx context.Context, sm *entity.SemanticModel) error {
	return nil
}
func (m *mockSemanticRepoForAI) DeleteSemanticModel(ctx context.Context, wsID, id string) error {
	return nil
}

func setupAIServiceTest(t *testing.T) (*AIService, *mockAIRepo) {
	t.Helper()
	crypto, err := cryptohelper.NewCryptoHelper("0123456789abcdef0123456789abcdef")
	if err != nil {
		t.Fatalf("failed to init crypto helper: %v", err)
	}

	aiRepo := &mockAIRepo{settings: make(map[string]*entity.AISettings)}
	connRepo := &mockConnRepoForAI{}
	semanticRepo := &mockSemanticRepoForAI{}
	cfg := &config.Config{
		AIDefaultProvider: "anthropic",
		AIDefaultModel:    "claude-3-5-sonnet",
		AIFastModel:       "gemini-1.5-flash",
	}
	logger := logging.NewLogger("test", "error")

	svc := NewAIService(aiRepo, connRepo, semanticRepo, crypto, cfg, logger)
	return svc, aiRepo
}

func TestAIService_GetSettingsDefault(t *testing.T) {
	svc, _ := setupAIServiceTest(t)
	ctx := context.Background()

	res, err := svc.GetSettings(ctx, "ws-123")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if res.WorkspaceID != "ws-123" {
		t.Errorf("expected ws-123, got %s", res.WorkspaceID)
	}
	if res.Provider != entity.AIProviderAnthropic {
		t.Errorf("expected default provider anthropic, got %s", res.Provider)
	}
	if res.HasAPIKey {
		t.Errorf("expected HasAPIKey=false for default unconfigured settings")
	}
}

func TestAIService_UpdateAndGetSettings(t *testing.T) {
	svc, _ := setupAIServiceTest(t)
	ctx := context.Background()

	temp := 0.5
	budget := 5000000
	req := request.UpdateAISettingsRequest{
		Provider:           entity.AIProviderOpenAI,
		APIKey:             "sk-test-secret-key-12345",
		DefaultModel:       "gpt-4o",
		FastModel:          "gpt-4o-mini",
		Temperature:        &temp,
		MaxTokensBudget:    &budget,
		CustomInstructions: "Always prefer CTE over subqueries",
		SchemaMasking:      []string{"secret_code", "ssn"},
	}

	res, err := svc.UpdateSettings(ctx, "ws-123", req)
	if err != nil {
		t.Fatalf("unexpected error updating settings: %v", err)
	}

	if res.Provider != entity.AIProviderOpenAI {
		t.Errorf("expected provider openai, got %s", res.Provider)
	}
	if !res.HasAPIKey {
		t.Errorf("expected HasAPIKey=true after providing key")
	}
	if res.DefaultModel != "gpt-4o" {
		t.Errorf("expected default model gpt-4o, got %s", res.DefaultModel)
	}
	if len(res.SchemaMasking) != 2 {
		t.Errorf("expected 2 schema masking patterns, got %d", len(res.SchemaMasking))
	}

	// Verify GetSettings reflects it
	fetched, err := svc.GetSettings(ctx, "ws-123")
	if err != nil {
		t.Fatalf("failed to fetch updated settings: %v", err)
	}
	if !fetched.HasAPIKey {
		t.Errorf("expected persisted HasAPIKey=true")
	}
	if fetched.CustomInstructions != "Always prefer CTE over subqueries" {
		t.Errorf("expected custom instructions preserved, got %s", fetched.CustomInstructions)
	}
}

func TestAIService_GenerateSQL_RejectsEmptyPrompt(t *testing.T) {
	svc, _ := setupAIServiceTest(t)
	ctx := context.Background()

	_, err := svc.GenerateSQL(ctx, "ws-123", request.GenerateSQLRequest{
		Prompt: "   ",
	})
	if err == nil {
		t.Fatalf("expected error for empty prompt")
	}

	appErr, ok := err.(*customerror.AppError)
	if !ok || appErr.Code != 400 {
		t.Errorf("expected 400 BadRequest error, got %v", err)
	}
}

func TestAIService_GenerateSQL_SuccessWithMock(t *testing.T) {
	svc, _ := setupAIServiceTest(t)
	ctx := context.Background()

	res, err := svc.GenerateSQL(ctx, "ws-123", request.GenerateSQLRequest{
		Prompt: "Show total transactions",
		EnvTag: "development",
	})
	if err != nil {
		t.Fatalf("unexpected error generating sql: %v", err)
	}

	if res.RawSQL == "" {
		t.Errorf("expected non-empty RawSQL in response")
	}
	if res.Prompt != "Show total transactions" {
		t.Errorf("expected prompt preserved, got %s", res.Prompt)
	}
}

func TestAIService_GenerateDashboard_SuccessWithMock(t *testing.T) {
	svc, _ := setupAIServiceTest(t)
	ctx := context.Background()

	res, err := svc.GenerateDashboard(ctx, "ws-123", request.GenerateDashboardRequest{
		Prompt: "Create revenue overview dashboard",
		Title:  "Financial Dashboard",
		EnvTag: "development",
	})
	if err != nil {
		t.Fatalf("unexpected error generating dashboard: %v", err)
	}

	if res.Title == "" {
		t.Errorf("expected dashboard title")
	}
	if len(res.Widgets) == 0 {
		t.Errorf("expected suggested widgets in response")
	}
	for _, w := range res.Widgets {
		if w.RawSQL == "" || w.ChartType == "" {
			t.Errorf("invalid widget generated: %+v", w)
		}
	}
}
