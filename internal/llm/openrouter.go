package llm

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"time"
)

const (
	openRouterBaseURL        = "https://openrouter.ai/api/v1/chat/completions"
	openRouterModel          = "anthropic/claude-sonnet-4"
	openRouterMaxTok         = 8192
	openRouterDefaultTimeout = 120 * time.Second
)

// OpenRouterProvider calls the OpenRouter API (OpenAI-compatible format).
type OpenRouterProvider struct {
	apiKey  string
	baseURL string
	model   string
	timeout time.Duration
}

type openRouterRequest struct {
	Model          string              `json:"model"`
	MaxTokens      int                 `json:"max_tokens"`
	Messages       []openRouterMessage `json:"messages"`
	ResponseFormat *openRouterFormat   `json:"response_format,omitempty"`
}

type openRouterFormat struct {
	Type       string         `json:"type"`
	JSONSchema map[string]any `json:"json_schema,omitempty"`
}

type openRouterMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type openRouterResponse struct {
	Choices []openRouterChoice `json:"choices"`
	Error   *struct {
		Message string `json:"message"`
	} `json:"error,omitempty"`
}

type openRouterChoice struct {
	Message openRouterMessage `json:"message"`
}

// Complete sends a prompt to the OpenRouter API and returns the text response.
func (p *OpenRouterProvider) Complete(prompt string) (string, error) {
	reqBody := openRouterRequest{
		Model:     p.model,
		MaxTokens: openRouterMaxTok,
		Messages: []openRouterMessage{
			{Role: "user", Content: prompt},
		},
	}

	body, err := json.Marshal(reqBody)
	if err != nil {
		return "", fmt.Errorf("marshal request: %w", err)
	}

	timeout := p.timeout
	if timeout == 0 {
		timeout = openRouterDefaultTimeout
	}

	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	url := p.baseURL
	if url == "" {
		url = openRouterBaseURL
	}
	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(body))
	if err != nil {
		return "", fmt.Errorf("create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+p.apiKey)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		if errors.Is(err, context.DeadlineExceeded) || ctx.Err() != nil {
			return "", fmt.Errorf("OpenRouter request timed out after %s. Try a smaller file or use `--provider ollama`", timeout)
		}
		return "", fmt.Errorf("send request: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(io.LimitReader(resp.Body, 10<<20)) // 10MB limit
	if err != nil {
		return "", fmt.Errorf("read response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("API error %d: %s", resp.StatusCode, string(respBody))
	}

	var result openRouterResponse
	if err := json.Unmarshal(respBody, &result); err != nil {
		return "", fmt.Errorf("unmarshal response: %w", err)
	}

	if result.Error != nil {
		return "", fmt.Errorf("API error: %s", result.Error.Message)
	}

	if len(result.Choices) == 0 {
		return "", fmt.Errorf("empty response from API")
	}

	return result.Choices[0].Message.Content, nil
}

// CompleteStructured sends a prompt and forces JSON output matching jsonSchema
// via OpenRouter's response_format parameter (OpenAI-compatible structured outputs).
func (p *OpenRouterProvider) CompleteStructured(ctx context.Context, prompt string, jsonSchema map[string]any) (map[string]any, error) {
	reqBody := openRouterRequest{
		Model:     p.model,
		MaxTokens: openRouterMaxTok,
		Messages:  []openRouterMessage{{Role: "user", Content: prompt}},
		ResponseFormat: &openRouterFormat{
			Type:       "json_schema",
			JSONSchema: jsonSchema,
		},
	}

	body, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("marshal request: %w", err)
	}

	timeout := p.timeout
	if timeout == 0 {
		timeout = openRouterDefaultTimeout
	}
	if _, hasDeadline := ctx.Deadline(); !hasDeadline {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, timeout)
		defer cancel()
	}

	url := p.baseURL
	if url == "" {
		url = openRouterBaseURL
	}
	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+p.apiKey)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("send request: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(io.LimitReader(resp.Body, 10<<20))
	if err != nil {
		return nil, fmt.Errorf("read response: %w", err)
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("API error %d: %s", resp.StatusCode, string(respBody))
	}

	var result openRouterResponse
	if err := json.Unmarshal(respBody, &result); err != nil {
		return nil, fmt.Errorf("unmarshal response: %w", err)
	}
	if result.Error != nil {
		return nil, fmt.Errorf("API error: %s", result.Error.Message)
	}
	if len(result.Choices) == 0 {
		return nil, fmt.Errorf("empty response from API")
	}

	var out map[string]any
	if err := json.Unmarshal([]byte(result.Choices[0].Message.Content), &out); err != nil {
		return nil, fmt.Errorf("parse structured response: %w", err)
	}
	return out, nil
}

func registerOpenRouterProvider() {
	RegisterProvider("openrouter", func(apiKey string) Provider {
		return &OpenRouterProvider{
			apiKey:  apiKey,
			baseURL: openRouterBaseURL,
			model:   openRouterModel,
			timeout: openRouterDefaultTimeout,
		}
	})
}

func init() {
	registerOpenRouterProvider()
}
