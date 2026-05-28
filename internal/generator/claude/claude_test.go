package claude_test

import (
	"testing"

	_ "github.com/mirandaguillaume/reify/internal/generator/claude"
	"github.com/mirandaguillaume/reify/pkg/model"
	"github.com/mirandaguillaume/reify/pkg/spec"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestClaudeGenerator_Registration(t *testing.T) {
	gen, err := spec.Get("claude")
	require.NoError(t, err)
	assert.NotNil(t, gen)
}

func TestClaudeGenerator_Target(t *testing.T) {
	gen, _ := spec.Get("claude")
	assert.Equal(t, "claude", gen.Target())
}

func TestClaudeGenerator_DefaultOutputDir(t *testing.T) {
	gen, _ := spec.Get("claude")
	assert.Equal(t, ".claude", gen.DefaultOutputDir())
}

func TestClaudeGenerator_SkillPath(t *testing.T) {
	gen, _ := spec.Get("claude")
	sg, ok := gen.(spec.SkillGenerator)
	require.True(t, ok)
	assert.Equal(t, "skills/code-review/SKILL.md", sg.SkillPath("code-review"))
}

func TestClaudeGenerator_AgentPath(t *testing.T) {
	gen, _ := spec.Get("claude")
	ag, ok := gen.(spec.AgentGenerator)
	require.True(t, ok)
	assert.Equal(t, "agents/code-reviewer.md", ag.AgentPath("code-reviewer"))
}

func TestClaudeGenerator_ImplementsInstructionsGenerator(t *testing.T) {
	gen, _ := spec.Get("claude")
	ig, ok := gen.(spec.InstructionsGenerator)
	require.True(t, ok, "Claude generator must implement InstructionsGenerator")
	assert.Equal(t, "CLAUDE.md", ig.InstructionsPath())
}

func TestClaudeGenerator_GenerateSkill(t *testing.T) {
	gen, _ := spec.Get("claude")
	sg, ok := gen.(spec.SkillGenerator)
	require.True(t, ok)
	skill := model.SkillBehavior{
		Skill: "test-skill",
		Strategy: model.StrategyFacet{
			Approach: "analytical",
		},
		Context: model.ContextFacet{
			Memory: model.MemoryShortTerm,
		},
		Security: model.SecurityFacet{
			Filesystem: model.AccessReadOnly,
			Network:    model.NetworkNone,
		},
	}
	md := sg.GenerateSkill(skill)
	assert.Contains(t, md, "# Test Skill")
	assert.Contains(t, md, "name: test-skill")
}

func TestClaudeGenerator_GenerateAgent(t *testing.T) {
	gen, _ := spec.Get("claude")
	ag, ok := gen.(spec.AgentGenerator)
	require.True(t, ok)
	agent := model.AgentComposition{
		Agent:         "my-agent",
		Orchestration: model.OrchestrationSequential,
		Skills:        []string{"skill-a"},
	}
	md := ag.GenerateAgent(agent, nil, ".claude")
	assert.Contains(t, md, "name: my-agent")
}

func TestClaudeGenerator_FullGenerator(t *testing.T) {
	gen, _ := spec.Get("claude")
	_, ok := gen.(spec.FullGenerator)
	assert.True(t, ok, "Claude generator should implement FullGenerator")
}
