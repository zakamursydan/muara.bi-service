package service

import (
	"context"
	"encoding/json"
	"fmt"
	"regexp"
	"strings"

	"muara.bi/core/internal/ai"
	"muara.bi/core/internal/config"
	"muara.bi/core/internal/domain/entity"
	"muara.bi/core/internal/dto/request"
	"muara.bi/core/internal/dto/response"
	"muara.bi/core/internal/duckdb"
	"muara.bi/core/internal/helper/cryptohelper"
	"muara.bi/core/internal/repository/pgsql"
	"muara.bi/core/pkg/customerror"
	"muara.bi/core/pkg/logging"
)

type AIServiceProvider interface {
	GetSettings(ctx context.Context, wsID string) (*response.AISettingsResponse, error)
	UpdateSettings(ctx context.Context, wsID string, req request.UpdateAISettingsRequest) (*response.AISettingsResponse, error)
	GenerateSQL(ctx context.Context, wsID string, req request.GenerateSQLRequest) (*response.GeneratedSQLResponse, error)
	GenerateDashboard(ctx context.Context, wsID string, req request.GenerateDashboardRequest) (*response.GeneratedDashboardResponse, error)
}

type AIService struct {
	aiRepo       pgsql.AIRepositoryProvider
	connRepo     pgsql.ConnectionRepositoryProvider
	semanticRepo pgsql.SemanticRepositoryProvider
	crypto       *cryptohelper.CryptoHelper
	cfg          *config.Config
	logger       *logging.Logger
}

func NewAIService(
	aiRepo pgsql.AIRepositoryProvider,
	connRepo pgsql.ConnectionRepositoryProvider,
	semanticRepo pgsql.SemanticRepositoryProvider,
	crypto *cryptohelper.CryptoHelper,
	cfg *config.Config,
	logger *logging.Logger,
) *AIService {
	return &AIService{
		aiRepo:       aiRepo,
		connRepo:     connRepo,
		semanticRepo: semanticRepo,
		crypto:       crypto,
		cfg:          cfg,
		logger:       logger,
	}
}

func (s *AIService) GetSettings(ctx context.Context, wsID string) (*response.AISettingsResponse, error) {
	settings, err := s.aiRepo.GetAISettingsByWorkspace(ctx, wsID)
	if err != nil {
		return nil, customerror.NewInternalError("failed to load ai settings", err)
	}

	if settings == nil {
		return &response.AISettingsResponse{
			WorkspaceID:     wsID,
			Provider:        entity.AIProviderAnthropic,
			HasAPIKey:       false,
			DefaultModel:    "claude-3-5-sonnet",
			FastModel:       "gemini-1.5-flash",
			Temperature:     0.2,
			MaxTokensBudget: 10000000,
			SchemaMasking:   []string{},
		}, nil
	}

	var masking []string
	_ = json.Unmarshal(settings.SchemaMasking, &masking)
	if masking == nil {
		masking = []string{}
	}

	return &response.AISettingsResponse{
		WorkspaceID:        settings.WorkspaceID,
		Provider:           settings.Provider,
		HasAPIKey:          len(settings.EncryptedAPIKey) > 0,
		BaseURL:            settings.BaseURL,
		DefaultModel:       settings.DefaultModel,
		FastModel:          settings.FastModel,
		Temperature:        settings.Temperature,
		MaxTokensBudget:    settings.MaxTokensBudget,
		CustomInstructions: settings.CustomInstructions,
		SchemaMasking:      masking,
	}, nil
}

func (s *AIService) UpdateSettings(ctx context.Context, wsID string, req request.UpdateAISettingsRequest) (*response.AISettingsResponse, error) {
	provider := strings.ToLower(strings.TrimSpace(req.Provider))
	if provider == "" {
		provider = entity.AIProviderAnthropic
	}

	defaultModel := strings.TrimSpace(req.DefaultModel)
	if defaultModel == "" {
		defaultModel = "claude-3-5-sonnet"
	}

	fastModel := strings.TrimSpace(req.FastModel)
	if fastModel == "" {
		fastModel = "gemini-1.5-flash"
	}

	temp := 0.2
	if req.Temperature != nil {
		temp = *req.Temperature
		if temp < 0 {
			temp = 0
		} else if temp > 1.0 {
			temp = 1.0
		}
	}

	budget := 10000000
	if req.MaxTokensBudget != nil && *req.MaxTokensBudget > 0 {
		budget = *req.MaxTokensBudget
	}

	var encryptedKey []byte
	if strings.TrimSpace(req.APIKey) != "" {
		var err error
		encryptedKey, err = s.crypto.Encrypt([]byte(strings.TrimSpace(req.APIKey)))
		if err != nil {
			return nil, customerror.NewInternalError("failed to encrypt api key", err)
		}
	}

	maskingBytes, _ := json.Marshal(req.SchemaMasking)

	settings := &entity.AISettings{
		WorkspaceID:        wsID,
		Provider:           provider,
		EncryptedAPIKey:    encryptedKey,
		BaseURL:            strings.TrimSpace(req.BaseURL),
		DefaultModel:       defaultModel,
		FastModel:          fastModel,
		Temperature:        temp,
		MaxTokensBudget:    budget,
		CustomInstructions: req.CustomInstructions,
		SchemaMasking:      maskingBytes,
	}

	if err := s.aiRepo.UpsertAISettings(ctx, settings); err != nil {
		return nil, customerror.NewInternalError("failed to update ai settings", err)
	}

	return s.GetSettings(ctx, wsID)
}

