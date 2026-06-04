package cursor

import (
	"fmt"
	"strings"

	"github.com/mirandaguillaume/reify/internal/generator"
	"github.com/mirandaguillaume/reify/pkg/model"
	"github.com/mirandaguillaume/reify/pkg/spec"
)

// GenerateConfig ports project-level harness config to Cursor with
// per-pillar fidelity, and is honest about what degrades:
//
//   - MCP: full fidelity. Cursor shares the mcpServers schema; only the path
//     differs (.cursor/mcp.json). A mechanical path-map, no warning.
//   - Hooks: lossy. Cursor has no hook system, so an enforced guarantee
//     becomes a prose suggestion in a rule file, and we warn.
//   - Native skills: lossy. Cursor has no native-skill concept, so each one
//     degrades to a rule (.mdc); model-invocation control is not preserved.
//
// Output dir is .cursor, so MCP lands at mcp.json and degraded prose at
// rules/*.mdc within it.
func (g *cursorGenerator) GenerateConfig(cfg model.ProjectConfig) ([]spec.ConfigFile, []string) {
	if cfg.IsEmpty() {
		return nil, nil
	}
	var files []spec.ConfigFile
	var warnings []string

	// MCP — full fidelity, just a different path.
	if len(cfg.MCPServers) > 0 {
		files = append(files, spec.ConfigFile{Path: "mcp.json", Content: generator.RenderMCPServers(cfg.MCPServers)})
	}

	// Hooks — degrade to a single always-applied prose rule, and warn.
	if len(cfg.Hooks) > 0 {
		files = append(files, spec.ConfigFile{
			Path:    "rules/ported-hooks.mdc",
			Content: hooksRule(cfg.Hooks),
		})
		warnings = append(warnings, fmt.Sprintf(
			"%d hook(s) degraded to prose suggestions in .cursor/rules/ported-hooks.mdc — "+
				"Cursor has no hook system, so these are NOT enforced; the model may ignore them.",
			len(cfg.Hooks)))
	}

	// Native skills — degrade each to a rule, and warn.
	for _, s := range cfg.NativeSkills {
		files = append(files, spec.ConfigFile{Path: "rules/" + s.Name + ".mdc", Content: renderSkillAsRule(s)})
	}
	if len(cfg.NativeSkills) > 0 {
		warnings = append(warnings, fmt.Sprintf(
			"%d native skill(s) degraded to Cursor rules — Cursor has no native-skill "+
				"concept, so model-invocation control (disable-model-invocation) is not preserved.",
			len(cfg.NativeSkills)))
	}

	return files, warnings
}

// hooksRule wraps the shared hooks prose body in an always-applied Cursor
// .mdc rule so the suggestions are always in context.
func hooksRule(hooks []model.Hook) string {
	var b strings.Builder
	b.WriteString("---\n")
	b.WriteString("description: Ported hooks (suggestions — Cursor cannot enforce these)\n")
	b.WriteString("alwaysApply: true\n")
	b.WriteString("---\n\n")
	b.WriteString(generator.RenderHooksProse(hooks, "Cursor"))
	return b.String()
}

// renderSkillAsRule turns a native skill into a Cursor rule (.mdc), mapping
// the skill to the right Cursor rule TYPE so it actually fires:
//
//   - With a description → "Agent Requested" (description + alwaysApply:false):
//     Cursor's agent pulls the rule in when the description is relevant, which
//     mirrors a model-invocable skill.
//   - Without a description → "Always" (alwaysApply:true): there is nothing for
//     the agent to match on, so leaving alwaysApply:false would make it a
//     "Manual" rule that only fires on explicit @mention — effectively dead.
//     Always-apply keeps it in context instead of silently never firing.
//
// The body survives as prose; the model-invocation control does not (warned).
func renderSkillAsRule(s model.NativeSkill) string {
	hasDesc := s.Description != ""
	var b strings.Builder
	b.WriteString("---\n")
	if hasDesc {
		fmt.Fprintf(&b, "description: %s\n", s.Description)
	}
	fmt.Fprintf(&b, "alwaysApply: %t\n", !hasDesc)
	b.WriteString("---\n\n")
	fmt.Fprintf(&b, "# %s\n\n", s.Name)
	b.WriteString(strings.TrimSpace(s.Body))
	b.WriteString("\n")
	return b.String()
}
