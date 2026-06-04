package copilot

import (
	"github.com/mirandaguillaume/reify/pkg/model"
	"github.com/mirandaguillaume/reify/pkg/spec"
)

// copilotJetBrainsGenerator is the Copilot-in-JetBrains surface (also Eclipse
// / Xcode). Per the vendor docs, MCP servers there are configured through the
// IDE settings UI (Copilot icon -> Edit settings -> MCP Servers); there is no
// documented stable project-level config file reify can write. So this is the
// lowest-fidelity surface: reify emits a reference snippet and warns that it
// must be entered through the IDE. Hooks and native skills degrade to prose.
type copilotJetBrainsGenerator struct {
	copilotGenerator
}

func (g *copilotJetBrainsGenerator) Target() string { return "copilot-jetbrains" }

func (g *copilotJetBrainsGenerator) GenerateConfig(cfg model.ProjectConfig) ([]spec.ConfigFile, []string) {
	if cfg.IsEmpty() {
		return nil, nil
	}
	var files []spec.ConfigFile
	var warnings []string

	if len(cfg.MCPServers) > 0 {
		// Emit as a reference snippet, not an authoritative auto-loaded file.
		files = append(files, spec.ConfigFile{Path: "copilot-jetbrains-mcp.json", Content: renderVSCodeMCP(cfg.MCPServers)})
		warnings = append(warnings,
			"MCP cannot be auto-configured for JetBrains/Eclipse/Xcode — it is UI-managed "+
				"(Copilot icon -> Edit settings -> MCP Servers). A reference snippet was written to "+
				".github/copilot-jetbrains-mcp.json; add it through the IDE. Schema may vary by IDE version.")
		if w := droppedExtraWarning(cfg.MCPServers); w != "" {
			warnings = append(warnings, w)
		}
	}

	if len(cfg.Hooks) > 0 {
		files = append(files, spec.ConfigFile{Path: "ported-hooks.md", Content: renderHooksProse(cfg.Hooks)})
		warnings = append(warnings, hookWarning(len(cfg.Hooks), ".github/ported-hooks.md"))
	}

	for _, s := range cfg.NativeSkills {
		files = append(files, spec.ConfigFile{Path: "ported-skills/" + s.Name + ".md", Content: renderNativeSkillProse(s)})
	}
	if len(cfg.NativeSkills) > 0 {
		warnings = append(warnings, skillWarning(len(cfg.NativeSkills)))
	}

	return files, warnings
}

func init() {
	spec.Register("copilot-jetbrains", func() spec.Generator { return &copilotJetBrainsGenerator{} })
}