func (s *AIService) GenerateSQL(ctx context.Context, wsID string, req request.GenerateSQLRequest) (*response.GeneratedSQLResponse, error) {
	prompt := strings.TrimSpace(req.Prompt)
	if prompt == "" {
		return nil, customerror.NewBadRequestError("prompt is required", nil)
	}

	// 1. Resolve client and settings
	client, settings, err := s.resolveClient(ctx, wsID, req.ModelPreference)
	if err != nil {
		return nil, err
	}

	// 2. Load schemas and prune/mask
	tables := s.collectWorkspaceSchemas(ctx, wsID, req.EnvTag, req.ProjectID)
	var maskList []string
	if settings != nil {
		_ = json.Unmarshal(settings.SchemaMasking, &maskList)
	}

	selectedTables, ddlContext := ai.PruneAndMaskSchema(prompt, tables, maskList, 8)

	var relevantNames []string
	for _, t := range selectedTables {
		relevantNames = append(relevantNames, t.TableName)
	}

	// 3. Formulate system prompt
	systemPrompt := fmt.Sprintf(`You are the AI Co-Pilot for muara.bi, an enterprise BI platform powered by DuckDB SQL.
Your goal is to generate clean, valid DuckDB SQL queries matching the user's intent.

CRITICAL RULES:
1. ONLY produce SELECT statements. NEVER generate DROP, DELETE, UPDATE, INSERT, ALTER, TRUNCATE, or PRAGMA statements.
2. DuckDB supports PostgreSQL syntax, CTEs, window functions, and aggregation.
3. The available schema metadata (sanitized, PII columns removed) is:
%s

%s

4. Return ONLY the SQL statement wrapped inside a single `+"```sql ... ```"+` code block. No additional markdown outside the block.`,
		ddlContext,
		s.formatCustomInstructions(settings),
	)

	// 4. Generate with LLM
	completion, err := client.GenerateCompletion(ctx, systemPrompt, prompt, 1024)
	if err != nil {
		return nil, customerror.NewInternalError("failed to generate sql from ai model", err)
	}

	rawSQL := extractSQL(completion)
	if rawSQL == "" {
		rawSQL = strings.TrimSpace(completion)
	}

	// 5. Enforce SQL guardrails
	if err := duckdb.ValidateQuery(rawSQL); err != nil {
		return nil, customerror.NewBadRequestError(fmt.Sprintf("AI-generated query was rejected by guardrails: %v", err), nil)
	}

	return &response.GeneratedSQLResponse{
		Prompt:         prompt,
		RawSQL:         rawSQL,
		Explanation:    fmt.Sprintf("Generated based on %d relevant tables: %s", len(relevantNames), strings.Join(relevantNames, ", ")),
		RelevantTables: relevantNames,
	}, nil
}

func (s *AIService) GenerateDashboard(ctx context.Context, wsID string, req request.GenerateDashboardRequest) (*response.GeneratedDashboardResponse, error) {
	prompt := strings.TrimSpace(req.Prompt)
	if prompt == "" {
		return nil, customerror.NewBadRequestError("prompt is required", nil)
	}

	client, settings, err := s.resolveClient(ctx, wsID, "default")
	if err != nil {
		return nil, err
	}

	tables := s.collectWorkspaceSchemas(ctx, wsID, req.EnvTag, req.ProjectID)
	var maskList []string
	if settings != nil {
		_ = json.Unmarshal(settings.SchemaMasking, &maskList)
	}

	_, ddlContext := ai.PruneAndMaskSchema(prompt, tables, maskList, 8)

	systemPrompt := fmt.Sprintf(`You are the AI Co-Pilot for muara.bi. Mode: DASHBOARD_JSON.
Generate a complete dashboard specification in JSON for the user's request.

CRITICAL RULES:
1. Return valid JSON only with this structure:
{
  "title": "string",
  "description": "string",
  "widgets": [
    {
      "title": "string",
      "chart_type": "kpi" | "line" | "bar" | "pie" | "table",
      "raw_sql": "SELECT ... FROM ...",
      "echarts_config": {"x_axis": "column_name", "y_axis": "metric_name"}
    }
  ]
}
2. Include 2 to 4 widgets that give a complete analytical picture.
3. Every raw_sql MUST be a safe DuckDB SELECT query. NEVER output DDL/DML.
4. Available sanitized schemas:
%s

%s`,
		ddlContext,
		s.formatCustomInstructions(settings),
	)

	completion, err := client.GenerateCompletion(ctx, systemPrompt, prompt, 2048)
	if err != nil {
		return nil, customerror.NewInternalError("failed to generate dashboard from ai model", err)
	}

	cleanedJSON := extractJSON(completion)
	var dashResp response.GeneratedDashboardResponse
	if err := json.Unmarshal([]byte(cleanedJSON), &dashResp); err != nil {
		return nil, customerror.NewInternalError(fmt.Sprintf("failed to parse ai dashboard json: %v", err), nil)
	}

	// Validate each widget SQL
	var safeWidgets []response.SuggestedWidgetResponse
	for _, w := range dashResp.Widgets {
		if err := duckdb.ValidateQuery(w.RawSQL); err == nil {
			if w.EchartsConfig == nil {
				w.EchartsConfig = map[string]any{}
			}
			safeWidgets = append(safeWidgets, w)
		}
	}
	dashResp.Widgets = safeWidgets

	if dashResp.Title == "" {
		if req.Title != "" {
			dashResp.Title = req.Title
		} else {
			dashResp.Title = "Analytics Dashboard"
		}
	}

	return &dashResp, nil
}

