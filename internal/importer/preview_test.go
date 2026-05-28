package importer

import (
	"bytes"
	"strings"
	"testing"

	"github.com/mirandaguillaume/reify/internal/analyzer"
	"github.com/mirandaguillaume/reify/internal/linter"
	"github.com/mirandaguillaume/reify/pkg/model"
	"github.com/stretchr/testify/assert"
)

func TestFormatPreview(t *testing.T) {
	result := ImportResult{
		Success: true,
		Skills: []SkillResult{
			{
				Skill: model.SkillBehavior{
					Skill: "code-review",
					Context: model.ContextFacet{
						Consumes: []string{"pull-request"},
						Produces: []string{"review-comment"},
					},
					Security: model.SecurityFacet{
						Filesystem: model.AccessReadOnly,
						Network:    model.NetworkNone,
					},
				},
				Score: analyzer.SkillScore{
					Skill: "code-review",
					Total: 72,
				},
				LintIssues: nil,
				LoopRisks:  nil,
			},
		},
	}

	var buf bytes.Buffer
	FormatPreview(result, &buf)
	out := buf.String()

	assert.Contains(t, out, "code-review")
	assert.Contains(t, out, "72/100")
	assert.Contains(t, out, "pull-request")
	assert.Contains(t, out, "review-comment")
	assert.Contains(t, out, "✓ all checks pass")
	// No agent block expected.
	assert.NotContains(t, out, "Agent:")
}

func TestFormatPreview_WithLintIssues(t *testing.T) {
	result := ImportResult{
		Success: true,
		Skills: []SkillResult{
			{
				Skill: model.SkillBehavior{
					Skill: "data-fetcher",
					Security: model.SecurityFacet{
						Filesystem: model.AccessFull,
						Network:    model.NetworkFull,
					},
				},
				Score: analyzer.SkillScore{
					Skill: "data-fetcher",
					Total: 30,
				},
				LintIssues: []linter.LintResult{
					{
						Rule:     "no-full-access",
						Severity: linter.SeverityWarning,
						Message:  "full filesystem access is discouraged",
					},
				},
				LoopRisks: []analyzer.LoopRisk{
					{
						Type:     analyzer.LoopNoTimeout,
						Skill:    "data-fetcher",
						Message:  "no timeout guardrail detected",
						Severity: "warning",
					},
				},
			},
		},
	}

	var buf bytes.Buffer
	FormatPreview(result, &buf)
	out := buf.String()

	assert.Contains(t, out, "data-fetcher")
	assert.Contains(t, out, "30/100")
	assert.Contains(t, out, "⚠")
	assert.Contains(t, out, "no-full-access")
	assert.Contains(t, out, "loop risk")
	// Should NOT show "✓ all checks pass" when there are issues.
	assert.NotContains(t, out, "✓ all checks pass")
}

func TestFormatPreview_WithAgent(t *testing.T) {
	result := ImportResult{
		Success: true,
		Skills: []SkillResult{
			{
				Skill: model.SkillBehavior{
					Skill: "fetch-data",
					Context: model.ContextFacet{
						Consumes: []string{"url"},
						Produces: []string{"raw-data"},
					},
					Security: model.SecurityFacet{
						Filesystem: model.AccessNone,
						Network:    model.NetworkAllowlist,
					},
				},
				Score: analyzer.SkillScore{Skill: "fetch-data", Total: 80},
			},
			{
				Skill: model.SkillBehavior{
					Skill: "parse-data",
					Context: model.ContextFacet{
						Consumes: []string{"raw-data"},
						Produces: []string{"structured-data"},
					},
					Security: model.SecurityFacet{
						Filesystem: model.AccessNone,
						Network:    model.NetworkNone,
					},
				},
				Score: analyzer.SkillScore{Skill: "parse-data", Total: 85},
			},
		},
		Agent: &AgentResult{
			Agent: model.AgentComposition{
				Agent:         "data-pipeline",
				Skills:        []string{"fetch-data", "parse-data"},
				Orchestration: model.OrchestrationSequential,
				Consumes:      []string{"url"},
				Produces:      []string{"structured-data"},
			},
			Score:          analyzer.AgentScore{Agent: "data-pipeline", Total: 90},
			DepIssues:      nil,
			OrderingIssues: nil,
		},
	}

	var buf bytes.Buffer
	FormatPreview(result, &buf)
	out := buf.String()

	// Decomposition header.
	assert.Contains(t, out, "Decomposition: Input agent → 2 skill(s) + 1 agent")

	// Skill details.
	assert.Contains(t, out, "fetch-data")
	assert.Contains(t, out, "80/100")
	assert.Contains(t, out, "parse-data")
	assert.Contains(t, out, "85/100")

	// Agent details.
	assert.Contains(t, out, "Agent: data-pipeline")
	assert.Contains(t, out, "90/100")
	assert.Contains(t, out, "sequential")
	assert.Contains(t, out, "fetch-data, parse-data")
	assert.Contains(t, out, "✓ dependencies satisfied")
	assert.Contains(t, out, "✓ skill ordering valid")
}

func TestFormatPreview_WithAgentIssues(t *testing.T) {
	result := ImportResult{
		Success: true,
		Skills:  nil,
		Agent: &AgentResult{
			Agent: model.AgentComposition{
				Agent:         "broken-pipeline",
				Skills:        []string{"missing-skill"},
				Orchestration: model.OrchestrationSequential,
			},
			Score: analyzer.AgentScore{Agent: "broken-pipeline", Total: 20},
			DepIssues: []analyzer.DependencyIssue{
				{
					Type:    analyzer.IssueMissing,
					Skill:   "missing-skill",
					Message: `Agent "broken-pipeline" references skill "missing-skill" which does not exist`,
				},
			},
			OrderingIssues: []analyzer.OrderingIssue{
				{
					Type:     analyzer.OrderDataFlowMismatch,
					Agent:    "broken-pipeline",
					Message:  "ordering problem detected",
					Severity: "warning",
				},
			},
		},
		Warnings: []string{"agent \"broken-pipeline\": missing required field"},
	}

	var buf bytes.Buffer
	FormatPreview(result, &buf)
	out := buf.String()

	assert.Contains(t, out, "broken-pipeline")
	assert.Contains(t, out, "20/100")
	assert.Contains(t, out, "✗ dependency")
	assert.Contains(t, out, "missing-skill")
	assert.Contains(t, out, "⚠ ordering")
	assert.Contains(t, out, "ordering problem detected")
	assert.Contains(t, out, "Warnings:")
	assert.Contains(t, out, "missing required field")
	// No "satisfied" or "ordering valid" when issues exist.
	assert.NotContains(t, out, "✓ dependencies satisfied")
	assert.NotContains(t, out, "✓ skill ordering valid")
}

func TestFormatPreview_NoAgentResultLine(t *testing.T) {
	result := ImportResult{
		Success: true,
		Skills: []SkillResult{
			{
				Skill: model.SkillBehavior{
					Skill:    "simple-skill",
					Security: model.SecurityFacet{Filesystem: model.AccessNone, Network: model.NetworkNone},
				},
				Score: analyzer.SkillScore{Skill: "simple-skill", Total: 50},
			},
		},
	}

	var buf bytes.Buffer
	FormatPreview(result, &buf)
	out := buf.String()

	// No agent means "Result: N skill(s)" header.
	assert.True(t, strings.HasPrefix(out, "Result: 1 skill(s)"))
	assert.NotContains(t, out, "Decomposition:")
}
