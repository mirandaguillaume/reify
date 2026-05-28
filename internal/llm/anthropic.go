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
	defaultBaseURL          = "https://api.anthropic.com/v1/messages"
	anthropicVersion        = "2023-06-01"
	anthropicDefaultModel   = "claude-sonnet-4-20250514"
	anthropicMaxTok         = 8192
	anthropicDefaultTimeout = 120 * time.Second
)

// AnthropicProvider calls the Anthropic Messages API.
type AnthropicProvider struct {
	apiKey  string
	model   string
	baseURL string
	timeout time.Duration
}

// resolveAnthropicModel maps short model names to full API model IDs.
func resolveAnthropicModel(name string) string {
	aliases := map[string]string{
		"haiku":  "claude-haiku-4-5-20251001",
		"sonnet": "claude-sonnet-4-20250514",
		"opus":   "claude-opus-4-20250514",
	}
	if full, ok := aliases[name]; ok {
		return full
	}
	return name // assume it's already a full model ID
}

type anthropicRequest struct {
	Model      string               `json:"model"`
	MaxTokens  int                  `json:"max_tokens"`
	Messages   []anthropicMessage   `json:"messages"`
	Tools      []anthropicTool      `json:"tools,omitempty"`
	ToolChoice *anthropicToolChoice `json:"tool_choice,omitempty"`
}

type anthropicMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

// anthropicTool declares a single tool for constrained-decoding (tool forcing).
type anthropicTool struct {
	Name        string         `json:"name"`
	Description string         `json:"description,omitempty"`
	InputSchema map[string]any `json:"input_schema"`
}

type anthropicToolChoice struct {
	Type string `json:"type"`
	Name string `json:"name,omitempty"`
}

type anthropicResponse struct {
	Content    []anthropicContent `json:"content"`
	StopReason string             `json:"stop_reason"`
	Usage      struct {
		InputTokens  int `json:"input_tokens"`
		OutputTokens int `json:"output_tokens"`
	} `json:"usage"`
	Error *struct {
		Message string `json:"message"`
	} `json:"error,omitempty"`
}

// anthropicAnyMessage is used in the agentic loop where Content may be a string
// (user turn) or a []any of content blocks (assistant + tool_result turns).
type anthropicAnyMessage struct {
	Role    string `json:"role"`
	Content any    `json:"content"`
}

// anthropicToolResultBlock is a tool_result content block sent back to the API.
type anthropicToolResultBlock struct {
	Type      string `json:"type"`
	ToolUseID string `json:"tool_use_id"`
	Content   string `json:"content"`
}

type anthropicContent struct {
	Type  string         `json:"type"`
	Text  string         `json:"text,omitempty"`
	ID    string         `json:"id,omitempty"`
	Name  string         `json:"name,omitempty"`
	Input map[string]any `json:"input,omitempty"`
}

// SetModel sets the model to use for API calls. Accepts short aliases (haiku, sonnet, opus)
// or full model IDs (claude-haiku-4-5-20251001).
func (p *AnthropicProvider) SetModel(model string) {
	p.model = resolveAnthropicModel(model)
}

// CompleteWithUsage sends a prompt and returns the response along with the
// exact token counts reported by the API (not estimated).
func (p *AnthropicProvider) CompleteWithUsage(prompt string) (string, TokenUsage, error) {
	text, result, err := p.completeInternal(prompt)
	if err != nil {
		return "", TokenUsage{}, err
	}
	return text, TokenUsage{
		InputTokens:  result.Usage.InputTokens,
		OutputTokens: result.Usage.OutputTokens,
	}, nil
}

// Complete sends a prompt to the Anthropic Messages API and returns the text response.
func (p *AnthropicProvider) Complete(prompt string) (string, error) {
	text, _, err := p.completeInternal(prompt)
	return text, err
}

