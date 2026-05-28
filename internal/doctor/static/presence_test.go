package static

import (
	"testing"

	"github.com/mirandaguillaume/reify/internal/doctor/parser"
	"github.com/stretchr/testify/assert"
)

func TestPresence_MissingSections(t *testing.T) {
	analysis := &parser.AgentAnalysis{
		Format:     "claude",
		Sections:   []parser.Section{{Header: "Rules", Level: 2}},
		RawContent: []byte("## Rules\nBe nice"),
	}

	check := &presenceCheck{}
	findings := check.Run(analysis)

	// Should find many missing sections (only "Rules" is present, which matches nothing)
	assert.True(t, len(findings) > 5, "expected many missing section findings, got %d", len(findings))

	// Verify findings have correct structure
	for _, f := range findings {
		assert.NotEmpty(t, f.Category)
		assert.Contains(t, f.Issue, "Missing")
		assert.Equal(t, "moderate", f.Confidence)
		assert.NotEmpty(t, f.CitationID)
	}
}

func TestPresence_AllPresent(t *testing.T) {
	// File that has keywords matching all 15 categories
	content := `## Identity
You are a code reviewer.

## Security
Filesystem access restricted.

## Guardrails
Timeout: 5min. Output limit: 1000 lines.

## Testing
Run go test ./...

## Examples
Here is an example output.

## Error Handling
On failure, retry once.

## Build Commands
go build ./cmd/reify

## Architecture
Layers: cmd → internal → pkg

## Output Format
Respond in JSON.

## Decision Authority
Escalate to human for destructive operations.

## Workflow Triggers
Invoke when PR is opened.

## Dependencies
Consumes: file content. Produces: findings.

## Memory
Session context only.

## Goals
Improve agent file quality.

## Constraints
Never modify source code directly.`

	analysis := &parser.AgentAnalysis{
		Format: "claude",
		Sections: []parser.Section{
			{Header: "Identity", Level: 2},
			{Header: "Security", Level: 2},
			{Header: "Guardrails", Level: 2},
			{Header: "Testing", Level: 2},
			{Header: "Examples", Level: 2},
			{Header: "Error Handling", Level: 2},
			{Header: "Build Commands", Level: 2},
			{Header: "Architecture", Level: 2},
			{Header: "Output Format", Level: 2},
			{Header: "Decision Authority", Level: 2},
			{Header: "Workflow Triggers", Level: 2},
			{Header: "Dependencies", Level: 2},
			{Header: "Memory", Level: 2},
			{Header: "Goals", Level: 2},
			{Header: "Constraints", Level: 2},
		},
		RawContent: []byte(content),
	}

	check := &presenceCheck{}
	findings := check.Run(analysis)
	assert.Empty(t, findings, "all sections present — no findings expected")
}

func TestPresence_BodyContentMatches_MultiWordOnly(t *testing.T) {
	// Body-content matching is restricted to multi-word phrases to avoid false positives.
	// Single words like "error", "run", "output" do NOT match in body — only in headings.
	analysis := &parser.AgentAnalysis{
		Format:     "claude",
		Sections:   []parser.Section{{Header: "Rules", Level: 2}},
		RawContent: []byte("## Rules\nYou are a code reviewer.\nAccess control is strict.\nRun tests with go test.\nHere is a test plan."),
	}

	check := &presenceCheck{}
	findings := check.Run(analysis)
	// "You are" matches identity, "access control" matches security, "test plan" matches testing
	// But single words like "error", "build", "output" should NOT match
	identityFound := false
	for _, f := range findings {
		if f.Category == "identity" {
			identityFound = true
		}
	}
	assert.False(t, identityFound, "'You are' in body should match identity via bodyOnlyKeywords")
}

func TestPresence_NilAnalysis(t *testing.T) {
	check := &presenceCheck{}
	assert.Nil(t, check.Run(nil))
}
