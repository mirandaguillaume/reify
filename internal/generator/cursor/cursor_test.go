package cursor_test

import (
	"testing"

	"github.com/mirandaguillaume/reify/pkg/spec"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newGenerator(t *testing.T) spec.Generator {
	t.Helper()
	gen, err := spec.Get("cursor")
	require.NoError(t, err)
	require.NotNil(t, gen)
	return gen
}

func TestCursorGenerator_Target(t *testing.T) {
	assert.Equal(t, "cursor", newGenerator(t).Target())
}

func TestCursorGenerator_DefaultOutputDir(t *testing.T) {
	assert.Equal(t, ".cursor", newGenerator(t).DefaultOutputDir())
}

func TestCursorGenerator_ContextDir(t *testing.T) {
	assert.Equal(t, "context", newGenerator(t).ContextDir())
}

func TestCursorGenerator_SkillPath(t *testing.T) {
	sg, ok := newGenerator(t).(spec.SkillGenerator)
	require.True(t, ok, "cursor must implement SkillGenerator")
	assert.Equal(t, "rules/code-review.mdc", sg.SkillPath("code-review"))
}

func TestCursorGenerator_AgentPath(t *testing.T) {
	ag, ok := newGenerator(t).(spec.AgentGenerator)
	require.True(t, ok, "cursor must implement AgentGenerator")
	// Cursor expresses agents as rules — the agent path lives under rules/, not agents/.
	assert.Equal(t, "rules/code-reviewer.mdc", ag.AgentPath("code-reviewer"))
}

func TestCursorGenerator_GenerateAgentReturnsEmpty(t *testing.T) {
	// Cursor has no dedicated agent file format — agents are expressed as rules.
	ag, ok := newGenerator(t).(spec.AgentGenerator)
	require.True(t, ok)
	assert.Equal(t, "", ag.GenerateAgent(testAgent(), nil, ".cursor"))
}

func TestCursorGenerator_InstructionsPathEscapesOutputDir(t *testing.T) {
	ig, ok := newGenerator(t).(spec.InstructionsGenerator)
	require.True(t, ok, "cursor must implement InstructionsGenerator")
	// .cursorrules lives at the repo root, one level above .cursor/ — verified intentional.
	assert.Equal(t, "../.cursorrules", ig.InstructionsPath())
}

func TestCursorGenerator_SetOptionsIsNoOp(t *testing.T) {
	c, ok := newGenerator(t).(spec.Configurable)
	require.True(t, ok)
	assert.NotPanics(t, func() {
		c.SetOptions(spec.GeneratorOptions{Compact: true})
	})
}
