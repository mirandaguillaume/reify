package cmd

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestSelectProviderExplicitOllama(t *testing.T) {
	p, name, err := selectProvider("ollama", "", false)
	// Ollama may or may not be running in CI — we only check that no API key is required.
	if err != nil {
		assert.Contains(t, err.Error(), "ollama")
		return
	}
	assert.NotNil(t, p)
	assert.Equal(t, "ollama", name)
}

func TestSelectProviderAnthropicKeyRequired(t *testing.T) {
	t.Setenv("ANTHROPIC_API_KEY", "")
	t.Setenv("ANTHROPIC_REIFY_API_KEY", "")
	t.Setenv("OPENROUTER_API_KEY", "")
	t.Setenv("OPENROUTER_REIFY_API_KEY", "")
	t.Setenv("REIFY_API_KEY", "")

	_, _, err := selectProvider("anthropic", "", false)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "ANTHROPIC_API_KEY")
}

func TestSelectProviderReifyAPIKeyOverride(t *testing.T) {
	t.Setenv("REIFY_API_KEY", "test-key")
	t.Setenv("ANTHROPIC_API_KEY", "")
	t.Setenv("OPENROUTER_API_KEY", "")

	// REIFY_API_KEY is accepted as a fallback for any cloud provider.
	p, _, err := selectProvider("anthropic", "", false)
	assert.NoError(t, err)
	assert.NotNil(t, p)
}

func TestSelectProviderNoProviderAvailable(t *testing.T) {
	t.Setenv("ANTHROPIC_API_KEY", "")
	t.Setenv("ANTHROPIC_REIFY_API_KEY", "")
	t.Setenv("OPENROUTER_API_KEY", "")
	t.Setenv("OPENROUTER_REIFY_API_KEY", "")
	t.Setenv("REIFY_API_KEY", "")

	_, _, err := selectProvider("", "", false)
	if err != nil {
		// Expected when Ollama is not running.
		assert.Contains(t, err.Error(), "no LLM provider available")
	}
}
