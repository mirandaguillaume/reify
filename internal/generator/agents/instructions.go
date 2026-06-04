package agents

import (
	"fmt"
	"strings"

	"github.com/mirandaguillaume/reify/internal/generator"
	"github.com/mirandaguillaume/reify/pkg/model"
)

// GenerateAgentsMd renders the root AGENTS.md: an agents overview (if any),
// then one section per skill rendered guardrails-first (primacy bias, the
// same facet ordering the claude/cursor targets use), then a global rules
// digest. Returns "" when there is nothing to emit.
func GenerateAgentsMd(skills []model.SkillBehavior, agents []model.AgentComposition) string {
	if len(skills) == 0 && len(agents) == 0 {
		return ""
	}

	var b []string
	b = append(b, "# AGENTS.md", "")
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

		// Observability — between strategy and security. The agents target
		// has no metrics channel, so it carries the intent as prose.
		if obs := demote(generator.FormatObservability(s.Observability)); obs != "" {
			b = append(b, obs, "")
		}

		// Security LAST (recency bias for least-privilege). The agents target
		// has no config/permission channel, so security degrades to prose
		// here rather than being silently dropped.
		if sec := demote(generator.FormatSecurity(s.Security)); sec != "" {
			b = append(b, sec, "")
		}
	}

	return strings.Join(b, "\n")
}

// demote turns a standalone "## Heading" section (as the shared generator
// helpers emit) into a "### Heading" subsection nested under a skill's "##"
// heading, and trims the trailing newline so it joins cleanly with the
// surrounding lines. Empty input passes through unchanged.
func demote(section string) string {
	if section == "" {
		return ""
	}
	return strings.TrimRight(strings.Replace(section, "## ", "### ", 1), "\n")
}