func (s *AIService) resolveClient(ctx context.Context, wsID, preference string) (ai.LLMClient, *entity.AISettings, error) {
	settings, _ := s.aiRepo.GetAISettingsByWorkspace(ctx, wsID)

	provider := entity.AIProviderAnthropic
	model := "claude-3-5-sonnet"
	baseURL := ""
	temp := 0.2
	var apiKey string

	if settings != nil {
		provider = settings.Provider
		baseURL = settings.BaseURL
		temp = settings.Temperature

		if strings.ToLower(preference) == "fast" && settings.FastModel != "" {
			model = settings.FastModel
		} else if settings.DefaultModel != "" {
			model = settings.DefaultModel
		}

		if len(settings.EncryptedAPIKey) > 0 {
			decrypted, err := s.crypto.Decrypt(settings.EncryptedAPIKey)
			if err == nil {
				apiKey = string(decrypted)
			}
		}
	}

	return ai.NewClient(ai.ClientConfig{
		Provider:    provider,
		APIKey:      apiKey,
		BaseURL:     baseURL,
		Model:       model,
		Temperature: temp,
	}), settings, nil
}

func (s *AIService) collectWorkspaceSchemas(ctx context.Context, wsID, envTag, projID string) []ai.TableSchema {
	var list []ai.TableSchema

	conns, err := s.connRepo.ListConnections(ctx, wsID)
	if err == nil {
		for _, c := range conns {
			env, err := s.connRepo.GetEnvironment(ctx, c.ID, envTag)
			if err == nil && env != nil && len(env.CachedSchema) > 0 {
				var tableMap map[string][]struct {
					Name string `json:"name"`
					Type string `json:"type"`
				}
				if err := json.Unmarshal(env.CachedSchema, &tableMap); err == nil {
					for tblName, cols := range tableMap {
						var colInfos []ai.ColumnInfo
						for _, col := range cols {
							colInfos = append(colInfos, ai.ColumnInfo{
								Name: col.Name,
								Type: col.Type,
							})
						}
						list = append(list, ai.TableSchema{
							AliasName: c.AliasName,
							TableName: tblName,
							Columns:   colInfos,
						})
					}
				}
			}
		}
	}

	// Also add semantic models if project specified
	if projID != "" {
		smModels, err := s.semanticRepo.ListSemanticModelsByProject(ctx, wsID, projID)
		if err == nil {
			for _, sm := range smModels {
				list = append(list, ai.TableSchema{
					AliasName: "",
					TableName: sm.Name,
					Columns: []ai.ColumnInfo{
						{Name: "id", Type: "INT"},
						{Name: "value", Type: "NUMERIC"},
					},
				})
			}
		}
	}

	return list
}

func (s *AIService) formatCustomInstructions(settings *entity.AISettings) string {
	if settings == nil || strings.TrimSpace(settings.CustomInstructions) == "" {
		return ""
	}
	return "Workspace Business Glossary & Instructions:\n" + settings.CustomInstructions
}

var sqlBlockRegex = regexp.MustCompile("(?s)```(?:sql)?\\s*(.*?)\\s*```")

func extractSQL(raw string) string {
	matches := sqlBlockRegex.FindStringSubmatch(raw)
	if len(matches) > 1 {
		return strings.TrimSpace(matches[1])
	}
	return strings.TrimSpace(raw)
}

var jsonBlockRegex = regexp.MustCompile("(?s)```(?:json)?\\s*(.*?)\\s*```")

func extractJSON(raw string) string {
	matches := jsonBlockRegex.FindStringSubmatch(raw)
	if len(matches) > 1 {
		return strings.TrimSpace(matches[1])
	}
	return strings.TrimSpace(raw)
}
