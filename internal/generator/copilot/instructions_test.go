package copilot_test

import (
	"testing"

	"github.com/mirandaguillaume/reify/internal/generator/copilot"
	"github.com/mirandaguillaume/reify/pkg/model"
	"github.com/stretchr/testify/assert"
)

func TestGenerateCopilotInstructions_EmptyForNil(t *testing.T) {
	result := copilot.GenerateCopilotInstructions(nil, nil)
	assert.Empty(t, result)
}

func TestGenerateCopilotInstructions_EmptyForEmptySlices(t *testing.T) {
	result := copilot.GenerateCopilotInstructions([]model.SkillBehavior{}, []model.AgentComposition{})
	assert.Empty(t, result)
}

func TestGenerateCopilotInstructions_WithSkills(t *testing.T) {
	skills := []model.SkillBehavior{
		{
			Skill: "code-review",
			Strategy: model.StrategyFacet{
				Approach: "analytical",
			},
			Context: model.ContextFacet{
				Consumes: []string{"source-code"},
				Produces: []string{"review-report"},
				Memory:   model.MemoryConversation,
			},
			Security: model.SecurityFacet{
				Filesystem: model.AccessReadOnly,
				Network:    model.NetworkNone,
			},
		},
	}
	result := copilot.GenerateCopilotInstructions(skills, nil)
	assert.NotEmpty(t, result)
	assert.Contains(t, result, "# Project Instructions")
	assert.Contains(t, result, "## Available Skills")
	assert.Contains(t, result, "**Code Review**")
	assert.Contains(t, result, "analytical")
}

func TestGenerateCopilotInstructions_WithAgents(t *testing.T) {
	agents := []model.AgentComposition{
		{
			Agent:         "code-reviewer",
			Description:   "Reviews code for quality",
			Skills:        []string{"code-review"},
			Orchestration: model.OrchestrationSequential,
		},
	}
	result := copilot.GenerateCopilotInstructions(nil, agents)
	assert.NotEmpty(t, result)
	assert.Contains(t, result, "## Available Agents")
	assert.Contains(t, result, "**Code Reviewer**")
	assert.Contains(t, result, "Reviews code for quality")
}

func TestGenerateCopilotInstructions_AgentWithoutDescription(t *testing.T) {
	agents := []model.AgentComposition{
		{
			Agent:         "my-agent",
			Skills:        []string{"a", "b", "c"},
			Orchestration: model.OrchestrationParallel,
		},
	}
	result := copilot.GenerateCopilotInstructions(nil, agents)
	assert.NotEmpty(t, result)
	assert.Contains(t, result, "**My Agent**: parallel agent with 3 skills")
}

func TestGenerateCopilotInstructions_WithGuardrails(t *testing.T) {
	skills := []model.SkillBehavior{
		{
			Skill: "safe-skill",
			Strategy: model.StrategyFacet{
				Approach: "careful",
			},
			Context: model.ContextFacet{
				Memory: model.MemoryShortTerm,
			},
			Guardrails: []model.GuardrailRule{
				makeGuardrailString("Never delete files"),
				makeGuardrailString("Always backup first"),
			},
			Security: model.SecurityFacet{
				Filesystem: model.AccessReadOnly,
				Network:    model.NetworkNone,
			},
		},
	}
	result := copilot.GenerateCopilotInstructions(skills, nil)
	assert.NotEmpty(t, result)
	assert.Contains(t, result, "## Global Guardrails")
	assert.Contains(t, result, "- Never delete files")
	assert.Contains(t, result, "- Always backup first")
}

func TestGenerateCopilotInstructions_NoGuardrailsSection(t *testing.T) {
	skills := []model.SkillBehavior{
		{
			Skill: "no-guardrails",
			Strategy: model.StrategyFacet{
				Approach: "fast",
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
	result := copilot.GenerateCopilotInstructions(skills, nil)
	assert.NotEmpty(t, result)
	assert.NotContains(t, result, "## Global Guardrails")
}

func TestGenerateCopilotInstructions_BothSkillsAndAgents(t *testing.T) {
	skills := []model.SkillBehavior{
		{
			Skill: "skill-a",
			Strategy: model.StrategyFacet{
				Approach: "methodical",
			},
			Context: model.ContextFacet{
				Produces: []string{"output-a"},
				Memory:   model.MemoryConversation,
			},
			Security: model.SecurityFacet{
				Filesystem: model.AccessReadOnly,
				Network:    model.NetworkNone,
			},
		},
	}
	agents := []model.AgentComposition{
		{
			Agent:         "agent-a",
			Description:   "Does things",
			Skills:        []string{"skill-a"},
			Orchestration: model.OrchestrationSequential,
		},
	}
	result := copilot.GenerateCopilotInstructions(skills, agents)
	assert.NotEmpty(t, result)
	assert.Contains(t, result, "## Available Skills")
	assert.Contains(t, result, "## Available Agents")
}
