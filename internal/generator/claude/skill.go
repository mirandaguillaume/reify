package claude

import (
	"fmt"
	"strings"

	"github.com/mirandaguillaume/reify/internal/generator"
	"github.com/mirandaguillaume/reify/pkg/model"
)

// GenerateSkillMd generates a Claude Code SKILL.md from a SkillBehavior.
// contracts is an optional map of output format templates keyed by produces name.
// contractsDir, when non-empty, generates file references instead of inlining templates.
func GenerateSkillMd(skill model.SkillBehavior, contracts map[string]string, contractsDir string) string {
	var lines []string

	// Frontmatter
	desc := generator.BuildSkillDescription(skill)
	lines = append(lines, "---")
	lines = append(lines, "name: "+skill.Skill)
	lines = append(lines, "description: "+desc)
	lines = append(lines, "---")
	lines = append(lines, "")

	// Title
	lines = append(lines, "# "+generator.ToTitle(skill.Skill))
	lines = append(lines, "")

	// Input — contract references for consumed data
	if inputSection := generator.FormatContractSectionWithDir(skill.Context.Consumes, contracts, contractsDir); inputSection != "" {
		lines = append(lines, strings.Replace(inputSection, "## Output", "## Input", 1))
	}

	// Guardrails FIRST (primacy bias)
	if len(skill.Guardrails) > 0 {
		lines = append(lines, "## Guardrails")
		for _, g := range skill.Guardrails {
			lines = append(lines, generator.FormatGuardrail(g))
		}
		lines = append(lines, "")
	}

	// Steps (core behavior)
	if len(skill.Strategy.Steps) > 0 {
		lines = append(lines, "## Steps")
		for i, step := range skill.Strategy.Steps {
			lines = append(lines, fmt.Sprintf("%d. %s", i+1, step))
		}
		lines = append(lines, "")
	}

	// Output — contract references for produced data
	if contractSection := generator.FormatContractSectionWithDir(skill.Context.Produces, contracts, contractsDir); contractSection != "" {
		lines = append(lines, contractSection)
	}

	// Examples (concrete, actionable)
	if exs := generator.FormatExamples(skill.Examples); exs != "" {
		lines = append(lines, exs)
	}

	// Anti-patterns / Red Flags
	if aps := generator.FormatAntiPatterns(skill.AntiPatterns); aps != "" {
		lines = append(lines, aps)
	}

	lines = append(lines, "")

	return strings.Join(lines, "\n")
}
