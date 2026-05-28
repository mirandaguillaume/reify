package static

import (
	"testing"

	"github.com/mirandaguillaume/reify/internal/doctor/parser"
	"github.com/stretchr/testify/assert"
)

func TestOrdering_CorrectOrder(t *testing.T) {
	// 9 sections — first third = positions 0-2 (indices 0, 1, 2)
	analysis := &parser.AgentAnalysis{
		Sections: []parser.Section{
			{Header: "Identity", Level: 2},     // 1/9 — first third
			{Header: "Guardrails", Level: 2},   // 2/9 — first third
			{Header: "Security", Level: 2},     // 3/9 — first third
			{Header: "Rules", Level: 2},
			{Header: "Examples", Level: 2},
			{Header: "Testing", Level: 2},
			{Header: "Build", Level: 2},
			{Header: "Architecture", Level: 2},
			{Header: "References", Level: 2},
		},
	}

	check := &orderingCheck{}
	findings := check.Run(analysis)
	assert.Empty(t, findings, "critical sections in first third — no findings")
}

func TestOrdering_BadOrder(t *testing.T) {
	analysis := &parser.AgentAnalysis{
		Sections: []parser.Section{
			{Header: "Examples", Level: 2},
			{Header: "Testing", Level: 2},
			{Header: "References", Level: 2},
			{Header: "Tools", Level: 2},
			{Header: "Guardrails", Level: 2},  // position 5/6 — should be in first third
			{Header: "Security", Level: 2},     // position 6/6 — should be in first third
		},
	}

	check := &orderingCheck{}
	findings := check.Run(analysis)
	assert.True(t, len(findings) >= 2, "guardrails and security are late — expected findings, got %d", len(findings))
}

func TestOrdering_MissingSections_NoFalsePositive(t *testing.T) {
	// Critical sections are missing — presence check handles this, ordering should not flag
	analysis := &parser.AgentAnalysis{
		Sections: []parser.Section{
			{Header: "Examples", Level: 2},
			{Header: "Testing", Level: 2},
			{Header: "References", Level: 2},
			{Header: "Tools", Level: 2},
		},
	}

	check := &orderingCheck{}
	findings := check.Run(analysis)
	assert.Empty(t, findings, "missing sections should not produce ordering findings")
}

func TestOrdering_TooFewSections(t *testing.T) {
	analysis := &parser.AgentAnalysis{
		Sections: []parser.Section{
			{Header: "Rules", Level: 2},
			{Header: "Security", Level: 2},
		},
	}

	check := &orderingCheck{}
	findings := check.Run(analysis)
	assert.Empty(t, findings, "less than 3 sections — skip ordering check")
}

func TestOrdering_NilAnalysis(t *testing.T) {
	check := &orderingCheck{}
	assert.Nil(t, check.Run(nil))
}
