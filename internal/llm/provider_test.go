package llm

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Mock provider for testing
type mockProvider struct {
	response string
	err      error
}

func (m *mockProvider) Complete(prompt string) (string, error) {
	return m.response, m.err
}

func TestMockProviderImplementsInterface(t *testing.T) {
	var p Provider = &mockProvider{response: "hello"}
	result, err := p.Complete("test")
	assert.NoError(t, err)
	assert.Equal(t, "hello", result)
}

func TestRegisterAndGet(t *testing.T) {
	// Save and restore
	old := providers
	providers = make(map[string]ProviderFactory)
	defer func() { providers = old }()

	RegisterProvider("mock", func(apiKey string) Provider {
		return &mockProvider{response: "ok"}
	})

	p, err := GetProvider("mock", "key123")
	require.NoError(t, err)

	result, err := p.Complete("test")
	assert.NoError(t, err)
	assert.Equal(t, "ok", result)
}

func TestGetUnregisteredProvider(t *testing.T) {
	old := providers
	providers = make(map[string]ProviderFactory)
	defer func() { providers = old }()

	_, err := GetProvider("unknown", "key")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "unknown provider")
}

func TestAnthropicComplete(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "POST", r.Method)
		assert.Equal(t, "application/json", r.Header.Get("Content-Type"))
		assert.Equal(t, "test-key", r.Header.Get("x-api-key"))
		assert.Equal(t, "2023-06-01", r.Header.Get("anthropic-version"))

		body, _ := io.ReadAll(r.Body)
		var req map[string]interface{}
		json.Unmarshal(body, &req)
		assert.Equal(t, "claude-sonnet-4-20250514", req["model"])

		resp := map[string]interface{}{
			"content": []map[string]interface{}{
				{"type": "text", "text": `{"skills": [{"yaml": "skill: test"}]}`},
			},
		}
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	p := &AnthropicProvider{
		apiKey:  "test-key",
		baseURL: server.URL,
	}

	result, err := p.Complete("analyze this agent")
	require.NoError(t, err)
	assert.Contains(t, result, "skills")
}

func TestAnthropicProviderRegistered(t *testing.T) {
	// init() should have registered it
	_, err := GetProvider("anthropic", "test-key")
	assert.NoError(t, err)
}

func TestOpenRouterComplete(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "POST", r.Method)
		assert.Equal(t, "application/json", r.Header.Get("Content-Type"))
		assert.Equal(t, "Bearer test-key", r.Header.Get("Authorization"))

		body, _ := io.ReadAll(r.Body)
		var req map[string]interface{}
		json.Unmarshal(body, &req)
		assert.Equal(t, "anthropic/claude-sonnet-4", req["model"])

		resp := map[string]interface{}{
			"choices": []map[string]interface{}{
				{"message": map[string]string{"role": "assistant", "content": `{"skills": [{"yaml": "skill: test"}]}`}},
			},
		}
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	p := &OpenRouterProvider{
		apiKey:  "test-key",
		baseURL: server.URL,
		model:   openRouterModel,
	}

	result, err := p.Complete("analyze this agent")
	require.NoError(t, err)
	assert.Contains(t, result, "skills")
}

func TestOpenRouterProviderRegistered(t *testing.T) {
	_, err := GetProvider("openrouter", "test-key")
	assert.NoError(t, err)
}
