package checker_test

import (
	"strings"
	"testing"

	"github.com/mirandaguillaume/reify/internal/checker"
	"github.com/mirandaguillaume/reify/internal/classifier"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCheck_EmptyInputReturnsEmptyResult(t *testing.T) {
	r := checker.Check("", "", nil, classifier.Result{})
	assert.Empty(t, r.Instructions)
	assert.Empty(t, r.Overall)
	assert.Empty(t, r.HighRiskCount)
}

func TestCheck_DefaultsToAllHarnessesWhenTargetsEmpty(t *testing.T) {
	content := "## Commands\n- Use tabs for indent"
	cls := classifier.Classify(content, "")
	r := checker.Check(content, "", nil, cls)
	require.Len(t, r.Instructions, 1)

	for _, h := range checker.Harnesses {
		_, ok := r.Instructions[0].Risks[h]
		assert.True(t, ok, "harness %q must be present in Risks", h)
	}
}

func TestCheck_PopulatesAllInstructionFields(t *testing.T) {
	content := "## Guardrails\n- Never commit secrets"
	cls := classifier.Classify(content, "")
	r := checker.Check(content, "", []string{"copilot"}, cls)
	require.Len(t, r.Instructions, 1)

	ir := r.Instructions[0]
	assert.Equal(t, "Never commit secrets", ir.Text)
	assert.Equal(t, classifier.FacetGuardrails, ir.Facet)
	assert.Equal(t, "Guardrails", ir.Section)
	assert.True(t, ir.IsNegative)
	assert.GreaterOrEqual(t, ir.Position, 0.0)
	assert.LessOrEqual(t, ir.Position, 1.0)
	assert.NotNil(t, ir.Risks["copilot"])
	assert.NotNil(t, ir.Factors["copilot"])
}

func TestCheck_OverallTracksWorstRisk(t *testing.T) {
	// Build a file with both a clean instruction and a risky one.
	content := strings.Repeat("filler line\n", 20) +
		"## Guardrails\n" +
		"- Never write unclear code\n" + // negative + semantic + middle → high
		"## Stack\n" +
		"- Go 1.22\n"
	cls := classifier.Classify(content, "")
	r := checker.Check(content, "", []string{"copilot"}, cls)

	require.NotEmpty(t, r.Instructions)
	assert.Equal(t, checker.RiskHigh, r.Overall["copilot"], "worst risk across instructions should be high")
	assert.GreaterOrEqual(t, r.HighRiskCount["copilot"], 1)
}

func TestCheck_HighRiskCount(t *testing.T) {
	// Two negative middle-of-file guardrails → both high on copilot.
	content := strings.Repeat("x\n", 10) +
		"## Guardrails\n" +
		"- Never do X\n" +
		"- Never do Y\n" +
		strings.Repeat("x\n", 10)
	cls := classifier.Classify(content, "")
	r := checker.Check(content, "", []string{"copilot"}, cls)
	assert.Equal(t, 2, r.HighRiskCount["copilot"])
}

func TestCheck_PositionEstimation(t *testing.T) {
	// Instruction near the top should yield a low Position; near the bottom, high.
	lines := []string{"## Top"}
	lines = append(lines, "- Top instruction")
	for i := 0; i < 50; i++ {
		lines = append(lines, "filler")
	}
	lines = append(lines, "## Bottom")
	lines = append(lines, "- Bottom instruction")
	content := strings.Join(lines, "\n")

	cls := classifier.Classify(content, "")
	r := checker.Check(content, "", []string{"claude-code"}, cls)
	require.Len(t, r.Instructions, 2)

	assert.Less(t, r.Instructions[0].Position, 0.25, "top instruction should be in top quartile")
	assert.Greater(t, r.Instructions[1].Position, 0.75, "bottom instruction should be in bottom quartile")
}

func TestCheck_HarnessesConstantStable(t *testing.T) {
	// Locks in the public harness set — if you add/remove one, update the consumers.
	assert.Equal(t, []string{"claude-code", "copilot", "cursor"}, checker.Harnesses)
}
