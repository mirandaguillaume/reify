package generator

import (
	"fmt"
	"sort"
	"strings"

	"github.com/mirandaguillaume/reify/pkg/model"
)

// ToTitle converts "my-skill-name" to "My Skill Name".
func ToTitle(slug string) string {
	words := strings.Split(slug, "-")
	for i, w := range words {
		if len(w) > 0 {
			words[i] = strings.ToUpper(w[:1]) + w[1:]
		}
	}
	return strings.Join(words, " ")
}

// CountWords counts whitespace-separated words in text.
func CountWords(text string) int {
	return len(strings.Fields(text))
}

// FormatGuardrail formats a guardrail rule as markdown list item(s).
func FormatGuardrail(g model.GuardrailRule) string {
	if s, ok := g.StringValue(); ok {
		return "- " + s
	}
	if m, ok := g.MapValue(); ok {
		keys := make([]string, 0, len(m))
		for k := range m {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		var lines []string
		for _, k := range keys {
			lines = append(lines, fmt.Sprintf("- %s: %v", k, m[k]))
		}
		return strings.Join(lines, "\n")
	}
	return ""
}

// FormatWhenToUse formats the when-to-use facet as markdown.
func FormatWhenToUse(w model.WhenToUseFacet) string {
	if w.IsEmpty() {
		return ""
	}
	var sb strings.Builder
	sb.WriteString("## When to Use\n")
	if len(w.Triggers) > 0 {
		sb.WriteString("\nUse for:\n")
		for _, t := range w.Triggers {
			sb.WriteString("- " + t + "\n")
		}
	}
	if len(w.Especially) > 0 {
		sb.WriteString("\n**Especially when:**\n")
		for _, e := range w.Especially {
			sb.WriteString("- " + e + "\n")
		}
	}
	if len(w.DontUse) > 0 {
		sb.WriteString("\n**Don't use for:**\n")
		for _, d := range w.DontUse {
			sb.WriteString("- " + d + "\n")
		}
	}
	return sb.String()
}

// FormatAntiPatterns formats anti-patterns as a markdown table.
func FormatAntiPatterns(aps []model.AntiPattern) string {
	if len(aps) == 0 {
		return ""
	}
	var sb strings.Builder
	sb.WriteString("## Red Flags\n\n")
	sb.WriteString("| Excuse | Reality |\n")
	sb.WriteString("|--------|--------|\n")
	for _, ap := range aps {
		sb.WriteString(fmt.Sprintf("| %s | %s |\n", ap.Excuse, ap.Reality))
	}
	return sb.String()
}

// FormatExamples formats code examples as markdown code blocks.
func FormatExamples(exs []model.CodeExample) string {
	if len(exs) == 0 {
		return ""
	}
	var sb strings.Builder
	sb.WriteString("## Examples\n\n")
	for i, ex := range exs {
		if i > 0 {
			sb.WriteString("\n")
		}
		sb.WriteString("**" + ex.Label + "**\n")
		sb.WriteString("```" + ex.Lang + "\n")
		sb.WriteString(ex.Code + "\n")
		sb.WriteString("```\n")
	}
	return sb.String()
}

// FormatCompactSkill formats a skill as a terse inline block for compact mode.
// Output is ~4 lines: header, I/O, steps, guardrails (omitted if empty).
func FormatCompactSkill(skill model.SkillBehavior) string {
	var sb strings.Builder

	// Line 1: **name** | approach | FS: x | Net: x
	sb.WriteString(fmt.Sprintf("**%s** | %s | FS: %s | Net: %s\n",
		skill.Skill, skill.Strategy.Approach,
		skill.Security.Filesystem, skill.Security.Network))

	// Line 2: In: ... → Out: ... | Mem: ...
	var ioParts []string
	if len(skill.Context.Consumes) > 0 {
		ioParts = append(ioParts, "In: "+strings.Join(skill.Context.Consumes, ", "))
	}
	if len(skill.Context.Produces) > 0 {
		ioParts = append(ioParts, "Out: "+strings.Join(skill.Context.Produces, ", "))
	}
	ioLine := strings.Join(ioParts, " → ")
	if skill.Context.Memory != "" {
		ioLine += " | Mem: " + string(skill.Context.Memory)
	}
	sb.WriteString(ioLine + "\n")

	// Line 3: Steps (collapsed)
	if len(skill.Strategy.Steps) > 0 {
		numbered := make([]string, len(skill.Strategy.Steps))
		for i, s := range skill.Strategy.Steps {
			numbered[i] = fmt.Sprintf("%d. %s", i+1, s)
		}
		sb.WriteString("Steps: " + strings.Join(numbered, "  ") + "\n")
	}

	// Line 4: Guardrails (collapsed, only if non-empty)
	if len(skill.Guardrails) > 0 {
		var parts []string
		for _, g := range skill.Guardrails {
			if s, ok := g.StringValue(); ok {
				parts = append(parts, s)
			}
			if m, ok := g.MapValue(); ok {
				for k, v := range m {
					parts = append(parts, fmt.Sprintf("%s: %v", k, v))
				}
			}
		}
		if len(parts) > 0 {
			sb.WriteString("Guardrails: " + strings.Join(parts, "; ") + "\n")
		}
	}

	return sb.String()
}

// BuildSkillDescription creates a human-readable description from skill facets.
func BuildSkillDescription(skill model.SkillBehavior) string {
	if skill.Strategy.Approach != "" {
		return skill.Strategy.Approach
	}
	// Fallback when approach is empty.
	if len(skill.Context.Produces) > 0 {
		return "Produces " + strings.Join(skill.Context.Produces, ", ")
	}
	return skill.Skill
}
