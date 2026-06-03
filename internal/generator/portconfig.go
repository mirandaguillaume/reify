package generator

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	"github.com/mirandaguillaume/reify/pkg/model"
)

// This file holds rendering helpers shared by every target's ConfigGenerator
// for porting project runtime config (MCP / hooks / native skills). Keeping
// them here avoids each target package re-implementing identical markdown and
// JSON shaping. Surface-specific schemas (e.g. VS Code's "servers" key) stay
// in their own package; only the genuinely common shapes live here.

// RenderMCPServers writes the standard cross-tool mcpServers schema
// (`{"mcpServers": {...}}`) used by Claude Code, Cursor, and the generic
// AGENTS.md reference snippet.
func RenderMCPServers(servers map[string]model.MCPServer) string {
	b, _ := json.MarshalIndent(map[string]any{"mcpServers": servers}, "", "  ")
	return string(b) + "\n"
}

// RenderHooksProse renders hooks as a markdown body describing them as manual
// steps, for targets with no hook system. systemName names the target in the
// "X has no hook system" sentence (e.g. "Cursor", "Copilot", "The AGENTS.md
// ecosystem"). The body carries no frontmatter; callers that need it (Cursor
// .mdc) prepend their own.
func RenderHooksProse(hooks []model.Hook, systemName string) string {
	var b strings.Builder
	b.WriteString("# Ported automation hooks\n\n")
	fmt.Fprintf(&b, "These were enforced hooks in the source harness. %s has no hook "+
		"system, so follow them manually — they are not enforced.\n\n", systemName)

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

// RenderNativeSkillBody renders a native skill as a plain markdown body
// (title, description, body). Targets that keep skill metadata in frontmatter
// (Cursor .mdc) render their own; this is the generic prose form.
func RenderNativeSkillBody(s model.NativeSkill) string {
	var b strings.Builder
	fmt.Fprintf(&b, "# %s\n\n", s.Name)
	if s.Description != "" {
		b.WriteString(s.Description + "\n\n")
	}
	b.WriteString(strings.TrimSpace(s.Body))
	b.WriteString("\n")
	return b.String()
}
