package cursor

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	"github.com/mirandaguillaume/reify/pkg/model"
	"github.com/mirandaguillaume/reify/pkg/spec"
)

// GenerateConfig ports project-level harness config to Cursor with
// per-pillar fidelity, and is honest about what degrades:
//
//   - MCP: full fidelity. Cursor shares the mcpServers schema; only the path
//     differs (.cursor/mcp.json). A mechanical path-map, no warning.
//   - Hooks: lossy. Cursor has no hook system, so an enforced guarantee
//     becomes a prose suggestion in a rule file, and we warn that it is no
//     longer enforced.
//   - Native skills: lossy. Cursor has no native-skill concept, so each one
//     degrades to a rule (.mdc); the model-invocation control is not
//     preserved, and we warn.
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
		files = append(files, spec.ConfigFile{Path: "mcp.json", Content: renderMCP(cfg.MCPServers)})
	}

	// Hooks — degrade to a single prose rule, and warn.
	if len(cfg.Hooks) > 0 {
		files = append(files, spec.ConfigFile{
			Path:    "rules/ported-hooks.mdc",
			Content: renderHooksAsProse(cfg.Hooks),
		})
		warnings = append(warnings, fmt.Sprintf(
			"%d hook(s) degraded to prose suggestions in .cursor/rules/ported-hooks.mdc — "+
				"Cursor has no hook system, so these are NOT enforced; the model may ignore them.",
			len(cfg.Hooks)))
	}

	// Native skills — degrade each to a rule, and warn.
	for _, s := range cfg.NativeSkills {
		files = append(files, spec.ConfigFile{
			Path:    "rules/" + s.Name + ".mdc",
			Content: renderSkillAsRule(s),
		})
	}
	if len(cfg.NativeSkills) > 0 {
		warnings = append(warnings, fmt.Sprintf(
			"%d native skill(s) degraded to Cursor rules — Cursor has no native-skill "+
				"concept, so model-invocation control (disable-model-invocation) is not preserved.",
			len(cfg.NativeSkills)))
	}

	return files, warnings
}

// renderMCP writes the standard cross-tool mcpServers schema verbatim.
func renderMCP(servers map[string]model.MCPServer) string {
	doc := map[string]any{"mcpServers": servers}
	b, _ := json.MarshalIndent(doc, "", "  ")
	return string(b) + "\n"
}

// renderHooksAsProse turns enforced hooks into an always-applied Cursor rule
// describing them as suggestions. The guarantee is gone; the intent survives.
func renderHooksAsProse(hooks []model.Hook) string {
	var b strings.Builder
	b.WriteString("---\n")
	b.WriteString("description: Ported hooks (suggestions — Cursor cannot enforce these)\n")
	b.WriteString("alwaysApply: true\n")
	b.WriteString("---\n\n")
	b.WriteString("# Ported automation hooks\n\n")
	b.WriteString("These were enforced hooks in the source harness. Cursor has no hook " +
		"system, so follow them manually — they are not guaranteed.\n\n")

	// Group by event for stable, readable output.
	byEvent := map[string][]model.Hook{}
	var events []string
	for _, h := range hooks {
		if _, seen := byEvent[h.Event]; !seen {
			events = append(events, h.Event)
		}
		byEvent[h.Event] = append(byEvent[h.Event], h)
	}
	sort.Strings(events)
	for _, e := range events {
		fmt.Fprintf(&b, "## On %s\n\n", e)
		for _, h := range byEvent[e] {
			fmt.Fprintf(&b, "- When `%s` runs: `%s`\n", h.Matcher, h.Command)
		}
		b.WriteString("\n")
	}
	return b.String()
}

// renderSkillAsRule turns a native skill into a Cursor rule (.mdc). The body
// survives as prose; the invocation control does not.
func renderSkillAsRule(s model.NativeSkill) string {
	var b strings.Builder
	b.WriteString("---\n")
	if s.Description != "" {
		fmt.Fprintf(&b, "description: %s\n", s.Description)
	}
	b.WriteString("alwaysApply: false\n")
	b.WriteString("---\n\n")
	fmt.Fprintf(&b, "# %s\n\n", s.Name)
	b.WriteString(strings.TrimSpace(s.Body))
	b.WriteString("\n")
	return b.String()
}
