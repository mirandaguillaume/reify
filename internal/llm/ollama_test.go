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

func TestOllamaProvider_Complete_Success(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Verify request format
		assert.Equal(t, "POST", r.Method)
		assert.Equal(t, "/api/generate", r.URL.Path)
		assert.Equal(t, "application/json", r.Header.Get("Content-Type"))

		var req ollamaRequest
		err := json.NewDecoder(r.Body).Decode(&req)
		require.NoError(t, err)
		assert.Equal(t, "llama3", req.Model)
		assert.Equal(t, "test prompt", req.Prompt)
		assert.Equal(t, false, req.Stream)
		assert.InDelta(t, 0.1, req.Options.Temperature, 0.01)
		assert.Equal(t, 2048, req.Options.NumPredict)

		json.NewEncoder(w).Encode(ollamaResponse{
			Response: "findings:\n  - category: guardrails\n    issue: test",
			Done:     true,
		})
	}))
	defer server.Close()

	p := &OllamaProvider{baseURL: server.URL, model: "llama3", timeout: 30 * time.Second}
	result, err := p.Complete("test prompt")
	require.NoError(t, err)
	assert.Contains(t, result, "guardrails")
	assert.Contains(t, result, "test")
}

func TestOllamaProvider_Complete_ConnectionRefused(t *testing.T) {
	p := &OllamaProvider{baseURL: "http://localhost:1", model: "llama3", timeout: 2 * time.Second}
	_, err := p.Complete("test")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "ollama not running")
	assert.Contains(t, err.Error(), "ollama serve")
}

func TestOllamaProvider_Complete_ModelNotFound(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		json.NewEncoder(w).Encode(map[string]string{"error": "model 'nonexistent' not found"})
	}))
	defer server.Close()

	p := &OllamaProvider{baseURL: server.URL, model: "nonexistent", timeout: 5 * time.Second}
	_, err := p.Complete("test")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "model not available")
	assert.Contains(t, err.Error(), "ollama pull")
}

func TestOllamaProvider_Complete_Timeout(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(3 * time.Second) // Exceed timeout
		json.NewEncoder(w).Encode(ollamaResponse{Response: "late", Done: true})
	}))
	defer server.Close()

	p := &OllamaProvider{baseURL: server.URL, model: "llama3", timeout: 1 * time.Second}
	_, err := p.Complete("test")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "timed out")
}

func TestOllamaProvider_Complete_MalformedResponse(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("not json"))
	}))
	defer server.Close()

	p := &OllamaProvider{baseURL: server.URL, model: "llama3", timeout: 5 * time.Second}
	_, err := p.Complete("test")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "invalid response")
}

func TestOllamaProvider_Complete_EmptyResponse(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(ollamaResponse{Response: "", Done: true})
	}))
	defer server.Close()

	p := &OllamaProvider{baseURL: server.URL, model: "llama3", timeout: 5 * time.Second}
	_, err := p.Complete("test")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "empty response")
}

func TestOllamaProvider_Complete_ServerError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte("internal error"))
	}))
	defer server.Close()

	p := &OllamaProvider{baseURL: server.URL, model: "llama3", timeout: 5 * time.Second}
	_, err := p.Complete("test")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "API error 500")
}

func TestOllamaProvider_Registry(t *testing.T) {
	p, err := GetProvider("ollama", "")
	require.NoError(t, err)
	assert.NotNil(t, p)

	ollama, ok := p.(*OllamaProvider)
	require.True(t, ok)
	assert.Equal(t, "http://localhost:11434", ollama.baseURL)
	assert.NotEmpty(t, ollama.model)
}

func TestOllamaProvider_Registry_BaseURLOverride(t *testing.T) {
	p, err := GetProvider("ollama", "http://remote-ollama:11434")
	require.NoError(t, err)

	ollama, ok := p.(*OllamaProvider)
	require.True(t, ok)
	assert.Equal(t, "http://remote-ollama:11434", ollama.baseURL)
}
