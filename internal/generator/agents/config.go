package agents

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	"github.com/mirandaguillaume/reify/pkg/model"
	"github.com/mirandaguillaume/reify/pkg/spec"
)

// GenerateConfig ports project runtime config to the generic AGENTS.md
// ecosystem. AGENTS.md is purely an instructions convention shared by many
// tools (Codex, Zed, recent Aider); it defines no standard location for MCP,
// hooks, or native skills. So every pillar degrades here: MCP is emitted as a
// reference snippet (each consuming tool configures it differently), and
// hooks/native skills become prose. All three carry a warning.
func (g *agentsGenerator) GenerateConfig(cfg model.ProjectConfig) ([]spec.ConfigFile, []string) {
	if cfg.IsEmpty() {
		return nil, nil
	}
	var files []spec.ConfigFile
	var warnings []string

	if len(cfg.MCPServers) > 0 {
		files = append(files, spec.ConfigFile{Path: "mcp.json", Content: renderMCPServers(cfg.MCPServers)})
		warnings = append(warnings,
			"AGENTS.md defines no standard MCP location — tools that read it (Codex, Zed, ...) "+
				"each configure MCP separately. A reference mcp.json was written; wire it per tool.")
	}

	if len(cfg.Hooks) > 0 {
		files = append(files, spec.ConfigFile{Path: "ported-hooks.md", Content: renderHooksProse(cfg.Hooks)})
		warnings = append(warnings, fmt.Sprintf(
			"%d hook(s) degraded to prose in ported-hooks.md — the AGENTS.md ecosystem has no "+
				"hook system, so they are NOT enforced; the model may ignore them.", len(cfg.Hooks)))
	}

	for _, s := range cfg.NativeSkills {
		files = append(files, spec.ConfigFile{Path: "ported-skills/" + s.Name + ".md", Content: renderNativeSkillProse(s)})
	}
	if len(cfg.NativeSkills) > 0 {
		warnings = append(warnings, fmt.Sprintf(
			"%d native skill(s) degraded to prose — the AGENTS.md ecosystem has no native-skill "+
				"concept, so model-invocation control is not preserved.", len(cfg.NativeSkills)))
	}

	return files, warnings
}

// renderMCPServers writes the standard cross-tool mcpServers schema.
func renderMCPServers(servers map[string]model.MCPServer) string {
	b, _ := json.MarshalIndent(map[string]any{"mcpServers": servers}, "", "  ")
	return string(b) + "\n"
}

func renderHooksProse(hooks []model.Hook) string {
	var b strings.Builder
	b.WriteString("# Ported automation hooks\n\n")
	b.WriteString("These were enforced hooks in the source harness. The AGENTS.md ecosystem " +
		"has no hook system, so follow them manually — they are not guaranteed.\n\n")

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

func renderNativeSkillProse(s model.NativeSkill) string {
	var b strings.Builder
	fmt.Fprintf(&b, "# %s\n\n", s.Name)
	if s.Description != "" {
		b.WriteString(s.Description + "\n\n")
	}
	b.WriteString(strings.TrimSpace(s.Body))
	b.WriteString("\n")
	return b.String()
}
