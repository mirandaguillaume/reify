package cursor

import (
	"fmt"
	"strings"

	"github.com/mirandaguillaume/reify/internal/generator"
	"github.com/mirandaguillaume/reify/pkg/model"
)

// GenerateCursorRules generates the root .cursorrules file aggregating
// global guardrails and listing available rule files.
func GenerateCursorRules(skills []model.SkillBehavior, agents []model.AgentComposition) string {
	if len(skills) == 0 && len(agents) == 0 {
		return ""
	}

	var lines []string

	if len(agents) > 0 {
		lines = append(lines, "# Agents")
		lines = append(lines, "")
		for _, agent := range agents {
			desc := agent.Description
			if desc == "" {
				desc = string(agent.Orchestration) + " agent with " + fmt.Sprintf("%d", len(agent.AllSkills())) + " skills"
			}
			lines = append(lines, fmt.Sprintf("## %s", generator.ToTitle(agent.Agent)))
			lines = append(lines, desc)
			lines = append(lines, "")
		}
	}

	if len(skills) > 0 {
		lines = append(lines, "# Available Rules")
		lines = append(lines, "")
		for _, skill := range skills {
			lines = append(lines, fmt.Sprintf("- **%s** (`.cursor/rules/%s.mdc`): %s",
				generator.ToTitle(skill.Skill),
				skill.Skill,
				generator.BuildSkillDescription(skill),
			))
		}
		lines = append(lines, "")
	}

	// Aggregate global guardrails
	var allGuardrails []model.GuardrailRule
	for _, s := range skills {
		allGuardrails = append(allGuardrails, s.Guardrails...)
	}
	if len(allGuardrails) > 0 {
		lines = append(lines, "# Global Rules")
		lines = append(lines, "")
		for _, g := range allGuardrails {
			lines = append(lines, generator.FormatGuardrail(g))
		}
		lines = append(lines, "")
	}

	return strings.Join(lines, "\n")
}
