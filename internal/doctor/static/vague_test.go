package static

import (
	"testing"

	"github.com/mirandaguillaume/reify/internal/doctor/parser"
	"github.com/stretchr/testify/assert"
)

func TestVague_DetectsPatterns(t *testing.T) {
	analysis := &parser.AgentAnalysis{
		RawContent: []byte("## Rules\nBe thorough when reviewing code.\nIf needed, add tests.\nUse your judgment for edge cases."),
	}

	check := &vagueCheck{}
	findings := check.Run(analysis)
	assert.Len(t, findings, 3)
	assert.Contains(t, findings[0].Issue, "be thorough")
	assert.Contains(t, findings[1].Issue, "if needed")
	assert.Contains(t, findings[2].Issue, "use your judgment")
}

func TestVague_SkipsCodeBlocks(t *testing.T) {
	analysis := &parser.AgentAnalysis{
		RawContent: []byte("## Rules\nBe specific.\n```\nBe thorough in this example\nIf needed, do stuff\n```\nMore instructions."),
	}

	check := &vagueCheck{}
	findings := check.Run(analysis)
	assert.Empty(t, findings, "vague patterns inside code fences should be skipped")
}

func TestVague_OnePerLine(t *testing.T) {
	// Line has two patterns — should only report one finding per line
	analysis := &parser.AgentAnalysis{
		RawContent: []byte("Be thorough and if needed add more."),
	}

	check := &vagueCheck{}
	findings := check.Run(analysis)
	assert.Len(t, findings, 1, "one finding per line even with multiple matches")
}

func TestVague_NoMatches(t *testing.T) {
	analysis := &parser.AgentAnalysis{
		RawContent: []byte("## Rules\nAlways run tests before committing.\nNever skip the linter.\nUse gofmt for formatting."),
	}

	check := &vagueCheck{}
	findings := check.Run(analysis)
	assert.Empty(t, findings)
}

func TestVague_NilAnalysis(t *testing.T) {
	check := &vagueCheck{}
	assert.Nil(t, check.Run(nil))
}

func TestVague_EmptyContent(t *testing.T) {
	check := &vagueCheck{}
	assert.Nil(t, check.Run(&parser.AgentAnalysis{RawContent: []byte{}}))
}
