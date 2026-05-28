package analyzer

import (
	"os"
	"strings"
	"testing"

	"github.com/mirandaguillaume/reify/internal/doctor/parser"
	"github.com/mirandaguillaume/reify/internal/doctor/registry"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// mockProvider returns a predefined response.
type mockProvider struct {
	response string
	err      error
}

func (m *mockProvider) Complete(_ string) (string, error) {
	return m.response, m.err
}

func TestAnalyze_Success(t *testing.T) {
	analysis := &parser.AgentAnalysis{
		Format:      "claude",
		Frontmatter: map[string]interface{}{"name": "test", "tools": "Read"},
		Sections:    []parser.Section{{Header: "Rules", Content: "- Be nice", Level: 2}},
		Tools:       []string{"Read"},
		RawContent:  []byte("---\nname: test\ntools: Read\n---\n## Rules\n- Be nice"),
	}

	provider := &mockProvider{
		response: `findings:
  - category: guardrails
    issue: "No timeout or output limits"
    confidence: high
    current_state: "No guardrails section"
    suggested_improvement: "Add guardrails with timeout and output limits"
  - category: security
    issue: "No security declarations"
    confidence: moderate
    current_state: "No security facet"
    suggested_improvement: "Add filesystem and network declarations"`,
	}

	findings, err := provider.Complete("test") // verify mock works
	require.NoError(t, err)
	assert.Contains(t, findings, "guardrails")

	results, err := Analyze(analysis, provider, nil)
	require.NoError(t, err)
	assert.Len(t, results, 2)
	assert.Equal(t, "guardrails", results[0].Category)
	assert.Equal(t, "high", results[0].Confidence)
	assert.Equal(t, "security", results[1].Category)
}

func TestAnalyze_WithCodeFences(t *testing.T) {
	analysis := &parser.AgentAnalysis{
		Format:     "claude",
		RawContent: []byte("test"),
	}

	provider := &mockProvider{
		response: "Here are my findings:\n```yaml\nfindings:\n  - category: ordering\n    issue: \"Bad ordering\"\n    confidence: low\n```",
	}

	results, err := Analyze(analysis, provider, nil)
	require.NoError(t, err)
	assert.Len(t, results, 1)
	assert.Equal(t, "ordering", results[0].Category)
}

func TestAnalyze_ProviderError(t *testing.T) {
	analysis := &parser.AgentAnalysis{Format: "claude", RawContent: []byte("test")}
	provider := &mockProvider{err: assert.AnError}

	_, err := Analyze(analysis, provider, nil)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "LLM analysis failed")
}

func TestAnalyze_EmptyResponse(t *testing.T) {
	analysis := &parser.AgentAnalysis{Format: "claude", RawContent: []byte("test")}
	provider := &mockProvider{response: ""}

	_, err := Analyze(analysis, provider, nil)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "empty")
}

func TestAnalyze_NilAnalysis(t *testing.T) {
	provider := &mockProvider{response: "test"}
	_, err := Analyze(nil, provider, nil)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "nil")
}

func TestBuildPrompt(t *testing.T) {
	reg, err := registry.Load("")
	require.NoError(t, err)

	analysis := &parser.AgentAnalysis{
		Format:      "claude",
		Frontmatter: map[string]interface{}{"name": "reviewer", "tools": "Read, Grep"},
		Sections:    []parser.Section{{Header: "Rules", Level: 2}, {Header: "Process", Level: 2}},
		Tools:       []string{"Read", "Grep"},
		RawContent:  []byte("---\nname: reviewer\n---\nBody"),
	}

	prompt := buildPrompt(analysis, reg)
	assert.Contains(t, prompt, "claude")
	assert.Contains(t, prompt, "Read, Grep")
	assert.Contains(t, prompt, "Rules")
	// LLM-only semantic categories (static checks handle the rest)
	assert.Contains(t, prompt, "decomposition")
	assert.Contains(t, prompt, "context")
	assert.Contains(t, prompt, "scope")
	assert.Contains(t, prompt, "error_handling")
	assert.Contains(t, prompt, "examples")
	// Static-only categories should NOT be in the LLM prompt
	assert.NotContains(t, prompt, "1. guardrails")
	assert.NotContains(t, prompt, "1. security")
	assert.NotContains(t, prompt, "1. ordering")
	assert.Contains(t, prompt, "findings:")
	// Verify CoT-before-score
	assert.Contains(t, prompt, "reasoning:")
	assert.Contains(t, prompt, "confidence:")
	reasoningIdx := strings.Index(prompt, "reasoning:")
	confidenceIdx := strings.Index(prompt, "confidence:")
	require.NotEqual(t, -1, reasoningIdx, "reasoning: must be in prompt")
	require.NotEqual(t, -1, confidenceIdx, "confidence: must be in prompt")
	assert.Less(t, reasoningIdx, confidenceIdx,
		"reasoning must appear before confidence in prompt example (CoT-before-score)")
	// Verify focused categories with citations
	assert.Contains(t, prompt, "Li et al., 2026") // SkillsBench for decomposition/scope
	assert.Contains(t, prompt, "across 8 categories")
}

func TestAnalyze_EnrichesCitationID(t *testing.T) {
	analysis := &parser.AgentAnalysis{
		Format:     "claude",
		RawContent: []byte("test"),
	}

	provider := &mockProvider{
		response: `findings:
  - category: guardrails
    issue: "No guardrails"
    confidence: high
    current_state: "None"
    suggested_improvement: "Add guardrails"`,
	}

	results, err := Analyze(analysis, provider, nil)
	require.NoError(t, err)
	assert.Len(t, results, 1)
	assert.Equal(t, "guardrails", results[0].CitationID)
}

func TestBuildPrompt_SizeGuard(t *testing.T) {
	reg, err := registry.Load("")
	require.NoError(t, err)

	// Create a large content that exceeds 128KB (~32K tokens)
	largeContent := make([]byte, 130_000)
	for i := range largeContent {
		largeContent[i] = 'A' + byte(i%26)
	}

	analysis := &parser.AgentAnalysis{
		Format:     "claude",
		RawContent: largeContent,
	}

	prompt := buildPrompt(analysis, reg)
	assert.Contains(t, prompt, "TRUNCATED")
	assert.Less(t, len(prompt), len(largeContent)+10_000,
		"prompt must be shorter than the original oversized content")
}

func TestBuildPrompt_NormalSizeNotTruncated(t *testing.T) {
	reg, err := registry.Load("")
	require.NoError(t, err)

	analysis := &parser.AgentAnalysis{
		Format:     "claude",
		RawContent: []byte("Normal sized content here."),
	}

	prompt := buildPrompt(analysis, reg)
	assert.NotContains(t, prompt, "TRUNCATED")
	assert.Contains(t, prompt, "Normal sized content here.")
}

func TestAnalyze_Integration(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}
	apiKey := os.Getenv("OPENROUTER_API_KEY")
	if apiKey == "" {
		t.Skip("OPENROUTER_API_KEY not set")
	}

	// This would test with a real LLM — skipped by default
	t.Log("Integration test placeholder — run with OPENROUTER_API_KEY set")
}
