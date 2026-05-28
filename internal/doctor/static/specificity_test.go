package static

import (
	"testing"

	"github.com/mirandaguillaume/reify/internal/doctor/parser"
	"github.com/stretchr/testify/assert"
)

func TestSpecificity_HighDirective(t *testing.T) {
	content := "You must always run tests. You must never skip the linter. Always ensure code is formatted."
	analysis := &parser.AgentAnalysis{RawContent: []byte(content)}

	check := &specificityCheck{}
	indicators := check.Indicators(analysis)
	assert.Len(t, indicators, 1)
	assert.True(t, indicators[0].Value > 0.8, "highly directive text should have ratio >0.8, got %.2f", indicators[0].Value)
}

func TestSpecificity_HighAdvisory(t *testing.T) {
	content := "You may consider running tests. You could perhaps use a linter. It might be suggested to format code."
	analysis := &parser.AgentAnalysis{RawContent: []byte(content)}

	check := &specificityCheck{}
	indicators := check.Indicators(analysis)
	assert.Len(t, indicators, 1)
	assert.True(t, indicators[0].Value < 0.3, "highly advisory text should have ratio <0.3, got %.2f", indicators[0].Value)
}

func TestSpecificity_NoKeywords(t *testing.T) {
	content := "The cat sat on the mat."
	analysis := &parser.AgentAnalysis{RawContent: []byte(content)}

	check := &specificityCheck{}
	indicators := check.Indicators(analysis)
	assert.Nil(t, indicators, "no directive/advisory words = no indicator")
}

func TestSpecificity_NilAnalysis(t *testing.T) {
	check := &specificityCheck{}
	assert.Nil(t, check.Indicators(nil))
}