// CompleteStructured sends a prompt and forces the model to respond with a JSON object
// matching jsonSchema via tool forcing. This implements StructuredProvider: the API
// guarantees the response conforms to the schema — no heuristic extraction needed.
func (p *AnthropicProvider) CompleteStructured(ctx context.Context, prompt string, jsonSchema map[string]any) (map[string]any, error) {
	if _, hasDeadline := ctx.Deadline(); !hasDeadline {
		timeout := p.timeout
		if timeout == 0 {
			timeout = anthropicDefaultTimeout
		}
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, timeout)
		defer cancel()
	}

	model := p.model
	if model == "" {
		model = anthropicDefaultModel
	}

	reqBody := anthropicRequest{
		Model:     model,
		MaxTokens: anthropicMaxTok,
		Messages:  []anthropicMessage{{Role: "user", Content: prompt}},
		Tools: []anthropicTool{{
			Name:        "structured_output",
			InputSchema: jsonSchema,
		}},
		ToolChoice: &anthropicToolChoice{Type: "tool", Name: "structured_output"},
	}

	result, err := p.sendRequest(ctx, reqBody)
	if err != nil {
		return nil, err
	}

	for _, block := range result.Content {
		if block.Type == "tool_use" && block.Name == "structured_output" {
			return block.Input, nil
		}
	}
	return nil, fmt.Errorf("no tool_use block in structured response")
}

func (p *AnthropicProvider) completeInternal(prompt string) (string, anthropicResponse, error) {
	model := p.model
	if model == "" {
		model = anthropicDefaultModel
	}

	reqBody := anthropicRequest{
		Model:     model,
		MaxTokens: anthropicMaxTok,
		Messages:  []anthropicMessage{{Role: "user", Content: prompt}},
	}

	timeout := p.timeout
	if timeout == 0 {
		timeout = anthropicDefaultTimeout
	}
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	result, err := p.sendRequest(ctx, reqBody)
	if err != nil {
		return "", anthropicResponse{}, err
	}

	if len(result.Content) == 0 {
		return "", result, fmt.Errorf("empty response from API")
	}

	return result.Content[0].Text, result, nil
}

