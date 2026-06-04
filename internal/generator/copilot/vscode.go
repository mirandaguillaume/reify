package copilot

import (
	"github.com/mirandaguillaume/reify/pkg/model"
	"github.com/mirandaguillaume/reify/pkg/spec"
)

// copilotVSCodeGenerator is the Copilot-in-VS-Code surface. It shares all
// instruction/skill generation with the base copilot generator (embedded)
// and adds a ConfigGenerator that targets VS Code's config locations:
// MCP at ../.vscode/mcp.json (the "servers" schema, full fidelity), with
// hooks and native skills degraded to prose under .github.
type copilotVSCodeGenerator struct {
	copilotGenerator
}

func (g *copilotVSCodeGenerator) Target() string { return "copilot-vscode" }

func (g *copilotVSCodeGenerator) GenerateConfig(cfg model.ProjectConfig) ([]spec.ConfigFile, []string) {
	if cfg.IsEmpty() {
		return nil, nil
	}
	var files []spec.ConfigFile
	var warnings []string

	// MCP — full fidelity via schema-map to VS Code's servers/stdio format.
	if len(cfg.MCPServers) > 0 {
		files = append(files, spec.ConfigFile{Path: "../.vscode/mcp.json", Content: renderVSCodeMCP(cfg.MCPServers)})
	}

	// Hooks — degrade to prose, warn.
	if len(cfg.Hooks) > 0 {
		files = append(files, spec.ConfigFile{Path: "ported-hooks.md", Content: renderHooksProse(cfg.Hooks)})
		warnings = append(warnings, hookWarning(len(cfg.Hooks), ".github/ported-hooks.md"))
	}

	// Native skills — degrade to prose, warn.
	for _, s := range cfg.NativeSkills {
		files = append(files, spec.ConfigFile{Path: "ported-skills/" + s.Name + ".md", Content: renderNativeSkillProse(s)})
	}
	if len(cfg.NativeSkills) > 0 {
		warnings = append(warnings, skillWarning(len(cfg.NativeSkills)))
	}

	return files, warnings
}

func init() {
	spec.Register("copilot-vscode", func() spec.Generator { return &copilotVSCodeGenerator{} })
}
