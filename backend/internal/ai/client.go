package ai

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"
)

type LLMClient interface {
	GenerateCompletion(ctx context.Context, systemPrompt, userPrompt string, maxTokens int) (string, error)
}

// ClientConfig holds resolved LLM parameters
type ClientConfig struct {
	Provider    string
	APIKey      string
	BaseURL     string
	Model       string
	Temperature float64
}

// NewClient returns an LLM client adapted to the provider
func NewClient(cfg ClientConfig) LLMClient {
	httpClient := &http.Client{Timeout: 45 * time.Second}

	if cfg.APIKey == "" && cfg.BaseURL == "" {
		return &mockLLMClient{cfg: cfg}
	}

	switch cfg.Provider {
	case "anthropic":
		return &anthropicClient{cfg: cfg, http: httpClient}
	case "gemini":
		return &geminiClient{cfg: cfg, http: httpClient}
	default:
		// Default to OpenAI / Ollama / Azure compatible chat completion API
		return &openAICompatibleClient{cfg: cfg, http: httpClient}
	}
}

// ─── OpenAI / Ollama Compatible Client ─────────────────────────────────────────

type openAICompatibleClient struct {
	cfg  ClientConfig
	http *http.Client
}

func (c *openAICompatibleClient) GenerateCompletion(ctx context.Context, systemPrompt, userPrompt string, maxTokens int) (string, error) {
	url := c.cfg.BaseURL
	if url == "" {
		url = "https://api.openai.com/v1"
	}
	url = strings.TrimSuffix(url, "/") + "/chat/completions"

	if maxTokens <= 0 {
		maxTokens = 2048
	}
	model := c.cfg.Model
	if model == "" {
		model = "gpt-4o-mini"
	}

	payload := map[string]any{
		"model": model,
		"messages": []map[string]string{
			{"role": "system", "content": systemPrompt},
			{"role": "user", "content": userPrompt},
		},
		"max_tokens":  maxTokens,
		"temperature": c.cfg.Temperature,
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return "", err
	}

	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewBuffer(body))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")
	if c.cfg.APIKey != "" {
		req.Header.Set("Authorization", "Bearer "+c.cfg.APIKey)
	}

	resp, err := c.http.Do(req)
	if err != nil {
		return "", fmt.Errorf("llm request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		var errResp map[string]any
		_ = json.NewDecoder(resp.Body).Decode(&errResp)
		return "", fmt.Errorf("llm returned status %d: %v", resp.StatusCode, errResp)
	}

	var res struct {
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		} `json:"choices"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&res); err != nil {
		return "", fmt.Errorf("failed to decode llm response: %w", err)
	}

	if len(res.Choices) == 0 {
		return "", fmt.Errorf("empty completion from llm")
	}

	return res.Choices[0].Message.Content, nil
}

// ─── Anthropic Claude Client ──────────────────────────────────────────────────

type anthropicClient struct {
	cfg  ClientConfig
	http *http.Client
}

func (c *anthropicClient) GenerateCompletion(ctx context.Context, systemPrompt, userPrompt string, maxTokens int) (string, error) {
	url := c.cfg.BaseURL
	if url == "" {
		url = "https://api.anthropic.com/v1"
	}
	url = strings.TrimSuffix(url, "/") + "/messages"

	if maxTokens <= 0 {
		maxTokens = 2048
	}
	model := c.cfg.Model
	if model == "" {
		model = "claude-3-5-sonnet-20241022"
	}

	payload := map[string]any{
		"model":      model,
		"system":     systemPrompt,
		"max_tokens": maxTokens,
		"messages": []map[string]string{
			{"role": "user", "content": userPrompt},
		},
		"temperature": c.cfg.Temperature,
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return "", err
	}

	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewBuffer(body))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("x-api-key", c.cfg.APIKey)
	req.Header.Set("anthropic-version", "2023-06-01")

	resp, err := c.http.Do(req)
	if err != nil {
		return "", fmt.Errorf("anthropic request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		var errResp map[string]any
		_ = json.NewDecoder(resp.Body).Decode(&errResp)
		return "", fmt.Errorf("anthropic returned status %d: %v", resp.StatusCode, errResp)
	}

	var res struct {
		Content []struct {
			Text string `json:"text"`
		} `json:"content"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&res); err != nil {
		return "", fmt.Errorf("failed to decode anthropic response: %w", err)
	}

	if len(res.Content) == 0 {
		return "", fmt.Errorf("empty completion from anthropic")
	}

	return res.Content[0].Text, nil
}

// ─── Google Gemini Client ─────────────────────────────────────────────────────

type geminiClient struct {
	cfg  ClientConfig
	http *http.Client
}

func (c *geminiClient) GenerateCompletion(ctx context.Context, systemPrompt, userPrompt string, maxTokens int) (string, error) {
	model := c.cfg.Model
	if model == "" {
		model = "gemini-1.5-flash"
	}

	url := fmt.Sprintf("https://generativelanguage.googleapis.com/v1beta/models/%s:generateContent?key=%s", model, c.cfg.APIKey)

	payload := map[string]any{
		"system_instruction": map[string]any{
			"parts": []map[string]string{{"text": systemPrompt}},
		},
		"contents": []map[string]any{
			{
				"parts": []map[string]string{{"text": userPrompt}},
			},
		},
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return "", err
	}

	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewBuffer(body))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.http.Do(req)
	if err != nil {
		return "", fmt.Errorf("gemini request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		var errResp map[string]any
		_ = json.NewDecoder(resp.Body).Decode(&errResp)
		return "", fmt.Errorf("gemini returned status %d: %v", resp.StatusCode, errResp)
	}

	var res struct {
		Candidates []struct {
			Content struct {
				Parts []struct {
					Text string `json:"text"`
				} `json:"parts"`
			} `json:"content"`
		} `json:"candidates"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&res); err != nil {
		return "", fmt.Errorf("failed to decode gemini response: %w", err)
	}

	if len(res.Candidates) == 0 || len(res.Candidates[0].Content.Parts) == 0 {
		return "", fmt.Errorf("empty completion from gemini")
	}

	return res.Candidates[0].Content.Parts[0].Text, nil
}

// ─── Mock / Offline Simulation Client ─────────────────────────────────────────

type mockLLMClient struct {
	cfg ClientConfig
}

func (m *mockLLMClient) GenerateCompletion(ctx context.Context, systemPrompt, userPrompt string, maxTokens int) (string, error) {
	// If asking for a dashboard JSON:
	if strings.Contains(systemPrompt, "DASHBOARD_JSON") {
		return `{
  "title": "Analytical Overview",
  "description": "Generated analytical dashboard based on your query",
  "widgets": [
    {
      "title": "Total Metric Summary",
      "chart_type": "kpi",
      "raw_sql": "SELECT COUNT(*) as total_count FROM (SELECT 1) LIMIT 1",
      "echarts_config": {"metric_field": "total_count"}
    },
    {
      "title": "Trend Over Time",
      "chart_type": "line",
      "raw_sql": "SELECT '2026-01-01' as date, 100 as value UNION ALL SELECT '2026-02-01', 150 LIMIT 10",
      "echarts_config": {"x_axis": "date", "y_axis": "value"}
    }
  ]
}`, nil
	}

	// Default SQL output
	return "SELECT 1 AS generated_metric, 'simulated' AS status LIMIT 100", nil
}
