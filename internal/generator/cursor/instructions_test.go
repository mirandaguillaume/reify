package cursor_test

import (
	"strings"
	"testing"

	"github.com/mirandaguillaume/reify/internal/generator/cursor"
	"github.com/mirandaguillaume/reify/pkg/model"
	"github.com/stretchr/testify/assert"
)

func testAgent() model.AgentComposition {
	return model.AgentComposition{
		Agent:         "code-reviewer",
		Description:   "Reviews code for quality and security issues",
		Skills:        []string{"code-review", "security-scan"},
		Orchestration: model.OrchestrationSequential,
	}
}

func TestGenerateCursorRules_EmptyInputs(t *testing.T) {
	assert.Equal(t, "", cursor.GenerateCursorRules(nil, nil))
	assert.Equal(t, "", cursor.GenerateCursorRules([]model.SkillBehavior{}, []model.AgentComposition{}))
}

func TestGenerateCursorRules_SkillsOnly(t *testing.T) {
	skills := []model.SkillBehavior{testSkill()}
	out := cursor.GenerateCursorRules(skills, nil)

	assert.Contains(t, out, "# Available Rules")
	assert.Contains(t, out, "- **Code Review** (`.cursor/rules/code-review.mdc`):")
	assert.NotContains(t, out, "# Agents")
}

func TestGenerateCursorRules_AgentsBeforeSkills(t *testing.T) {
	skills := []model.SkillBehavior{testSkill()}
	agents := []model.AgentComposition{testAgent()}
	out := cursor.GenerateCursorRules(skills, agents)

	agentsIdx := strings.Index(out, "# Agents")
	skillsIdx := strings.Index(out, "# Available Rules")
	assert.Greater(t, agentsIdx, -1)
	assert.Greater(t, skillsIdx, agentsIdx, "Agents header should appear before Available Rules")
}

func TestGenerateCursorRules_AgentDescriptionFallback(t *testing.T) {
	agent := testAgent()
	agent.Description = ""
	out := cursor.GenerateCursorRules(nil, []model.AgentComposition{agent})

	// Fallback description: "<orchestration> agent with N skills"
	assert.Contains(t, out, "sequential agent with 2 skills")
}

func TestGenerateCursorRules_AgentTitle(t *testing.T) {
	out := cursor.GenerateCursorRules(nil, []model.AgentComposition{testAgent()})
	assert.Contains(t, out, "## Code Reviewer")
}

func TestGenerateCursorRules_GlobalGuardrailsAggregated(t *testing.T) {
	a := testSkill() // 2 guardrails
	b := testSkill()
	b.Skill = "other-skill"
	b.Guardrails = []model.GuardrailRule{
		makeGuardrailString("Never commit secrets"),
	}
	out := cursor.GenerateCursorRules([]model.SkillBehavior{a, b}, nil)

	assert.Contains(t, out, "# Global Rules")
	assert.Contains(t, out, "Never modify source files directly")
	assert.Contains(t, out, "Always explain reasoning")
	assert.Contains(t, out, "Never commit secrets")
}

func TestGenerateCursorRules_NoGuardrailsSectionWhenEmpty(t *testing.T) {
	skill := testSkill()
	skill.Guardrails = nil
	out := cursor.GenerateCursorRules([]model.SkillBehavior{skill}, nil)
	assert.NotContains(t, out, "# Global Rules")
}

func TestGenerateCursorRules_SkillListEntryFormat(t *testing.T) {
	skills := []model.SkillBehavior{testSkill()}
	out := cursor.GenerateCursorRules(skills, nil)
	// Format: "- **<Title>** (`.cursor/rules/<slug>.mdc`): <description>"
	assert.Regexp(t, `- \*\*Code Review\*\* \(\x60\.cursor/rules/code-review\.mdc\x60\): .+`, out)
}
