package checker

import (
	"testing"

	"github.com/mirandaguillaume/reify/internal/classifier"
	"github.com/stretchr/testify/assert"
)

func TestRiskLevel_String(t *testing.T) {
	assert.Equal(t, "low", RiskLow.String())
	assert.Equal(t, "medium", RiskMedium.String())
	assert.Equal(t, "high", RiskHigh.String())
	assert.Equal(t, "unknown", RiskLevel(99).String())
}

func TestRiskFactors_Count(t *testing.T) {
	cases := []struct {
		name string
		f    RiskFactors
		want int
	}{
		{"none", RiskFactors{}, 0},
		{"one-negative", RiskFactors{NegativeFraming: true}, 1},
		{"two-mixed", RiskFactors{NegativeFraming: true, MiddlePosition: true}, 2},
		{"all-four", RiskFactors{NegativeFraming: true, MiddlePosition: true, SemanticConstraint: true, HarnessWeakness: true}, 4},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.want, tc.f.Count())
		})
	}
}

func TestRiskFactors_ActiveFactors(t *testing.T) {
	f := RiskFactors{NegativeFraming: true, MiddlePosition: true, SemanticConstraint: false, HarnessWeakness: true}
	got := f.ActiveFactors()
	assert.Len(t, got, 3)
	assert.Contains(t, got[0], "negative framing")
	assert.Contains(t, got[1], "middle of file")
	assert.Contains(t, got[2], "empirically weaker")

	assert.Nil(t, RiskFactors{}.ActiveFactors())
}

func TestIsNegativeInstruction(t *testing.T) {
	cases := []struct {
		text string
		want bool
	}{
		{"Never commit secrets", true},
		{"never commit", true}, // case-insensitive
		{"  Never violate  ", true}, // trimmed before prefix check
		{"Don't push to main", true},
		{"Do not bypass auth", true},
		{"Avoid using globals", true},
		{"Must not skip review", true},
		{"You should never push", true},     // " never " contains
		{"You do not need approval", true},  // " do not " contains
		{"Always run tests", false},
		{"Use semicolons", false},
		{"Run go test", false},
		{"", false},
	}
	for _, tc := range cases {
		t.Run(tc.text, func(t *testing.T) {
			assert.Equal(t, tc.want, isNegativeInstruction(tc.text))
		})
	}
}

func TestIsVerifiableInstruction(t *testing.T) {
	// Security facet is always considered verifiable.
	assert.True(t, isVerifiableInstruction("anything goes here", classifier.FacetSecurity))

	// Verifiable keywords on non-security facets.
	verifiable := []string{
		"Use 2 spaces for indent",
		"Add import for fmt",
		"Use tab not space",
		"Files must use .test. suffix",
		"Test file naming: _test.",
		"Use type interface",
		"Add copyright header",
	}
	for _, txt := range verifiable {
		assert.True(t, isVerifiableInstruction(txt, classifier.FacetStrategy), "should be verifiable: %q", txt)
	}

	// Non-verifiable on non-security facets.
	nonVerifiable := []string{
		"Write clean code",
		"Be helpful",
		"Think carefully",
	}
	for _, txt := range nonVerifiable {
		assert.False(t, isVerifiableInstruction(txt, classifier.FacetStrategy), "should NOT be verifiable: %q", txt)
	}
}

func TestAssessRisk_ZeroFactorsIsLow(t *testing.T) {
	ir := InstructionResult{
		Text:         "Use 2 spaces for indent",
		Facet:        classifier.FacetStrategy,
		Position:     0.10, // top of file → no MiddlePosition
		IsNegative:   false,
		IsVerifiable: true, // → no SemanticConstraint
	}
	level, f := AssessRisk(ir, "claude-code")
	assert.Equal(t, RiskLow, level)
	assert.Equal(t, 0, f.Count())
}

func TestAssessRisk_OneFactorIsMedium(t *testing.T) {
	ir := InstructionResult{
		Text:         "Use 2 spaces for indent",
		Facet:        classifier.FacetStrategy,
		Position:     0.50, // middle → MiddlePosition
		IsVerifiable: true,
	}
	level, f := AssessRisk(ir, "claude-code")
	assert.Equal(t, RiskMedium, level)
	assert.Equal(t, 1, f.Count())
	assert.True(t, f.MiddlePosition)
}

