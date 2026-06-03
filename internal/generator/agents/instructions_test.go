package agents_test

import (
	"strings"
	"testing"

	"github.com/mirandaguillaume/reify/internal/generator/agents"
	"github.com/mirandaguillaume/reify/pkg/model"
	"github.com/stretchr/testify/assert"
	"gopkg.in/yaml.v3"
)

// GuardrailRule is an opaque type unmarshalled from YAML; construct a
// string-valued one the same way the loader does.
func makeGuardrailString(s string) model.GuardrailRule {
	var g model.GuardrailRule
	_ = g.UnmarshalYAML(&yaml.Node{Kind: yaml.ScalarNode, Value: s})
	return g
}

func testSkill() model.SkillBehavior {
	return model.SkillBehavior{
		Skill:   "code-review",
		Version: "1.0",
		Strategy: model.StrategyFacet{
			Tools: []string{"read", "grep"},
			Steps: []string{"Read the code", "Analyze patterns", "Write report"},
		},
		Guardrails: []model.GuardrailRule{
			makeGuardrailString("Never modify source files directly"),
			makeGuardrailString("Always explain reasoning"),
		},
	}
}

func testAgent() model.AgentComposition {
	return model.AgentComposition{
		Agent:         "code-reviewer",
		Description:   "Reviews code for quality and security issues",
		Skills:        []string{"code-review", "security-scan"},
		Orchestration: model.OrchestrationSequential,
	}
}

func TestGenerateAgentsMd_Empty(t *testing.T) {
	assert.Equal(t, "", agents.GenerateAgentsMd(nil, nil))
	assert.Equal(t, "", agents.GenerateAgentsMd([]model.SkillBehavior{}, []model.AgentComposition{}))
}

func TestGenerateAgentsMd_HasTitle(t *testing.T) {
	out := agents.GenerateAgentsMd([]model.SkillBehavior{testSkill()}, nil)
	assert.True(t, strings.HasPrefix(out, "# AGENTS.md"), "file starts with the AGENTS.md H1")
}

func TestGenerateAgentsMd_SkillSection(t *testing.T) {
	out := agents.GenerateAgentsMd([]model.SkillBehavior{testSkill()}, nil)
	assert.Contains(t, out, "## Code Review")           // skill rendered as a section, title-cased
	assert.Contains(t, out, "Read the code")            // strategy steps present
	assert.Contains(t, out, "Never modify source files") // guardrail present
}

func TestGenerateAgentsMd_GuardrailsBeforeSteps(t *testing.T) {
	// Facet ordering: guardrails (primacy) come before strategy steps.
	out := agents.GenerateAgentsMd([]model.SkillBehavior{testSkill()}, nil)
	gIdx := strings.Index(out, "Never modify source files")
	sIdx := strings.Index(out, "Read the code")
	assert.Greater(t, gIdx, -1)
	assert.Greater(t, sIdx, gIdx, "guardrails must appear before strategy steps")
}

func TestGenerateAgentsMd_AgentsBeforeSkills(t *testing.T) {
	out := agents.GenerateAgentsMd([]model.SkillBehavior{testSkill()}, []model.AgentComposition{testAgent()})
	aIdx := strings.Index(out, "code-reviewer")
	sIdx := strings.Index(out, "## Code Review")
	assert.Greater(t, aIdx, -1)
	assert.Greater(t, sIdx, aIdx, "agents section before skills section")
}

func TestGenerateAgentsMd_AgentDescriptionFallback(t *testing.T) {
	a := testAgent()
	a.Description = ""
	out := agents.GenerateAgentsMd(nil, []model.AgentComposition{a})
	assert.Contains(t, out, "sequential agent with 2 skills")
}
