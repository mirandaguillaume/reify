package cursor

import (
	"fmt"
	"strings"

	"github.com/mirandaguillaume/reify/internal/generator"
	"github.com/mirandaguillaume/reify/pkg/model"
)

// GenerateCursorSkillMdc generates a Cursor .mdc rule file from a SkillBehavior.
func GenerateCursorSkillMdc(skill model.SkillBehavior) string {
	var lines []string

	// Frontmatter
	desc := generator.BuildSkillDescription(skill)
	globs := inferGlobs(skill)
	lines = append(lines, "---")
	lines = append(lines, "description: "+desc)
	if globs != "" {
		lines = append(lines, "globs: "+globs)
	}
	lines = append(lines, "alwaysApply: false")
	lines = append(lines, "---")
	lines = append(lines, "")

	lines = append(lines, "# "+generator.ToTitle(skill.Skill))
	lines = append(lines, "")

	// Guardrails first (primacy bias — same pattern as Claude target)
	if len(skill.Guardrails) > 0 {
		lines = append(lines, "## Rules")
		for _, g := range skill.Guardrails {
			lines = append(lines, generator.FormatGuardrail(g))
		}
		lines = append(lines, "")
	}

	// Strategy steps
	if len(skill.Strategy.Steps) > 0 {
		lines = append(lines, "## Steps")
		for i, step := range skill.Strategy.Steps {
			lines = append(lines, fmt.Sprintf("%d. %s", i+1, step))
		}
		lines = append(lines, "")
	}

	// Tools
	if len(skill.Strategy.Tools) > 0 {
		lines = append(lines, "## Tools")
		lines = append(lines, strings.Join(skill.Strategy.Tools, ", "))
		lines = append(lines, "")
	}

	return strings.Join(lines, "\n")
}

// inferGlobs produces a Cursor glob pattern from the skill's tool list and name.
// Best-effort — no globs if nothing matches.
func inferGlobs(skill model.SkillBehavior) string {
	name := strings.ToLower(skill.Skill)
	tools := strings.ToLower(strings.Join(skill.Strategy.Tools, " "))

	switch {
	case strings.Contains(name, "typescript") || strings.Contains(tools, "typescript"):
		return "**/*.ts, **/*.tsx"
	case strings.Contains(name, "python") || strings.Contains(tools, "python"):
		return "**/*.py"
	case strings.Contains(name, "go") || strings.Contains(tools, "golang"):
		return "**/*.go"
	case strings.Contains(name, "test"):
		return "**/*_test.*, **/*.test.*, **/*.spec.*"
	case strings.Contains(name, "css") || strings.Contains(name, "style"):
		return "**/*.css, **/*.scss, **/*.sass"
	default:
		return ""
	}
}
