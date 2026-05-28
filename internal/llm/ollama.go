package llm

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"
)

const (
	ollamaDefaultBaseURL = "http://localhost:11434"
	ollamaDefaultModel   = "llama3.1:8b-instruct-q5_K_M"
	ollamaDefaultTimeout = 300 * time.Second
	ollamaMaxTokens      = 2048
	ollamaTemperature    = 0.1
)

// OllamaProvider calls a local Ollama instance via its native REST API.
type OllamaProvider struct {
	baseURL string
	model   string
	timeout time.Duration
}

type ollamaRequest struct {
	Model   string        `json:"model"`
	Prompt  string        `json:"prompt"`
	Stream  bool          `json:"stream"`
	Options ollamaOptions `json:"options"`
}

type ollamaOptions struct {
	Temperature float64 `json:"temperature"`
	NumPredict  int     `json:"num_predict"`
	Think       *bool   `json:"think,omitempty"`
}

type ollamaResponse struct {
	Response string `json:"response"`
	Done     bool   `json:"done"`
	Error    string `json:"error,omitempty"`
}

// Complete sends a prompt to the local Ollama instance and returns the text response.
func (p *OllamaProvider) Complete(prompt string) (string, error) {
	reqBody := ollamaRequest{
		Model:  p.model,
		Prompt: prompt,
		Stream: false,
		Options: ollamaOptions{
			Temperature: ollamaTemperature,
			NumPredict:  ollamaMaxTokens,
		},
	}

	body, err := json.Marshal(reqBody)
	if err != nil {
		return "", fmt.Errorf("marshal request: %w", err)
	}

	timeout := p.timeout
	if timeout == 0 {
		timeout = ollamaDefaultTimeout
	}

	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	endpoint := strings.TrimRight(p.baseURL, "/") + "/api/generate"
	req, err := http.NewRequestWithContext(ctx, "POST", endpoint, bytes.NewReader(body))
	if err != nil {
		return "", fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		if isConnectionRefused(err) {
			return "", fmt.Errorf("ollama not running. Start with `ollama serve` or use `--provider anthropic`")
		}
		if errors.Is(err, context.DeadlineExceeded) || ctx.Err() != nil {
			return "", fmt.Errorf("analysis timed out. Try a smaller model or use `--provider anthropic`")
		}
		return "", fmt.Errorf("send request: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(io.LimitReader(resp.Body, 10<<20)) // 10MB limit
	if err != nil {
		return "", fmt.Errorf("read response: %w", err)
	}

	if resp.StatusCode == http.StatusNotFound {
		return "", fmt.Errorf("model not available. Run `ollama pull %s` or use `--model <available_model>`", p.model)
	}

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("API error %d: %s", resp.StatusCode, string(respBody))
	}

	var result ollamaResponse
	if err := json.Unmarshal(respBody, &result); err != nil {
		return "", fmt.Errorf("ollama returned invalid response. Try a different model or use `--provider anthropic`")
	}

	if result.Error != "" {
		if strings.Contains(result.Error, "not found") {
			return "", fmt.Errorf("model not available. Run `ollama pull %s` or use `--model <available_model>`", p.model)
		}
		return "", fmt.Errorf("ollama error: %s", result.Error)
	}

	if result.Response == "" {
		return "", fmt.Errorf("ollama returned empty response. Try a different model or use `--provider anthropic`")
	}

	return result.Response, nil
}

// isConnectionRefused checks if the error indicates a connection refused.
func isConnectionRefused(err error) bool {
	if err == nil {
		return false
	}
	var opErr *net.OpError
	if errors.As(err, &opErr) {
		return opErr.Op == "dial"
	}
	return strings.Contains(err.Error(), "connection refused") ||
		strings.Contains(err.Error(), "connect:")
}

func init() {
	RegisterProvider("ollama", func(apiKey string) Provider {
		baseURL := ollamaDefaultBaseURL
		if apiKey != "" {
			// Use apiKey as base URL override for remote Ollama instances.
			// Validate it looks like a URL.
			if u, err := url.Parse(apiKey); err == nil && u.Scheme != "" && u.Host != "" {
				baseURL = apiKey
			}
			// If not a valid URL, ignore and use default (apiKey might be an actual key — Ollama doesn't need one)
		}
		return &OllamaProvider{
			baseURL: baseURL,
			model:   ollamaDefaultModel,
			timeout: ollamaDefaultTimeout,
		}
	})
}
