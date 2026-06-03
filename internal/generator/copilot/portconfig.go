package copilot

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	"github.com/mirandaguillaume/reify/pkg/model"
)

// This file holds the shared building blocks for porting project runtime
// config (hooks/MCP/native skills) to the Copilot family of targets. The
// Copilot "surfaces" (VS Code, CLI, JetBrains) read MCP from different files
// with DIFFERENT schemas, so each surface target supplies its own MCP
// renderer; the hook/skill degradation is identical across them and lives
// here. Schemas are grounded in the current vendor docs (June 2026):
//   - VS Code .vscode/mcp.json : key "servers", per-server "type":"stdio".
//   - Copilot CLI ~/.copilot/mcp-config.json : key "mcpServers", per-server
//     "type":"local" + "tools":["*"].

// renderVSCodeMCP emits the VS Code mcp.json schema: a top-level "servers"
// object, each entry tagged "type":"stdio". NOT the mcpServers schema.
func renderVSCodeMCP(servers map[string]model.MCPServer) string {
	out := map[string]any{}
	for name, s := range servers {
		entry := map[string]any{"type": "stdio", "command": s.Command}
		if len(s.Args) > 0 {
			entry["args"] = s.Args
		}
		if len(s.Env) > 0 {
			entry["env"] = s.Env
		}
		out[name] = entry
	}
	b, _ := json.MarshalIndent(map[string]any{"servers": out}, "", "  ")
	return string(b) + "\n"
}

// renderCLIMCP emits the Copilot CLI mcp-config.json schema: a top-level
// "mcpServers" object, each entry tagged "type":"local" with "tools":["*"].
func renderCLIMCP(servers map[string]model.MCPServer) string {
	out := map[string]any{}
	for name, s := range servers {
		entry := map[string]any{"type": "local", "command": s.Command, "tools": []string{"*"}}
		if len(s.Args) > 0 {
			entry["args"] = s.Args
		}
		if len(s.Env) > 0 {
			entry["env"] = s.Env
		}
		out[name] = entry
	}
	b, _ := json.MarshalIndent(map[string]any{"mcpServers": out}, "", "  ")
	return string(b) + "\n"
}

// renderHooksProse turns enforced hooks into a prose markdown file describing
// them as manual steps. Copilot has no hook system on any surface.
func renderHooksProse(hooks []model.Hook) string {
	var b strings.Builder
	b.WriteString("# Ported automation hooks\n\n")
	b.WriteString("These were enforced hooks in the source harness. Copilot has no hook " +
		"system, so they are NOT enforced here — follow them manually.\n\n")

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

// renderNativeSkillProse turns a native skill into a plain markdown file. The
// body survives; the invocation control does not.
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

// hookWarning / skillWarning are the shared degradation messages.
func hookWarning(n int, where string) string {
	return fmt.Sprintf("%d hook(s) degraded to prose in %s — Copilot has no hook system, "+
		"so they are NOT enforced; the model may ignore them.", n, where)
}

func skillWarning(n int) string {
	return fmt.Sprintf("%d native skill(s) degraded to prose — Copilot has no native-skill "+
		"concept, so model-invocation control is not preserved.", n)
}
