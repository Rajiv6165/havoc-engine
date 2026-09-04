package postmortem

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"text/template"
	"time"

	_ "embed"
)

//go:embed prompt.tmpl
var promptTemplateStr string

// ExperimentData contains all the context needed for the LLM to generate a report.
type ExperimentData struct {
	ExperimentType   string
	TargetNamespace  string
	RecoveryTimeMs   *int
	ErrorRatePercent *float64
	SafetyIntervened bool
	ResilienceScore  int
}

// Generator defines the interface for generating postmortem reports.
type Generator interface {
	GenerateReport(ctx context.Context, data ExperimentData) (string, error)
}

// ClaudeGenerator implements the Generator interface using Anthropic's Claude API.
type ClaudeGenerator struct {
	APIKey     string
	HTTPClient *http.Client
	Template   *template.Template
}

// NewClaudeGenerator creates a new ClaudeGenerator.
func NewClaudeGenerator(apiKey string) (*ClaudeGenerator, error) {
	tmpl, err := template.New("prompt").Parse(promptTemplateStr)
	if err != nil {
		return nil, fmt.Errorf("failed to parse prompt template: %w", err)
	}

	return &ClaudeGenerator{
		APIKey: apiKey,
		HTTPClient: &http.Client{
			Timeout: 15 * time.Second,
		},
		Template: tmpl,
	}, nil
}

// anthropicRequest represents the request body for the Claude API.
type anthropicRequest struct {
	Model     string    `json:"model"`
	MaxTokens int       `json:"max_tokens"`
	Messages  []message `json:"messages"`
}

type message struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

// anthropicResponse represents the response body from the Claude API.
type anthropicResponse struct {
	Content []struct {
		Text string `json:"text"`
	} `json:"content"`
	Error *struct {
		Message string `json:"message"`
	} `json:"error,omitempty"`
}

// GenerateReport calls the Anthropic API to generate a postmortem report.
func (g *ClaudeGenerator) GenerateReport(ctx context.Context, data ExperimentData) (string, error) {
	if g.APIKey == "" {
		return "", fmt.Errorf("Anthropic API key is not set")
	}

	var promptBuf bytes.Buffer
	if err := g.Template.Execute(&promptBuf, data); err != nil {
		return "", fmt.Errorf("failed to execute prompt template: %w", err)
	}

	reqBody := anthropicRequest{
		Model:     "claude-3-haiku-20240307",
		MaxTokens: 500,
		Messages: []message{
			{
				Role:    "user",
				Content: promptBuf.String(),
			},
		},
	}

	reqBytes, err := json.Marshal(reqBody)
	if err != nil {
		return "", fmt.Errorf("failed to marshal request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, "POST", "https://api.anthropic.com/v1/messages", bytes.NewReader(reqBytes))
	if err != nil {
		return "", fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("x-api-key", g.APIKey)
	req.Header.Set("anthropic-version", "2023-06-01")
	req.Header.Set("content-type", "application/json")

	resp, err := g.HTTPClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("HTTP request failed: %w", err)
	}
	defer resp.Body.Close()

	respBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("failed to read response body: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		var errResp anthropicResponse
		if err := json.Unmarshal(respBytes, &errResp); err == nil && errResp.Error != nil {
			return "", fmt.Errorf("API error (%d): %s", resp.StatusCode, errResp.Error.Message)
		}
		return "", fmt.Errorf("API error (%d): %s", resp.StatusCode, string(respBytes))
	}

	var apiResp anthropicResponse
	if err := json.Unmarshal(respBytes, &apiResp); err != nil {
		return "", fmt.Errorf("failed to decode response: %w", err)
	}

	if len(apiResp.Content) == 0 {
		return "", fmt.Errorf("empty content in response")
	}

	return apiResp.Content[0].Text, nil
}
