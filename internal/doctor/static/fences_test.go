package static

import (
	"testing"

	"github.com/mirandaguillaume/reify/internal/doctor/parser"
	"github.com/stretchr/testify/assert"
)

func TestUnclosedFence_Detected(t *testing.T) {
	content := "## Rules\n" +
		"Always validate input.\n" +
		"```go\n" +
		"func main() {\n" +
		"    fmt.Println(\"hello\")\n" +
		"// missing closing ```\n" +
		"## Hidden section\n" +
		"This content is silently skipped by other checks.\n"

	analysis := &parser.AgentAnalysis{RawContent: []byte(content)}
	check := &unclosedFenceCheck{}
	findings := check.Run(analysis)

	assert.Len(t, findings, 1, "expected 1 unclosed fence finding")
	assert.Contains(t, findings[0].Issue, "Unclosed code fence")
	assert.Contains(t, findings[0].CurrentState, "line 3", "should report opening fence line number")
}

func TestUnclosedFence_Closed(t *testing.T) {
	content := "## Rules\n```go\nfunc main() {}\n```\n## Done\n"
	analysis := &parser.AgentAnalysis{RawContent: []byte(content)}
	check := &unclosedFenceCheck{}
	findings := check.Run(analysis)

	assert.Empty(t, findings, "properly closed fence should produce no findings")
}

func TestUnclosedFence_NoFences(t *testing.T) {
	content := "## Rules\nJust prose, no code blocks at all.\n## Done\n"
	analysis := &parser.AgentAnalysis{RawContent: []byte(content)}
	check := &unclosedFenceCheck{}
	findings := check.Run(analysis)

	assert.Empty(t, findings)
}

func TestUnclosedFence_MultipleClosedFences(t *testing.T) {
	content := "```go\nA\n```\n## Mid\n```python\nB\n```\n## End"
	analysis := &parser.AgentAnalysis{RawContent: []byte(content)}
	check := &unclosedFenceCheck{}
	findings := check.Run(analysis)

	assert.Empty(t, findings, "multiple properly closed fences should not trigger")
}

func TestUnclosedFence_NilAnalysis(t *testing.T) {
	check := &unclosedFenceCheck{}
	assert.Nil(t, check.Run(nil))
}

func TestUnclosedFence_EmptyContent(t *testing.T) {
	check := &unclosedFenceCheck{}
	assert.Nil(t, check.Run(&parser.AgentAnalysis{RawContent: []byte{}}))
}

func TestUnclosedFence_DefaultTagged(t *testing.T) {
	check := &unclosedFenceCheck{}
	assert.Equal(t, []string{"default"}, check.Tags(), "must run in default mode so users see this on every check")
}
