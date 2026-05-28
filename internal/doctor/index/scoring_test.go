package index

import (
	"testing"

	"github.com/mirandaguillaume/reify/internal/doctor/parser"
	"github.com/stretchr/testify/assert"
)

func TestScoreIndex_AllCovered(t *testing.T) {
	// 6 files, each with content matching their categories
	resolved := []ResolvedFile{
		{Path: ".agents/identity.md", Title: "Identity", Analysis: &parser.AgentAnalysis{
			Format: "claude", RawContent: []byte("# Identity\nYou are a dev.\n## Goals\nQuality.\n## Decision Authority\nEscalate."),
			Sections: []parser.Section{{Header: "Identity", Level: 1}, {Header: "Goals", Level: 2}, {Header: "Decision Authority", Level: 2}},
		}},
		{Path: ".agents/security.md", Title: "Security", Analysis: &parser.AgentAnalysis{
			Format: "claude", RawContent: []byte("# Security\nFilesystem restricted.\n## Prompt Injection\nSeparate contexts."),
			Sections: []parser.Section{{Header: "Security", Level: 1}, {Header: "Prompt Injection", Level: 2}},
		}},
		{Path: ".agents/testing.md", Title: "Testing", Analysis: &parser.AgentAnalysis{
			Format: "claude", RawContent: []byte("# Testing\nRun go test.\n## Examples\nHere."),
			Sections: []parser.Section{{Header: "Testing", Level: 1}, {Header: "Examples", Level: 2}},
		}},
		{Path: ".agents/architecture.md", Title: "Architecture", Analysis: &parser.AgentAnalysis{
			Format: "claude", RawContent: []byte("# Architecture\nLayered.\n## Context\nCodebase refs.\n## Dependencies\nInput/output."),
			Sections: []parser.Section{{Header: "Architecture", Level: 1}, {Header: "Context", Level: 2}, {Header: "Dependencies", Level: 2}},
		}},
		{Path: ".agents/guardrails.md", Title: "Guardrails", Analysis: &parser.AgentAnalysis{
			Format: "claude", RawContent: []byte("# Guardrails\nTimeout 5min.\n## Constraints\nNever delete.\n## Output Constraints\nMax 500 lines.\n## Output Format\nJSON."),
			Sections: []parser.Section{{Header: "Guardrails", Level: 1}, {Header: "Constraints", Level: 2}, {Header: "Output Constraints", Level: 2}, {Header: "Output Format", Level: 2}},
		}},
		{Path: ".agents/error-handling.md", Title: "Error Handling", Analysis: &parser.AgentAnalysis{
			Format: "claude", RawContent: []byte("# Error Handling\nRetry once.\n## Idempotency\nSafe to re-run."),
			Sections: []parser.Section{{Header: "Error Handling", Level: 1}, {Header: "Idempotency", Level: 2}},
		}},
	}

	agg := ScoreIndex(resolved, "quick")
	assert.True(t, agg.Covered > 10, "most categories should be covered, got %d/%d", agg.Covered, agg.Total)
}

func TestScoreIndex_MissingFile(t *testing.T) {
	resolved := []ResolvedFile{
		{Path: ".agents/identity.md", Title: "Identity", Analysis: &parser.AgentAnalysis{
			Format: "claude", RawContent: []byte("# Identity\nYou are a dev."),
			Sections: []parser.Section{{Header: "Identity", Level: 1}},
		}},
		{Path: ".agents/security.md", Title: "Security", Missing: true},
	}

	agg := ScoreIndex(resolved, "quick")
	// security.md is missing → its categories are uncovered
	hasSecurity := false
	for _, u := range agg.Uncovered {
		if u == "security" {
			hasSecurity = true
		}
	}
	assert.True(t, hasSecurity, "security should be uncovered when file is missing")
}

func TestScoreIndex_EmptyResolved(t *testing.T) {
	agg := ScoreIndex(nil, "quick")
	assert.Equal(t, 0, agg.Covered)
}

