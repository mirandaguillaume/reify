package doctor

import (
	"testing"

	"github.com/mirandaguillaume/reify/internal/doctor/llmutil"
	"github.com/stretchr/testify/assert"
)

func TestGate_AllPass(t *testing.T) {
	gate := DefaultGate()
	structural := StructuralResult{Passed: 14, Total: 16}
	findings := []llmutil.Finding{
		{Category: "ordering", Issue: "Bad ordering", Confidence: "moderate"},
	}

	result := gate.Evaluate(structural, findings)
	assert.True(t, result.Pass)
	assert.Empty(t, result.Failures)
}

func TestGate_SecretsFail(t *testing.T) {
	gate := DefaultGate()
	structural := StructuralResult{Passed: 14, Total: 16}
	findings := []llmutil.Finding{
		{Category: "security", Issue: "Possible OpenAI API key detected on line 5", Confidence: "high"},
	}

	result := gate.Evaluate(structural, findings)
	assert.False(t, result.Pass, "secrets should fail gate")
	assert.Len(t, result.Failures, 1)
	assert.Contains(t, result.Failures[0], "secrets_found")
}

func TestGate_LowStructuralFail(t *testing.T) {
	gate := DefaultGate()
	structural := StructuralResult{Passed: 3, Total: 16} // 18.75%
	findings := []llmutil.Finding{}

	result := gate.Evaluate(structural, findings)
	assert.False(t, result.Pass, "low structural score should fail gate")
	assert.Len(t, result.Failures, 1)
	assert.Contains(t, result.Failures[0], "structural_pct")
}

func TestGate_HighFindingsWarning(t *testing.T) {
	gate := DefaultGate()
	structural := StructuralResult{Passed: 14, Total: 16}
	findings := make([]llmutil.Finding, 0, 6)
	for i := 0; i < 6; i++ {
		findings = append(findings, llmutil.Finding{Category: "specificity", Issue: "Vague", Confidence: "high"})
	}

	result := gate.Evaluate(structural, findings)
	assert.True(t, result.Pass, "high_findings is non-blocking — gate should pass")
	assert.Len(t, result.Warnings, 1)
	assert.Contains(t, result.Warnings[0], "high_findings")
}

func TestGate_CustomConditions(t *testing.T) {
	gate := &QualityGate{
		Conditions: []GateCondition{
			{Metric: "total_findings", Operator: "<=", Threshold: 3, Blocking: true},
		},
	}
	structural := StructuralResult{Passed: 10, Total: 16}
	findings := make([]llmutil.Finding, 5)

	result := gate.Evaluate(structural, findings)
	assert.False(t, result.Pass)
	assert.Contains(t, result.Failures[0], "total_findings")
}

// TestGate_EmptyConditions_Warns verifies AC #4 (Story 4-0) — a quality gate
// with zero conditions must NOT silently pass. Silent pass is dangerous because
// it implies "all checks satisfied" when in reality no checks ran. Instead the
// gate must surface a warning so the user knows the configuration is empty.
func TestGate_EmptyConditions_Warns(t *testing.T) {
	gate := &QualityGate{Conditions: []GateCondition{}}
	structural := StructuralResult{Passed: 10, Total: 16}
	findings := []llmutil.Finding{}

	result := gate.Evaluate(structural, findings)
	// Pass remains true (no failed conditions), but warning surfaces the empty config
	assert.True(t, result.Pass, "empty gate should still pass — there are no failing conditions")
	assert.NotEmpty(t, result.Warnings, "empty gate must produce a warning, not a silent pass")
	assert.Contains(t, result.Warnings[0], "no conditions")
}

// TestIsSecretFinding verifies AC #3 (Story 4-0) — the shared helper that
// classifies a finding as a "detected secret". Used by both gate.Evaluate
// (via computeMetrics) and ComputeStructural — must stay consistent across
// both call sites.
func TestIsSecretFinding(t *testing.T) {
	// Positive cases
	assert.True(t, isSecretFinding(llmutil.Finding{Category: "security", Issue: "OpenAI key detected on line 5"}))
	assert.True(t, isSecretFinding(llmutil.Finding{Category: "security", Issue: "AWS secret detected"}))

	// Negative: wrong category
	assert.False(t, isSecretFinding(llmutil.Finding{Category: "ordering", Issue: "key detected"}))
	// Negative: security category but no "detected" word
	assert.False(t, isSecretFinding(llmutil.Finding{Category: "security", Issue: "Possible exposure"}))
	// Negative: empty
	assert.False(t, isSecretFinding(llmutil.Finding{}))
}
