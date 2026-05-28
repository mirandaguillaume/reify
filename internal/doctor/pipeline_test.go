package doctor

import (
	"fmt"
	"testing"

	"github.com/mirandaguillaume/reify/internal/doctor/llmutil"
	"github.com/mirandaguillaume/reify/internal/doctor/parser"
	"github.com/stretchr/testify/assert"
)

func TestRunPipeline_QuickMode(t *testing.T) {
	analysis := &parser.AgentAnalysis{
		Format:     "claude",
		RawContent: []byte("## Rules\nBe nice"),
		Sections:   []parser.Section{{Header: "Rules", Level: 2}},
	}

	report := RunPipeline(analysis, nil, PipelineOpts{Mode: "quick", SectionCount: 15})

	assert.Equal(t, "claude", report.Format)
	assert.True(t, len(report.StaticFindings) > 0, "quick mode should produce static findings")
	assert.Empty(t, report.LLMFindings)
	assert.True(t, report.StructuralScore.Total > 0)
}

func TestRunPipeline_WithLLMFindings(t *testing.T) {
	analysis := &parser.AgentAnalysis{
		Format:     "claude",
		RawContent: []byte("## Security\nRestricted.\n## Rules\nBe nice"),
		Sections: []parser.Section{
			{Header: "Security", Level: 2},
			{Header: "Rules", Level: 2},
		},
	}

	llmFindings := []llmutil.Finding{
		{Category: "specificity", Issue: "Vague instruction", Confidence: "moderate"},
	}

	report := RunPipeline(analysis, llmFindings, PipelineOpts{Mode: "default", SectionCount: 15})

	assert.True(t, len(report.AllFindings) > 0)
	// Should contain both static and LLM findings
	hasLLM := false
	for _, f := range report.AllFindings {
		if f.Issue == "Vague instruction" {
			hasLLM = true
		}
	}
	assert.True(t, hasLLM, "merged findings should include LLM findings")
}

func TestRunPipeline_GatePass(t *testing.T) {
	analysis := &parser.AgentAnalysis{
		Format: "claude",
		RawContent: []byte("## Identity\nYou are a reviewer.\n## Security\nRestricted.\n## Guardrails\nTimeout 5min.\n## Testing\nRun go test.\n## Examples\nHere.\n## Error Handling\nRetry.\n## Build\ngo build.\n## Architecture\nLayered.\n## Output\nJSON.\n## Decision\nEscalate.\n## Trigger\nOn PR.\n## Dependencies\nInput/output.\n## Memory\nSession.\n## Goals\nQuality.\n## Constraints\nNever delete."),
		Sections: []parser.Section{
			{Header: "Identity", Level: 2}, {Header: "Security", Level: 2},
			{Header: "Guardrails", Level: 2}, {Header: "Testing", Level: 2},
			{Header: "Examples", Level: 2}, {Header: "Error Handling", Level: 2},
			{Header: "Build", Level: 2}, {Header: "Architecture", Level: 2},
			{Header: "Output", Level: 2}, {Header: "Decision", Level: 2},
			{Header: "Trigger", Level: 2}, {Header: "Dependencies", Level: 2},
			{Header: "Memory", Level: 2}, {Header: "Goals", Level: 2},
			{Header: "Constraints", Level: 2},
		},
	}

	report := RunPipeline(analysis, nil, PipelineOpts{Mode: "quick", SectionCount: 15})
	assert.True(t, report.GateResult.Pass, "all sections present, no secrets → gate should pass")
}

func TestRunPipeline_DefaultGate(t *testing.T) {
	report := RunPipeline(
		&parser.AgentAnalysis{Format: "claude", RawContent: []byte("x")},
		nil,
		PipelineOpts{Mode: "quick"},
	)
	// Default gate should be applied
	assert.NotNil(t, report.GateResult)
}

// TestRunPipeline_NilAnalysis_NoPanic verifies AC #1 (Story 4-0) — RunPipeline
// must nil-guard the analysis parameter and return an empty Report instead of
// panicking. This catches a real defect: prior versions dereferenced
// analysis.Format unconditionally.
func TestRunPipeline_NilAnalysis_NoPanic(t *testing.T) {
	assert.NotPanics(t, func() {
		report := RunPipeline(nil, nil, PipelineOpts{Mode: "quick"})
		assert.NotNil(t, report, "must return non-nil Report even on nil analysis")
		assert.Empty(t, report.StaticFindings)
		assert.Empty(t, report.LLMFindings)
		assert.Empty(t, report.AllFindings)
		// GateResult must pass — a zero-value GateResult has Pass=false which would
		// trigger ErrFindings in callers even though there are no findings. (Review patch)
		assert.True(t, report.GateResult.Pass, "nil-analysis Report must have GateResult.Pass=true")
	})
}

// TestRunPipeline_GateSeesPreTruncationFindings verifies AC #8 (Story 4-0) —
// the quality gate must evaluate against the *full* merged findings list, not
// the post-truncated AllFindings. This locks in the current correct behavior
// so future changes can't accidentally regress and gate against truncated input.
//
// Setup: 25 unique LLM findings, MaxFindings=3. The gate has a custom condition
// `total_findings <= 10`. If gate evaluates pre-truncation it sees ≥25 → fails.
// If gate evaluates post-truncation it sees ≤4 (3 + summary) → passes incorrectly.
//
// Findings are made unique so dedup (PostProcess) doesn't collapse them and
// hide the bug.
func TestRunPipeline_GateSeesPreTruncationFindings(t *testing.T) {
	llmFindings := make([]llmutil.Finding, 25)
	for i := 0; i < 25; i++ {
		llmFindings[i] = llmutil.Finding{
			Category:   "specificity",
			Issue:      fmt.Sprintf("Vague instruction #%d", i), // unique to defeat dedup
			Confidence: "moderate",
		}
	}

	customGate := &QualityGate{
		Conditions: []GateCondition{
			{Metric: "total_findings", Operator: "<=", Threshold: 10, Blocking: true},
		},
	}

	report := RunPipeline(
		&parser.AgentAnalysis{
			Format:     "claude",
			RawContent: []byte("## Rules\nx"),
			Sections:   []parser.Section{{Header: "Rules", Level: 2}},
		},
		llmFindings,
		PipelineOpts{Mode: "quick", MaxFindings: 3, Gate: customGate, SectionCount: 15},
	)

	// Display list is truncated to MaxFindings + 1 (PostProcess appends a "summary"
	// finding noting how many were hidden)
	assert.LessOrEqual(t, len(report.AllFindings), 4, "display list should be ≤ MaxFindings + summary marker")

	// Gate must see all merged findings → fail because >> 10
	assert.False(t, report.GateResult.Pass, "gate must evaluate against pre-truncation findings (full merged list)")
	assert.NotEmpty(t, report.GateResult.Failures)
	assert.Contains(t, report.GateResult.Failures[0], "total_findings")
}
