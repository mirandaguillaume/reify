package doctor

import (
	"testing"

	"github.com/mirandaguillaume/reify/internal/doctor/llmutil"
	"github.com/stretchr/testify/assert"
)

func TestPostProcess_Dedup(t *testing.T) {
	findings := []llmutil.Finding{
		{Category: "security", Issue: "Missing security", Confidence: "moderate"},
		{Category: "security", Issue: "Missing security", Confidence: "moderate"},
		{Category: "ordering", Issue: "Bad ordering", Confidence: "moderate"},
	}
	result := PostProcess(findings, 20)
	assert.Len(t, result, 2)
}

func TestPostProcess_Sort(t *testing.T) {
	findings := []llmutil.Finding{
		{Category: "z-low", Issue: "Low issue", Confidence: "low"},
		{Category: "a-high", Issue: "High issue", Confidence: "high"},
		{Category: "m-mod", Issue: "Mod issue", Confidence: "moderate"},
	}
	result := PostProcess(findings, 20)
	assert.Equal(t, "high", result[0].Confidence)
	assert.Equal(t, "moderate", result[1].Confidence)
	assert.Equal(t, "low", result[2].Confidence)
}

func TestPostProcess_Limit(t *testing.T) {
	findings := make([]llmutil.Finding, 25)
	for i := range findings {
		findings[i] = llmutil.Finding{Category: "test", Issue: "issue", Confidence: "low"}
	}
	// Make each unique for dedup
	for i := range findings {
		findings[i].Issue = "issue " + string(rune('A'+i))
	}

	result := PostProcess(findings, 5)
	assert.Len(t, result, 6) // 5 + 1 truncation message
	assert.Contains(t, result[5].Issue, "20 more findings")
}

func TestPostProcess_Empty(t *testing.T) {
	result := PostProcess(nil, 20)
	assert.Nil(t, result)
}
