package checker_test

import (
	"strings"
	"testing"

	"github.com/mirandaguillaume/reify/internal/checker"
	"github.com/mirandaguillaume/reify/internal/classifier"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// resultOf constructs a classifier.Result manually so checker tests don't
// depend on a classifier implementation. Item facet defaults to context
// unless caller passes one.
func resultOf(items ...classifier.Item) classifier.Result {
	return classifier.Result{Items: items}
}

func TestCheck_EmptyInputReturnsEmptyResult(t *testing.T) {
	r := checker.Check("", "", nil, classifier.Result{})
	assert.Empty(t, r.Instructions)
	assert.Empty(t, r.Overall)
	assert.Empty(t, r.HighRiskCount)
}

func TestCheck_DefaultsToAllHarnessesWhenTargetsEmpty(t *testing.T) {
	content := "## Commands\n- Use tabs for indent"
	cls := resultOf(classifier.Item{Text: "Use tabs for indent", Facet: classifier.FacetStrategy, Section: "Commands"})
	r := checker.Check(content, "", nil, cls)
	require.Len(t, r.Instructions, 1)

	for _, h := range checker.Harnesses {
		_, ok := r.Instructions[0].Risks[h]
		assert.True(t, ok, "harness %q must be present in Risks", h)
	}
}

func TestCheck_PopulatesAllInstructionFields(t *testing.T) {
	content := "## Guardrails\n- Never commit secrets"
	cls := resultOf(classifier.Item{Text: "Never commit secrets", Facet: classifier.FacetGuardrails, Section: "Guardrails"})
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
	content := strings.Repeat("filler line\n", 20) +
		"## Guardrails\n" +
		"- Never write unclear code\n" +
		"## Stack\n" +
		"- Go 1.22\n"
	cls := resultOf(
		classifier.Item{Text: "Never write unclear code", Facet: classifier.FacetGuardrails, Section: "Guardrails"},
		classifier.Item{Text: "Go 1.22", Facet: classifier.FacetContext, Section: "Stack"},
	)
	r := checker.Check(content, "", []string{"copilot"}, cls)

	require.NotEmpty(t, r.Instructions)
	assert.Equal(t, checker.RiskHigh, r.Overall["copilot"], "worst risk across instructions should be high")
	assert.GreaterOrEqual(t, r.HighRiskCount["copilot"], 1)
}

func TestCheck_HighRiskCount(t *testing.T) {
	content := strings.Repeat("x\n", 10) +
		"## Guardrails\n" +
		"- Never do X\n" +
		"- Never do Y\n" +
		strings.Repeat("x\n", 10)
	cls := resultOf(
		classifier.Item{Text: "Never do X", Facet: classifier.FacetGuardrails, Section: "Guardrails"},
		classifier.Item{Text: "Never do Y", Facet: classifier.FacetGuardrails, Section: "Guardrails"},
	)
	r := checker.Check(content, "", []string{"copilot"}, cls)
	assert.Equal(t, 2, r.HighRiskCount["copilot"])
}

func TestCheck_PositionEstimation(t *testing.T) {
	lines := []string{"## Top"}
	lines = append(lines, "- Top instruction")
	for i := 0; i < 50; i++ {
		lines = append(lines, "filler")
	}
	lines = append(lines, "## Bottom")
	lines = append(lines, "- Bottom instruction")
	content := strings.Join(lines, "\n")

	cls := resultOf(
		classifier.Item{Text: "Top instruction", Facet: classifier.FacetContext, Section: "Top"},
		classifier.Item{Text: "Bottom instruction", Facet: classifier.FacetContext, Section: "Bottom"},
	)
	r := checker.Check(content, "", []string{"claude-code"}, cls)
	require.Len(t, r.Instructions, 2)

	assert.Less(t, r.Instructions[0].Position, 0.25, "top instruction should be in top quartile")
	assert.Greater(t, r.Instructions[1].Position, 0.75, "bottom instruction should be in bottom quartile")
}

func TestCheck_HarnessesConstantStable(t *testing.T) {
	assert.Equal(t, []string{"claude-code", "copilot", "cursor"}, checker.Harnesses)
}
