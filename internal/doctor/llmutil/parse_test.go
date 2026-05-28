package llmutil

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseFindings_Valid(t *testing.T) {
	yaml := `findings:
  - category: guardrails
    issue: "No timeout specified"
    confidence: high
    current_state: "No guardrails section"
    suggested_improvement: "Add timeout and output limits"
  - category: security
    issue: "No security declarations"
    confidence: moderate
    current_state: "No security section"
    suggested_improvement: "Add filesystem and network declarations"`

	findings, err := ParseFindings(yaml)
	require.NoError(t, err)
	assert.Len(t, findings, 2)
	assert.Equal(t, "guardrails", findings[0].Category)
	assert.Equal(t, "high", findings[0].Confidence)
	assert.Equal(t, "security", findings[1].Category)
}

func TestParseFindings_EmptyFindings(t *testing.T) {
	yaml := "findings: []"
	findings, err := ParseFindings(yaml)
	require.NoError(t, err)
	assert.Empty(t, findings)
}

func TestParseFindings_WrongKey(t *testing.T) {
	yaml := `results:
  - category: guardrails
    issue: test`

	_, err := ParseFindings(yaml)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "unexpected key")
}

func TestParseFindings_InvalidYAML(t *testing.T) {
	_, err := ParseFindings("not: [valid: yaml: {{{")
	require.Error(t, err)
}

func TestParseFindings_EmptyInput(t *testing.T) {
	_, err := ParseFindings("")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "empty")
}

func TestParseFindings_SkipsEmptyIssues(t *testing.T) {
	yaml := `findings:
  - category: guardrails
    issue: "Real issue"
    confidence: high
  - category: security
    issue: ""
    confidence: low`

	findings, err := ParseFindings(yaml)
	require.NoError(t, err)
	assert.Len(t, findings, 1, "should skip finding with empty issue")
}

func TestParseFindings_AllCategories(t *testing.T) {
	yaml := `findings:
  - category: guardrails
    issue: g
    confidence: high
  - category: security
    issue: s
    confidence: moderate
  - category: ordering
    issue: o
    confidence: low
  - category: decomposition
    issue: d
    confidence: high
  - category: context
    issue: c
    confidence: moderate`

	findings, err := ParseFindings(yaml)
	require.NoError(t, err)
	assert.Len(t, findings, 5)

	categories := make(map[string]bool)
	for _, f := range findings {
		categories[f.Category] = true
	}
	assert.True(t, categories["guardrails"])
	assert.True(t, categories["security"])
	assert.True(t, categories["ordering"])
	assert.True(t, categories["decomposition"])
	assert.True(t, categories["context"])
}
