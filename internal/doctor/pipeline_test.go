package doctor

import (
	"fmt"
	"testing"

	"github.com/mirandaguillaume/reify/internal/doctor/llmutil"
	"github.com/mirandaguillaume/reify/internal/doctor/parser"
	"github.com/stretchr/testify/assert"
)

func TestRunPipeline_LLMFindingsAreCarriedThrough(t *testing.T) {
	analysis := &parser.AgentAnalysis{
		Format:     "claude",
		RawContent: []byte("## Rules\nBe nice"),
		Sections:   []parser.Section{{Header: "Rules", Level: 2}},
	}
	llmFindings := []llmutil.Finding{
		{Category: "specificity", Issue: "Vague instruction", Confidence: "moderate"},
	}

	report := RunPipeline(analysis, llmFindings, PipelineOpts{})

	assert.Equal(t, "claude", report.Format)
	assert.Len(t, report.LLMFindings, 1)
	assert.True(t, len(report.AllFindings) > 0)
}

func TestRunPipeline_NoFindings_GatePasses(t *testing.T) {
	analysis := &parser.AgentAnalysis{Format: "claude", RawContent: []byte("x")}
	report := RunPipeline(analysis, nil, PipelineOpts{})
	assert.True(t, report.GateResult.Pass)
}

// TestRunPipeline_NilAnalysis_NoPanic: RunPipeline must nil-guard the
// analysis parameter and return an empty Report instead of panicking.
func TestRunPipeline_NilAnalysis_NoPanic(t *testing.T) {
	assert.NotPanics(t, func() {
		report := RunPipeline(nil, nil, PipelineOpts{})
		assert.NotNil(t, report)
		assert.Empty(t, report.LLMFindings)
		assert.Empty(t, report.AllFindings)
		assert.True(t, report.GateResult.Pass, "nil-analysis Report must have GateResult.Pass=true")
	})
}

// TestRunPipeline_GateSeesPreTruncationFindings: the quality gate must
// evaluate against the *full* merged findings list, not the post-truncated
// AllFindings. Locks in current correct behaviour.
func TestRunPipeline_GateSeesPreTruncationFindings(t *testing.T) {
	llmFindings := make([]llmutil.Finding, 25)
	for i := 0; i < 25; i++ {
		llmFindings[i] = llmutil.Finding{
			Category:   "specificity",
			Issue:      fmt.Sprintf("Vague instruction #%d", i),
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
		PipelineOpts{MaxFindings: 3, Gate: customGate},
	)

	assert.LessOrEqual(t, len(report.AllFindings), 4, "display list should be ≤ MaxFindings + summary marker")
	assert.False(t, report.GateResult.Pass, "gate must evaluate against pre-truncation findings")
	assert.NotEmpty(t, report.GateResult.Failures)
	assert.Contains(t, report.GateResult.Failures[0], "total_findings")
}
