package copilot_test

import (
	"testing"

	_ "github.com/mirandaguillaume/reify/internal/generator/copilot"
	"github.com/mirandaguillaume/reify/pkg/model"
	"github.com/mirandaguillaume/reify/pkg/spec"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCopilotGenerator_Registration(t *testing.T) {
	gen, err := spec.Get("copilot")
	require.NoError(t, err)
	assert.NotNil(t, gen)
}

func TestCopilotGenerator_Target(t *testing.T) {
	gen, _ := spec.Get("copilot")
	assert.Equal(t, "copilot", gen.Target())
}

func TestCopilotGenerator_DefaultOutputDir(t *testing.T) {
	gen, _ := spec.Get("copilot")
	assert.Equal(t, ".github", gen.DefaultOutputDir())
}

func TestCopilotGenerator_SkillPath(t *testing.T) {
	gen, _ := spec.Get("copilot")
	sg, ok := gen.(spec.SkillGenerator)
	require.True(t, ok)
	assert.Equal(t, "skills/code-review/SKILL.md", sg.SkillPath("code-review"))
}

func TestCopilotGenerator_AgentPath(t *testing.T) {
	gen, _ := spec.Get("copilot")
	ag, ok := gen.(spec.AgentGenerator)
	require.True(t, ok)
	assert.Equal(t, "agents/code-reviewer.agent.md", ag.AgentPath("code-reviewer"))
}

func TestCopilotGenerator_InstructionsPath(t *testing.T) {
	gen, _ := spec.Get("copilot")
	ig, ok := gen.(spec.InstructionsGenerator)
	require.True(t, ok, "Copilot generator should implement InstructionsGenerator")
	assert.Equal(t, "copilot-instructions.md", ig.InstructionsPath())
}

func TestCopilotGenerator_GenerateInstructions_NotEmpty(t *testing.T) {
	gen, _ := spec.Get("copilot")
	ig, ok := gen.(spec.InstructionsGenerator)
	require.True(t, ok)
	skills := []model.SkillBehavior{
		{
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
		},
	}
	result := ig.GenerateInstructions(skills, nil)
	assert.NotEmpty(t, result)
	assert.Contains(t, result, "# Project Instructions")
}

func TestCopilotGenerator_GenerateInstructions_EmptyForNoInput(t *testing.T) {
	gen, _ := spec.Get("copilot")
	ig, ok := gen.(spec.InstructionsGenerator)
	require.True(t, ok)
	result := ig.GenerateInstructions(nil, nil)
	assert.Empty(t, result)
}

func TestCopilotGenerator_GenerateSkill(t *testing.T) {
	gen, _ := spec.Get("copilot")
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

func TestCopilotGenerator_GenerateAgent(t *testing.T) {
	gen, _ := spec.Get("copilot")
	ag, ok := gen.(spec.AgentGenerator)
	require.True(t, ok)
	agent := model.AgentComposition{
		Agent:         "my-agent",
		Orchestration: model.OrchestrationSequential,
		Skills:        []string{"skill-a"},
	}
	md := ag.GenerateAgent(agent, nil, ".github")
	assert.Contains(t, md, "name: my-agent")
}

func TestCopilotGenerator_FullGenerator(t *testing.T) {
	gen, _ := spec.Get("copilot")
	_, ok := gen.(spec.FullGenerator)
	assert.True(t, ok, "Copilot generator should implement FullGenerator")
}
