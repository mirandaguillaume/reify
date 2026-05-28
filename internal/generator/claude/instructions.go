package claude

import (
	"fmt"
	"strings"

	"github.com/mirandaguillaume/reify/internal/generator"
	"github.com/mirandaguillaume/reify/pkg/model"
)

// GenerateClaudeMd generates a CLAUDE.md from skills and agents.
// Returns empty string if there are no skills or agents.
func GenerateClaudeMd(skills []model.SkillBehavior, agents []model.AgentComposition) string {
	if len(skills) == 0 && len(agents) == 0 {
		return ""
	}

	var lines []string

	if len(agents) > 0 {
		lines = append(lines, "## Available Agents")
		lines = append(lines, "")
		for _, agent := range agents {
			desc := agent.Description
			if desc == "" {
				desc = string(agent.Orchestration) + " agent coordinating " + fmt.Sprintf("%d", len(agent.AllSkills())) + " skills"
			}
			lines = append(lines, fmt.Sprintf("- **%s**: %s", generator.ToTitle(agent.Agent), desc))
			lines = append(lines, fmt.Sprintf("  Use: `.claude/agents/%s.md`", agent.Agent))
		}
		lines = append(lines, "")
	}

	if len(skills) > 0 {
		lines = append(lines, "## Available Skills")
		lines = append(lines, "")
		for _, skill := range skills {
			desc := generator.BuildSkillDescription(skill)
			lines = append(lines, fmt.Sprintf("- **%s**: %s", generator.ToTitle(skill.Skill), desc))
		}
		lines = append(lines, "")
	}

	// Aggregate global guardrails across all skills.
	var allGuardrails []model.GuardrailRule
	for _, s := range skills {
		allGuardrails = append(allGuardrails, s.Guardrails...)
	}
	if len(allGuardrails) > 0 {
		lines = append(lines, "## Global Guardrails")
		lines = append(lines, "")
		for _, g := range allGuardrails {
			lines = append(lines, generator.FormatGuardrail(g))
		}
		lines = append(lines, "")
	}

	return strings.Join(lines, "\n")
}