func TestFormatAggregate(t *testing.T) {
	agg := &AggregateScore{
		Covered: 11, Total: 15,
		Uncovered: []string{"examples", "goals"},
		PerFile: []FileScore{
			{Path: ".agents/identity.md", Title: "Identity", Passed: 2, Total: 3, Missing: []string{"goals"}},
		},
	}
	output := FormatAggregate(agg)
	assert.Contains(t, output, "11/15")
	assert.Contains(t, output, "73%")
	assert.Contains(t, output, "identity.md")
	assert.Contains(t, output, "goals")
}

func TestCategoriesForFile(t *testing.T) {
	cats := categoriesForFile(".agents/security.md")
	assert.Contains(t, cats, "security")
	// prompt_injection is scaffold-only, not in verifiableCategories
	assert.NotContains(t, cats, "prompt_injection")

	cats = categoriesForFile(".agents/unknown.md")
	assert.Nil(t, cats)

	// Exact basename match, not substring
	cats = categoriesForFile("old/identity.md.bak")
	assert.Nil(t, cats)
}

func TestScoreIndex_FullScaffold_15of15(t *testing.T) {
	// Simulate what scaffold produces — each file has the right keywords
	resolved := []ResolvedFile{
		{Path: ".agents/identity.md", Title: "Identity", Analysis: &parser.AgentAnalysis{
			Format: "claude",
			RawContent: []byte("# Identity\nYou are a dev.\n## Goals\nQuality.\n## Decision Authority\nEscalate.\n## Memory Strategy\nSession only.\n## When to Invoke\nUse for dev tasks."),
			Sections: []parser.Section{
				{Header: "Identity", Level: 1}, {Header: "Goals", Level: 2},
				{Header: "Decision Authority", Level: 2}, {Header: "Memory Strategy", Level: 2},
				{Header: "When to Invoke", Level: 2},
			},
		}},
		{Path: ".agents/security.md", Title: "Security", Analysis: &parser.AgentAnalysis{
			Format: "claude", RawContent: []byte("# Security\nFilesystem restricted."),
			Sections: []parser.Section{{Header: "Security", Level: 1}},
		}},
		{Path: ".agents/testing.md", Title: "Testing", Analysis: &parser.AgentAnalysis{
			Format: "claude",
			RawContent: []byte("# Testing\nRun go test.\n## Examples\nHere.\n## Build Commands\ngo build."),
			Sections: []parser.Section{
				{Header: "Testing", Level: 1}, {Header: "Examples", Level: 2},
				{Header: "Build Commands", Level: 2},
			},
		}},
		{Path: ".agents/architecture.md", Title: "Architecture", Analysis: &parser.AgentAnalysis{
			Format: "claude",
			RawContent: []byte("# Architecture\nLayered.\n## Dependencies\nInput: files. Output: analysis."),
			Sections: []parser.Section{
				{Header: "Architecture", Level: 1}, {Header: "Dependencies", Level: 2},
			},
		}},
		{Path: ".agents/guardrails.md", Title: "Guardrails", Analysis: &parser.AgentAnalysis{
			Format: "claude",
			RawContent: []byte("# Guardrails\nTimeout 5min.\n## Constraints\nNever delete.\n## Output Format\nJSON."),
			Sections: []parser.Section{
				{Header: "Guardrails", Level: 1}, {Header: "Constraints", Level: 2},
				{Header: "Output Format", Level: 2},
			},
		}},
		{Path: ".agents/error-handling.md", Title: "Error Handling", Analysis: &parser.AgentAnalysis{
			Format: "claude", RawContent: []byte("# Error Handling\nRetry once."),
			Sections: []parser.Section{{Header: "Error Handling", Level: 1}},
		}},
	}

	agg := ScoreIndex(resolved, "quick")
	assert.Equal(t, 15, agg.Covered, "scaffold should cover all 15 categories, got %d. Uncovered: %v", agg.Covered, agg.Uncovered)
	assert.Equal(t, 15, agg.Total)
	assert.Empty(t, agg.Uncovered, "no categories should be uncovered")
}
