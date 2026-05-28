package reify

import (
	"fmt"
	"strings"

	"github.com/mirandaguillaume/reify/internal/generator"
	"github.com/mirandaguillaume/reify/pkg/model"
)

// BuildPromptTemplate constructs a prompt template string from a skill's facets.
// Template variables like {{ .type_name }} are replaced at runtime with actual input values.
func BuildPromptTemplate(skill model.SkillBehavior) string {
	var b strings.Builder

	b.WriteString("You are: " + skill.Skill + "\n\n")

	if skill.Strategy.Approach != "" {
		b.WriteString("## Approach\n" + skill.Strategy.Approach + "\n\n")
	}

	if len(skill.Strategy.Steps) > 0 {
		b.WriteString("## Steps\n")
		for i, step := range skill.Strategy.Steps {
			fmt.Fprintf(&b, "%d. %s\n", i+1, step)
		}
		b.WriteString("\n")
	}

	if len(skill.Guardrails) > 0 {
		b.WriteString("## Guardrails\n")
		for _, g := range skill.Guardrails {
			line := generator.FormatGuardrail(g)
			if line != "" {
				b.WriteString(line + "\n")
			}
		}
		b.WriteString("\n")
	}

	if len(skill.Context.Consumes) > 0 {
		b.WriteString("## Input\n")
		for _, c := range skill.Context.Consumes {
			fmt.Fprintf(&b, "%s:\n{{ .%s }}\n\n", c, c)
		}
	}

	if len(skill.Context.Produces) > 0 {
		b.WriteString("## Output\n")
		b.WriteString("Produce: " + strings.Join(skill.Context.Produces, ", ") + "\n")
	}

	return b.String()
}
