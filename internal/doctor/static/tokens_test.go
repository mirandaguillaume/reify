package static

import (
	"strings"
	"testing"

	"github.com/mirandaguillaume/reify/internal/doctor/parser"
	"github.com/stretchr/testify/assert"
)

func TestTokens_UnderThreshold(t *testing.T) {
	analysis := &parser.AgentAnalysis{
		RawContent: []byte("Short file."),
		Sections:   []parser.Section{{Header: "Rules", Content: "Be nice"}},
	}
	check := &tokensCheck{}
	assert.Empty(t, check.Run(analysis))
}

func TestTokens_WarnThreshold(t *testing.T) {
	// 8000 tokens ≈ 8000 / 0.78 * 4 ≈ 41,026 chars
	content := strings.Repeat("word ", 8500) // ~42,500 chars
	analysis := &parser.AgentAnalysis{
		RawContent: []byte(content),
		Sections:   []parser.Section{{Header: "Rules", Content: content}},
	}
	check := &tokensCheck{}
	findings := check.Run(analysis)
	assert.True(t, len(findings) >= 1, "expected warning for large file")
}

func TestTokens_Estimation(t *testing.T) {
	// 1000 chars → 1000/4*0.78 = 195 tokens
	assert.Equal(t, 195, estimateTokens(1000))
}

func TestTokens_Indicators(t *testing.T) {
	analysis := &parser.AgentAnalysis{
		RawContent: []byte(strings.Repeat("x", 2000)),
		Sections: []parser.Section{
			{Header: "A", Content: strings.Repeat("x", 1000)},
			{Header: "B", Content: strings.Repeat("x", 1000)},
		},
	}
	check := &tokensCheck{}
	indicators := check.Indicators(analysis)
	assert.Len(t, indicators, 1)
	assert.Equal(t, "Tokens", indicators[0].Name)
	assert.True(t, indicators[0].Value > 0)
}