// sendRequest marshals reqBody, POSTs to the Anthropic Messages API, and returns
// the parsed response. Timeout handling and error wrapping live here so both the
// text and structured paths share the same transport logic.
func (p *AnthropicProvider) sendRequest(ctx context.Context, reqBody anthropicRequest) (anthropicResponse, error) {
	body, err := json.Marshal(reqBody)
	if err != nil {
		return anthropicResponse{}, fmt.Errorf("marshal request: %w", err)
	}

	url := p.baseURL
	if url == "" {
		url = defaultBaseURL
	}
	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(body))
	if err != nil {
		return anthropicResponse{}, fmt.Errorf("create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("x-api-key", p.apiKey)
	req.Header.Set("anthropic-version", anthropicVersion)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		if errors.Is(err, context.DeadlineExceeded) || ctx.Err() != nil {
			timeout := p.timeout
			if timeout == 0 {
				timeout = anthropicDefaultTimeout
			}
			return anthropicResponse{}, fmt.Errorf("anthropic request timed out after %s. Try a smaller file or use `--provider ollama`", timeout)
		}
		return anthropicResponse{}, fmt.Errorf("send request: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(io.LimitReader(resp.Body, 10<<20)) // 10MB limit
	if err != nil {
		return anthropicResponse{}, fmt.Errorf("read response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return anthropicResponse{}, fmt.Errorf("API error %d: %s", resp.StatusCode, string(respBody))
	}

	var result anthropicResponse
	if err := json.Unmarshal(respBody, &result); err != nil {
		return anthropicResponse{}, fmt.Errorf("unmarshal response: %w", err)
	}

	if result.Error != nil {
		return anthropicResponse{}, fmt.Errorf("API error: %s", result.Error.Message)
	}

	return result, nil
}

const agenticMaxIterations = 10

// CompleteWithTools runs the agentic loop: sends the prompt with tool definitions,
// executes any tool_use blocks the model emits, and repeats until the model
// produces a final text response (stop_reason != "tool_use").
// Stats reports the number of API calls and per-tool invocation counts.
func (p *AnthropicProvider) CompleteWithTools(ctx context.Context, prompt string, tools []Tool, opts ...AgenticOptions) (string, AgenticStats, error) {
	var opt AgenticOptions
	if len(opts) > 0 {
		opt = opts[0]
	}

	stats := AgenticStats{ToolCalls: make(map[string]int)}

	if _, hasDeadline := ctx.Deadline(); !hasDeadline {
		timeout := p.timeout
		if timeout == 0 {
			timeout = anthropicDefaultTimeout
		}
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, timeout)
		defer cancel()
	}

	apiTools := make([]anthropicTool, len(tools))
	for i, t := range tools {
		schema := t.InputSchema
		if schema == nil {
			schema = map[string]any{"type": "object", "properties": map[string]any{}}
		}
		apiTools[i] = anthropicTool{Name: t.Name, Description: t.Description, InputSchema: schema}
	}

	toolIndex := make(map[string]Tool, len(tools))
	for _, t := range tools {
		toolIndex[t.Name] = t
	}

	messages := []anthropicAnyMessage{{Role: "user", Content: prompt}}

	for i := 0; i < agenticMaxIterations; i++ {
		if opt.OnIteration != nil {
			if err := opt.OnIteration(i); err != nil {
				return "", stats, err
			}
		}

		result, err := p.sendRequestAny(ctx, apiTools, messages)
		if err != nil {
			return "", stats, err
		}
		stats.APICalls++

		if result.StopReason != "tool_use" {
			for _, block := range result.Content {
				if block.Type == "text" {
					return block.Text, stats, nil
				}
			}
			return "", stats, fmt.Errorf("no text block in final response")
		}

		// Append assistant message with all content blocks.
		assistantBlocks := make([]any, len(result.Content))
		for j, block := range result.Content {
			assistantBlocks[j] = block
		}
		messages = append(messages, anthropicAnyMessage{Role: "assistant", Content: assistantBlocks})

		// Execute each tool_use block and collect results.
		var resultBlocks []any
		for _, block := range result.Content {
			if block.Type != "tool_use" {
				continue
			}
			t, ok := toolIndex[block.Name]
			if !ok {
				return "", stats, fmt.Errorf("model called unknown tool %q", block.Name)
			}
			output, toolErr := t.Run(ctx, block.Input)
			if toolErr != nil {
				output = fmt.Sprintf("error: %s", toolErr)
			}
			stats.ToolCalls[block.Name]++
			resultBlocks = append(resultBlocks, anthropicToolResultBlock{
				Type:      "tool_result",
				ToolUseID: block.ID,
				Content:   output,
			})
		}
		messages = append(messages, anthropicAnyMessage{Role: "user", Content: resultBlocks})
	}

	return "", stats, fmt.Errorf("agentic loop exceeded %d iterations", agenticMaxIterations)
}

// sendRequestAny is the agentic-path variant of sendRequest: it accepts
// []anthropicAnyMessage (Content any) instead of []anthropicMessage (Content string).
func (p *AnthropicProvider) sendRequestAny(ctx context.Context, tools []anthropicTool, messages []anthropicAnyMessage) (anthropicResponse, error) {
	model := p.model
	if model == "" {
		model = anthropicDefaultModel
	}

	reqBody := struct {
		Model     string                `json:"model"`
		MaxTokens int                   `json:"max_tokens"`
		Messages  []anthropicAnyMessage `json:"messages"`
		Tools     []anthropicTool       `json:"tools,omitempty"`
	}{
		Model:     model,
		MaxTokens: anthropicMaxTok,
		Messages:  messages,
		Tools:     tools,
	}

	body, err := json.Marshal(reqBody)
	if err != nil {
		return anthropicResponse{}, fmt.Errorf("marshal request: %w", err)
	}

	url := p.baseURL
	if url == "" {
		url = defaultBaseURL
	}
	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(body))
	if err != nil {
		return anthropicResponse{}, fmt.Errorf("create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("x-api-key", p.apiKey)
	req.Header.Set("anthropic-version", anthropicVersion)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return anthropicResponse{}, fmt.Errorf("send request: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(io.LimitReader(resp.Body, 10<<20))
	if err != nil {
		return anthropicResponse{}, fmt.Errorf("read response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return anthropicResponse{}, fmt.Errorf("API error %d: %s", resp.StatusCode, string(respBody))
	}

	var result anthropicResponse
	if err := json.Unmarshal(respBody, &result); err != nil {
		return anthropicResponse{}, fmt.Errorf("unmarshal response: %w", err)
	}

	if result.Error != nil {
		return anthropicResponse{}, fmt.Errorf("API error: %s", result.Error.Message)
	}

	return result, nil
}

func registerAnthropicProvider() {
	RegisterProvider("anthropic", func(apiKey string) Provider {
		return &AnthropicProvider{
			apiKey:  apiKey,
			baseURL: defaultBaseURL,
			timeout: anthropicDefaultTimeout,
		}
	})
}

func init() {
	registerAnthropicProvider()
}
