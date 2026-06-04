package aider

import (
	"fmt"
	"strings"

	"github.com/mirandaguillaume/reify/internal/generator"
	"github.com/mirandaguillaume/reify/pkg/model"
)

// GenerateConventionsMd renders aider's CONVENTIONS.md: an agents overview (if
// any), then one section per skill rendered guardrails-first (primacy), then
// observability and security last (recency) — the same proven-efficacy order
// the agents/cursor targets use. Returns "" when there is nothing to emit.
func GenerateConventionsMd(skills []model.SkillBehavior, agents []model.AgentComposition) string {
	if len(skills) == 0 && len(agents) == 0 {
		return ""
	}

	var b []string
	b = append(b, "# Coding Conventions", "")
	b = append(b, "> Compiled by reify. Edit the Reify source, not this file.", "")

	if len(agents) > 0 {
		b = append(b, "## Agents", "")
		for _, a := range agents {
			desc := a.Description
			if desc == "" {
				desc = fmt.Sprintf("%s agent with %d skills", a.Orchestration, len(a.AllSkills()))
			}
			b = append(b, fmt.Sprintf("- **%s**: %s", a.Agent, desc))
		}
		b = append(b, "")
	}

	for _, s := range skills {
		b = append(b, "## "+generator.ToTitle(s.Skill), "")

		// Guardrails first (primacy bias).
		if len(s.Guardrails) > 0 {
			b = append(b, "### Rules")
			for _, g := range s.Guardrails {
				b = append(b, generator.FormatGuardrail(g))
			}
			b = append(b, "")
		}

		// Strategy steps.
		if len(s.Strategy.Steps) > 0 {
			b = append(b, "### Steps")
			for i, step := range s.Strategy.Steps {
				b = append(b, fmt.Sprintf("%d. %s", i+1, step))
			}
			b = append(b, "")
		}

		// Tools.
		if len(s.Strategy.Tools) > 0 {
			b = append(b, "### Tools")
			b = append(b, strings.Join(s.Strategy.Tools, ", "))
			b = append(b, "")
		}

		// Observability, then security LAST (recency for least-privilege).
		if obs := demote(generator.FormatObservability(s.Observability)); obs != "" {
			b = append(b, obs, "")
		}
		if sec := demote(generator.FormatSecurity(s.Security)); sec != "" {
			b = append(b, sec, "")
		}
	}

	return strings.Join(b, "\n")
}

// demote turns a standalone "## Heading" section (as the shared generator
// helpers emit) into a "### Heading" subsection nested under a skill's "##"
// heading, trimming the trailing newline so it joins cleanly.
func demote(section string) string {
	if section == "" {
		return ""
	}
	return strings.TrimRight(strings.Replace(section, "## ", "### ", 1), "\n")
}
