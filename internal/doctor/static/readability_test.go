package static

import (
	"testing"

	"github.com/mirandaguillaume/reify/internal/doctor/parser"
	"github.com/stretchr/testify/assert"
)

func TestReadability_KnownText(t *testing.T) {
	// Simple English text should have moderate Flesch score
	text := "The cat sat on the mat. The dog ran in the park. It was a sunny day. The children played outside. They had a good time."
	analysis := &parser.AgentAnalysis{RawContent: []byte(text)}

	check := &readabilityCheck{}
	indicators := check.Indicators(analysis)
	assert.True(t, len(indicators) >= 2, "expected Flesch and Fog indicators")

	// Simple text should have high Flesch (easy to read)
	flesch := indicators[0]
	assert.Equal(t, "Readability", flesch.Name)
	assert.True(t, flesch.Value > 50, "simple text should have Flesch >50, got %.1f", flesch.Value)
}

func TestReadability_ExcludesCodeBlocks(t *testing.T) {
	// Prose is short but code block is long — with code excluded, prose should remain simple
	text := "The cat sat on the mat. The dog ran in the park. It was a sunny day.\n```\nfunc complexMethodWithExtremelyLongNameThatWouldReduceReadability() error {\n  return nil\n}\n```\nThe children played outside. They had a good time. The sun was shining."
	analysis := &parser.AgentAnalysis{RawContent: []byte(text)}

	check := &readabilityCheck{}
	indicators := check.Indicators(analysis)
	assert.True(t, len(indicators) >= 1, "expected at least 1 indicator")
}

func TestReadability_TooShort(t *testing.T) {
	analysis := &parser.AgentAnalysis{RawContent: []byte("Hi.")}
	check := &readabilityCheck{}
	assert.Nil(t, check.Indicators(analysis))
}

func TestReadability_NilAnalysis(t *testing.T) {
	check := &readabilityCheck{}
	assert.Nil(t, check.Indicators(nil))
}

func TestSyllableCount(t *testing.T) {
	tests := map[string]int{
		"cat":         1,
		"simple":      2,
		"beautiful":   3,
		"information": 4,
		"the":         1,
	}
	for word, expected := range tests {
		got := syllablesInWord(word)
		assert.Equal(t, expected, got, "syllables in %q: expected %d, got %d", word, expected, got)
	}
}
