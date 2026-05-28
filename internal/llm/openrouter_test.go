package llm

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestOpenRouterProvider_Complete_Success(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "POST", r.Method)
		assert.Equal(t, "application/json", r.Header.Get("Content-Type"))
		assert.Equal(t, "Bearer test-key", r.Header.Get("Authorization"))

		var req openRouterRequest
		err := json.NewDecoder(r.Body).Decode(&req)
		require.NoError(t, err)
		assert.Equal(t, "test prompt", req.Messages[0].Content)

		json.NewEncoder(w).Encode(openRouterResponse{
			Choices: []openRouterChoice{
				{Message: openRouterMessage{Role: "assistant", Content: "findings:\n  - category: test"}},
			},
		})
	}))
	defer server.Close()

	p := &OpenRouterProvider{apiKey: "test-key", baseURL: server.URL, model: "test-model", timeout: 30 * time.Second}
	result, err := p.Complete("test prompt")
	require.NoError(t, err)
	assert.Contains(t, result, "findings")
}

func TestOpenRouterProvider_Complete_Timeout(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(3 * time.Second)
		json.NewEncoder(w).Encode(openRouterResponse{
			Choices: []openRouterChoice{
				{Message: openRouterMessage{Content: "late"}},
			},
		})
	}))
	defer server.Close()

	p := &OpenRouterProvider{apiKey: "test-key", baseURL: server.URL, model: "test-model", timeout: 1 * time.Second}
	_, err := p.Complete("test")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "timed out")
}

func TestOpenRouterProvider_Complete_APIError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusTooManyRequests)
		w.Write([]byte(`{"error":{"message":"rate limit exceeded"}}`))
	}))
	defer server.Close()

	p := &OpenRouterProvider{apiKey: "test-key", baseURL: server.URL, model: "test-model", timeout: 5 * time.Second}
	_, err := p.Complete("test")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "API error 429")
}

func TestOpenRouterProvider_Complete_EmptyResponse(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(openRouterResponse{Choices: []openRouterChoice{}})
	}))
	defer server.Close()

	p := &OpenRouterProvider{apiKey: "test-key", baseURL: server.URL, model: "test-model", timeout: 5 * time.Second}
	_, err := p.Complete("test")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "empty response")
}

func TestOpenRouterProvider_Registry(t *testing.T) {
	p, err := GetProvider("openrouter", "test-key")
	require.NoError(t, err)
	assert.NotNil(t, p)

	or, ok := p.(*OpenRouterProvider)
	require.True(t, ok)
	assert.Equal(t, openRouterBaseURL, or.baseURL)
	assert.Equal(t, openRouterModel, or.model)
	assert.Equal(t, openRouterDefaultTimeout, or.timeout)
}
