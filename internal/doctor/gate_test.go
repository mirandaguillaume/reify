package doctor

import (
	"testing"

	"github.com/mirandaguillaume/reify/internal/doctor/llmutil"
	"github.com/stretchr/testify/assert"
)

func TestGate_AllPass(t *testing.T) {
	gate := DefaultGate()
	findings := []llmutil.Finding{
		{Category: "ordering", Issue: "Bad ordering", Confidence: "moderate"},
	}

	result := gate.Evaluate(findings)
	assert.True(t, result.Pass)
	assert.Empty(t, result.Failures)
}

func TestGate_HighFindingsWarning(t *testing.T) {
	gate := DefaultGate()
	findings := make([]llmutil.Finding, 0, 6)
	for i := 0; i < 6; i++ {
		findings = append(findings, llmutil.Finding{Category: "specificity", Issue: "Vague", Confidence: "high"})
	}

	result := gate.Evaluate(findings)
	assert.True(t, result.Pass, "high_findings is non-blocking — gate should pass")
	assert.Len(t, result.Warnings, 1)
	assert.Contains(t, result.Warnings[0], "high_findings")
}

func TestGate_CustomBlockingCondition(t *testing.T) {
	gate := &QualityGate{
		Conditions: []GateCondition{
			{Metric: "total_findings", Operator: "<=", Threshold: 3, Blocking: true},
		},
	}
	findings := make([]llmutil.Finding, 5)

	result := gate.Evaluate(findings)
	assert.False(t, result.Pass)
	assert.Contains(t, result.Failures[0], "total_findings")
}

// TestGate_EmptyConditions_Warns: a quality gate with zero conditions must
// NOT silently pass. Silent pass is dangerous because it implies "all checks
// satisfied" when in reality no checks ran. Instead the gate must surface a
// warning so the user knows the configuration is empty.
func TestGate_EmptyConditions_Warns(t *testing.T) {
	gate := &QualityGate{Conditions: []GateCondition{}}
	findings := []llmutil.Finding{}

	result := gate.Evaluate(findings)
	assert.True(t, result.Pass, "empty gate should still pass — there are no failing conditions")
	assert.NotEmpty(t, result.Warnings, "empty gate must produce a warning, not a silent pass")
	assert.Contains(t, result.Warnings[0], "no conditions")
}

func TestGate_UnknownMetricWarns(t *testing.T) {
	gate := &QualityGate{
		Conditions: []GateCondition{
			{Metric: "unknown_metric", Operator: "<=", Threshold: 0, Blocking: true},
		},
	}
	result := gate.Evaluate(nil)
	assert.True(t, result.Pass, "unknown metric is a warning, not a failure")
	assert.NotEmpty(t, result.Warnings)
	assert.Contains(t, result.Warnings[0], "unknown metric")
}
