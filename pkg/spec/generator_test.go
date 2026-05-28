package spec_test

import (
	"testing"

	"github.com/mirandaguillaume/reify/pkg/model"
	"github.com/mirandaguillaume/reify/pkg/spec"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// mockGenerator implements Generator + SkillGenerator + AgentGenerator (no InstructionsGenerator).
type mockGenerator struct{}

func (m *mockGenerator) Target() string            { return "mock" }
func (m *mockGenerator) DefaultOutputDir() string  { return ".mock" }
func (m *mockGenerator) ContextDir() string        { return "context" }
func (m *mockGenerator) GenerateSkill(_ model.SkillBehavior) string { return "skill-md" }
func (m *mockGenerator) GenerateAgent(_ model.AgentComposition, _ []model.SkillBehavior, _ string) string {
	return "agent-md"
}
func (m *mockGenerator) SkillPath(name string) string { return "skills/" + name + "/SKILL.md" }
func (m *mockGenerator) AgentPath(name string) string { return "agents/" + name + ".md" }

// mockFullGenerator implements all 4 interfaces including InstructionsGenerator.
type mockFullGenerator struct {
	mockGenerator
}

func (m *mockFullGenerator) GenerateInstructions(_ []model.SkillBehavior, _ []model.AgentComposition) string {
	return "instructions-md"
}
func (m *mockFullGenerator) InstructionsPath() string { return "instructions.md" }

func TestRegisterAndGet(t *testing.T) {
	spec.Reset()
	spec.Register("mock", func() spec.Generator { return &mockGenerator{} })

	gen, err := spec.Get("mock")
	require.NoError(t, err)
	assert.Equal(t, "mock", gen.Target())
	assert.Equal(t, ".mock", gen.DefaultOutputDir())

	sg, ok := gen.(spec.SkillGenerator)
	require.True(t, ok)
	assert.Equal(t, "skill-md", sg.GenerateSkill(model.SkillBehavior{}))
}

func TestGet_Unknown(t *testing.T) {
	spec.Reset()
	_, err := spec.Get("unknown")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "unknown build target")
}

func TestAvailable(t *testing.T) {
	spec.Reset()
	spec.Register("beta", func() spec.Generator { return &mockGenerator{} })
	spec.Register("alpha", func() spec.Generator { return &mockGenerator{} })

	targets := spec.Available()
	assert.Equal(t, []string{"alpha", "beta"}, targets) // sorted
}

func TestAvailable_Empty(t *testing.T) {
	spec.Reset()
	targets := spec.Available()
	assert.Empty(t, targets)
}

func TestReset(t *testing.T) {
	spec.Reset()
	spec.Register("test", func() spec.Generator { return &mockGenerator{} })
	assert.Len(t, spec.Available(), 1)
	spec.Reset()
	assert.Empty(t, spec.Available())
}

func TestGeneratorMethods(t *testing.T) {
	spec.Reset()
	spec.Register("mock", func() spec.Generator { return &mockGenerator{} })
	gen, _ := spec.Get("mock")

	sg, ok := gen.(spec.SkillGenerator)
	require.True(t, ok)
	assert.Equal(t, "skills/test/SKILL.md", sg.SkillPath("test"))

	ag, ok := gen.(spec.AgentGenerator)
	require.True(t, ok)
	assert.Equal(t, "agents/test.md", ag.AgentPath("test"))

	_, ok = gen.(spec.InstructionsGenerator)
	assert.False(t, ok, "mockGenerator should not implement InstructionsGenerator")
}

func TestInstructionsGenerator(t *testing.T) {
	spec.Reset()
	spec.Register("full", func() spec.Generator { return &mockFullGenerator{} })
	gen, _ := spec.Get("full")

	ig, ok := gen.(spec.InstructionsGenerator)
	require.True(t, ok, "mockFullGenerator should implement InstructionsGenerator")
	assert.Equal(t, "instructions-md", ig.GenerateInstructions(nil, nil))
	assert.Equal(t, "instructions.md", ig.InstructionsPath())
}

func TestFullGeneratorInterface(t *testing.T) {
	spec.Reset()
	spec.Register("mock", func() spec.Generator { return &mockGenerator{} })
	gen, _ := spec.Get("mock")

	_, ok := gen.(spec.FullGenerator)
	assert.True(t, ok, "mockGenerator should implement FullGenerator")
}
