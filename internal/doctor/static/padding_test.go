package static

import (
	"testing"

	"github.com/mirandaguillaume/reify/internal/doctor/parser"
	"github.com/stretchr/testify/assert"
)

func TestPadding_DetectsFillerPhrases(t *testing.T) {
	analysis := &parser.AgentAnalysis{
		RawContent: []byte("Make sure to run tests.\nIt is important to validate.\nPlease note that this matters."),
	}
	check := &paddingCheck{}
	findings := check.Run(analysis)
	assert.Len(t, findings, 3)
	assert.Contains(t, findings[0].Issue, "make sure to")
}

func TestPadding_SkipsCodeBlocks(t *testing.T) {
	analysis := &parser.AgentAnalysis{
		RawContent: []byte("## Rules\nBe direct.\n```\nMake sure to initialize.\n```\nMore rules."),
	}
	check := &paddingCheck{}
	findings := check.Run(analysis)
	assert.Empty(t, findings, "filler inside code blocks should be skipped")
}

func TestPadding_NoFiller(t *testing.T) {
	analysis := &parser.AgentAnalysis{
		RawContent: []byte("Always run tests.\nNever skip linting.\nUse gofmt."),
	}
	check := &paddingCheck{}
	assert.Empty(t, check.Run(analysis))
}

func TestPadding_NilAnalysis(t *testing.T) {
	check := &paddingCheck{}
	assert.Nil(t, check.Run(nil))
}

func TestPadding_RealWorldFillerPhrases(t *testing.T) {
	// Real-world filler patterns found in GitHub agent files.
	// Each line tests a different pattern from the paddingPatterns list.
	analysis := &parser.AgentAnalysis{
		RawContent: []byte(`## Guidelines
Be sure to check for edge cases in all code.
Don't forget to run the test suite.
Remember to update the documentation.
You should always follow the coding standards.
It is essential to validate user inputs.
It is worth mentioning that performance matters.
It should be noted that security is a priority.
`),
	}
	check := &paddingCheck{}
	findings := check.Run(analysis)
	assert.GreaterOrEqual(t, len(findings), 7,
		"should detect at least 7 filler phrases in real-world text")
}

func TestPadding_MixedWithRealContent(t *testing.T) {
	// Real agent file excerpt with some filler mixed in with legitimate content.
	analysis := &parser.AgentAnalysis{
		RawContent: []byte(`## Error Handling
When a function returns an error, check it immediately.
Make sure to wrap errors with context using fmt.Errorf.
Use structured logging for all error paths.
It is important to distinguish between recoverable and fatal errors.

## Testing
Write table-driven tests for all public functions.
Use testify/assert for assertions.
`),
	}
	check := &paddingCheck{}
	findings := check.Run(analysis)
	assert.Len(t, findings, 2, "should find exactly 2 filler phrases")
	assert.Contains(t, findings[0].Issue, "make sure to")
	assert.Contains(t, findings[1].Issue, "it is important to")
}

func TestPadding_EmptyContent(t *testing.T) {
	check := &paddingCheck{}
	analysis := &parser.AgentAnalysis{RawContent: []byte{}}
	assert.Nil(t, check.Run(analysis))
}
