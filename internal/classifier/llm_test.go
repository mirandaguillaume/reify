package classifier

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// fakeProvider is an in-memory llm.Provider implementation for tests.
type fakeProvider struct {
	response string
	err      error
	calls    int
	lastPrompt string
}

func (f *fakeProvider) Complete(prompt string) (string, error) {
	f.calls++
	f.lastPrompt = prompt
	return f.response, f.err
}

func TestClassifyLLM_EmptyContentSkipsProvider(t *testing.T) {
	fp := &fakeProvider{response: "should-not-be-called"}
	r, err := ClassifyLLM("", "", fp)
	require.NoError(t, err)
	assert.Empty(t, r.Items)
	assert.Equal(t, 0, fp.calls, "provider must not be called when there are no items")
}

func TestClassifyLLM_AppliesLLMFacets(t *testing.T) {
	content := `## Commands
- First instruction
- Second instruction`

	fp := &fakeProvider{
		response: `[{"i": 1, "facet": "guardrails"}, {"i": 2, "facet": "security"}]`,
	}
	r, err := ClassifyLLM(content, "claude", fp)
	require.NoError(t, err)
	require.Len(t, r.Items, 2)
	assert.Equal(t, FacetGuardrails, r.Items[0].Facet)
	assert.Equal(t, FacetSecurity, r.Items[1].Facet)
	assert.Equal(t, "claude", r.Format)
	assert.Equal(t, 1, fp.calls)
}

func TestClassifyLLM_FallsBackOnInvalidJSON(t *testing.T) {
	content := `## Commands
- An instruction`
	fp := &fakeProvider{response: "this is not JSON at all"}
	r, err := ClassifyLLM(content, "", fp)
	require.NoError(t, err, "invalid JSON should fall back, not error")
	require.Len(t, r.Items, 1)
	// Static fallback picks the section facet (Commands → strategy).
	assert.Equal(t, FacetStrategy, r.Items[0].Facet)
}

func TestClassifyLLM_PropagatesProviderError(t *testing.T) {
	content := "- one"
	fp := &fakeProvider{err: errors.New("boom")}
	_, err := ClassifyLLM(content, "", fp)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "LLM classification failed")
	assert.Contains(t, err.Error(), "boom")
}

func TestClassifyLLM_PromptContainsAllItems(t *testing.T) {
	content := `## X
- alpha
- beta`
	fp := &fakeProvider{response: `[]`}
	_, _ = ClassifyLLM(content, "", fp)
	assert.Contains(t, fp.lastPrompt, "alpha")
	assert.Contains(t, fp.lastPrompt, "beta")
	assert.Contains(t, fp.lastPrompt, "1. alpha")
	assert.Contains(t, fp.lastPrompt, "2. beta")
}

func TestClassifyLLM_ExtractsJSONFromNoise(t *testing.T) {
	content := "- one"
	fp := &fakeProvider{
		response: "Sure! Here you go:\n\n[{\"i\": 1, \"facet\": \"security\"}]\n\nHope that helps.",
	}
	r, err := ClassifyLLM(content, "", fp)
	require.NoError(t, err)
	require.Len(t, r.Items, 1)
	assert.Equal(t, FacetSecurity, r.Items[0].Facet)
}

func TestClassifyLLM_HandlesThinkBlocks(t *testing.T) {
	content := "- one"
	fp := &fakeProvider{
		response: "<think>let me reason about this...</think>[{\"i\": 1, \"facet\": \"strategy\"}]",
	}
	r, err := ClassifyLLM(content, "", fp)
	require.NoError(t, err)
	assert.Equal(t, FacetStrategy, r.Items[0].Facet)
}

func TestClassifyLLM_MissingIndexFallsBackToStatic(t *testing.T) {
	content := `## Commands
- alpha
- beta`
	// LLM only classifies item 1; item 2 should retain its static facet.
	fp := &fakeProvider{response: `[{"i": 1, "facet": "guardrails"}]`}
	r, err := ClassifyLLM(content, "", fp)
	require.NoError(t, err)
	require.Len(t, r.Items, 2)
	assert.Equal(t, FacetGuardrails, r.Items[0].Facet)
	assert.Equal(t, FacetStrategy, r.Items[1].Facet, "missing LLM index should fall back to static section facet")
}

func TestNormalizeFacet(t *testing.T) {
	cases := []struct {
		input string
		want  Facet
	}{
		{"strategy", FacetStrategy},
		{"STRATEGY", FacetStrategy},
		{"  strategy  ", FacetStrategy},
		{"guardrails", FacetGuardrails},
		{"guardrail", FacetGuardrails},
		{"observability", FacetObservability},
		{"observ", FacetObservability},
		{"security", FacetSecurity},
		{"context", FacetContext},
		{"unknown-facet", FacetContext}, // default
		{"", FacetContext},
	}
	for _, tc := range cases {
		t.Run(tc.input, func(t *testing.T) {
			assert.Equal(t, tc.want, normalizeFacet(tc.input))
		})
	}
}

func TestStripThinkBlocks(t *testing.T) {
	cases := []struct {
		name string
		in   string
		want string
	}{
		{"no-blocks", "plain text", "plain text"},
		{"single-block", "<think>reasoning</think>answer", "answer"},
		{"multiple-blocks", "<think>a</think>x<think>b</think>y", "xy"},
		{"unclosed-block", "before<think>never closed", "before"},
		{"only-block", "<think>only</think>", ""},
		{"surrounding-whitespace", "   <think>x</think>answer   ", "answer"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.want, stripThinkBlocks(tc.in))
		})
	}
}

func TestExtractJSONArray(t *testing.T) {
	cases := []struct {
		name string
		in   string
		want string
	}{
		{"clean-array", `[1,2,3]`, `[1,2,3]`},
		{"with-prefix", `text before [1,2]`, `[1,2]`},
		{"with-suffix", `[1,2] text after`, `[1,2]`},
		{"both-sides", `noise [a,b] noise`, `[a,b]`},
		{"no-brackets", `nothing here`, `nothing here`},
		{"closing-before-opening", `] [`, `] [`}, // end < start → returns original
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.want, extractJSONArray(tc.in))
		})
	}
}
