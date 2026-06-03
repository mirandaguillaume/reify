package copilot

import (
	"encoding/json"
	"fmt"

	"github.com/mirandaguillaume/reify/internal/generator"
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

// renderHooksProse / renderNativeSkillProse delegate to the shared generator
// helpers, naming Copilot as the system that cannot enforce them.
func renderHooksProse(hooks []model.Hook) string {
	return generator.RenderHooksProse(hooks, "Copilot")
}

func renderNativeSkillProse(s model.NativeSkill) string {
	return generator.RenderNativeSkillBody(s)
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