func TestAssessRisk_TwoFactorsIsHigh(t *testing.T) {
	ir := InstructionResult{
		Text:         "Never write unclear code",
		Facet:        classifier.FacetGuardrails,
		Position:     0.50, // middle
		IsNegative:   true, // negative framing
		IsVerifiable: false,
	}
	level, _ := AssessRisk(ir, "claude-code")
	assert.Equal(t, RiskHigh, level)
}

func TestAssessRisk_ContextFacetDoesNotTriggerSemanticConstraint(t *testing.T) {
	ir := InstructionResult{
		Text:         "Project uses Go 1.22",
		Facet:        classifier.FacetContext,
		Position:     0.10,
		IsVerifiable: false,
	}
	_, f := AssessRisk(ir, "claude-code")
	assert.False(t, f.SemanticConstraint, "context facet should not be flagged as semantic constraint")
}

func TestHarnessWeakness_Copilot(t *testing.T) {
	// Negative → weak.
	assert.True(t, harnessWeakness(InstructionResult{IsNegative: true}, "copilot"))
	// Guardrail in late position → weak.
	assert.True(t, harnessWeakness(InstructionResult{Position: 0.80, Facet: classifier.FacetGuardrails}, "copilot"))
	// Same guardrail at top → not weak.
	assert.False(t, harnessWeakness(InstructionResult{Position: 0.20, Facet: classifier.FacetGuardrails}, "copilot"))
}

func TestHarnessWeakness_Cursor(t *testing.T) {
	assert.True(t, harnessWeakness(InstructionResult{IsNegative: true}, "cursor"))
	assert.False(t, harnessWeakness(InstructionResult{IsNegative: false}, "cursor"))
}

func TestHarnessWeakness_ClaudeCode(t *testing.T) {
	// Claude-code has no observed weaknesses encoded.
	assert.False(t, harnessWeakness(InstructionResult{IsNegative: true, Position: 0.9, Facet: classifier.FacetGuardrails}, "claude-code"))
}

func TestHarnessWeakness_UnknownHarness(t *testing.T) {
	assert.False(t, harnessWeakness(InstructionResult{IsNegative: true}, "made-up-harness"))
}

func TestDedupe(t *testing.T) {
	assert.Nil(t, dedupe(nil))
	assert.Equal(t, []string{"a"}, dedupe([]string{"a", "a", "a"}))
	assert.Equal(t, []string{"a", "b", "c"}, dedupe([]string{"a", "b", "a", "c", "b"}))
}

func TestBuildSuggestions_NegativeHighRiskSuggestsReframe(t *testing.T) {
	ir := InstructionResult{
		Text:       "Never do X",
		Facet:      classifier.FacetGuardrails,
		Position:   0.50, // middle (factor) + negative (factor) + harness weakness for copilot → high on copilot
		IsNegative: true,
	}
	got := buildSuggestions(ir, []string{"copilot"})
	require := assert.NotEmpty
	require(t, got)
	assert.Contains(t, got[0], "Reframe as positive")
}

func TestBuildSuggestions_LateGuardrailSuggestsMove(t *testing.T) {
	ir := InstructionResult{
		Text:     "Use tabs",
		Facet:    classifier.FacetGuardrails,
		Position: 0.80,
	}
	got := buildSuggestions(ir, []string{"claude-code"})
	found := false
	for _, s := range got {
		if assert.Contains(t, s, "Move to top 25%") {
			found = true
		}
	}
	assert.True(t, found)
}

func TestBuildSuggestions_NoIssuesNoSuggestions(t *testing.T) {
	ir := InstructionResult{
		Text:         "Use 2 spaces",
		Facet:        classifier.FacetStrategy,
		Position:     0.10,
		IsVerifiable: true,
	}
	assert.Empty(t, buildSuggestions(ir, Harnesses))
}

func TestBuildSuggestions_DedupesAcrossTargets(t *testing.T) {
	// Same suggestion would fire on multiple harnesses; result should appear once.
	ir := InstructionResult{
		Text:       "Never do X",
		Facet:      classifier.FacetGuardrails,
		Position:   0.50,
		IsNegative: true,
	}
	got := buildSuggestions(ir, []string{"copilot", "cursor"})
	count := 0
	for _, s := range got {
		if s == `Reframe as positive — "always do X" instead of "never do Y" (IFEval shows better compliance)` {
			count++
		}
	}
	assert.Equal(t, 1, count, "reframe suggestion must not duplicate per-target")
}
