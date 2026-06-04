package copilot

import (
	"github.com/mirandaguillaume/reify/pkg/model"
	"github.com/mirandaguillaume/reify/pkg/spec"
)

// copilotCLIGenerator is the GitHub Copilot CLI surface. Its MCP config lives
// at ~/.copilot/mcp-config.json — a USER-level path, outside the project the
// compiler controls. So reify emits a portable artifact in the project (in
// the CLI's own mcpServers/type:local/tools schema) and warns that it must be
// copied to the user-level location. Hooks and native skills degrade to prose.
type copilotCLIGenerator struct {
	copilotGenerator
}

func (g *copilotCLIGenerator) Target() string { return "copilot-cli" }

func (g *copilotCLIGenerator) GenerateConfig(cfg model.ProjectConfig) ([]spec.ConfigFile, []string) {
	if cfg.IsEmpty() {
		return nil, nil
	}
	var files []spec.ConfigFile
	var warnings []string

	if len(cfg.MCPServers) > 0 {
		files = append(files, spec.ConfigFile{Path: "copilot-cli-mcp-config.json", Content: renderCLIMCP(cfg.MCPServers)})
		warnings = append(warnings,
			"MCP written as a portable artifact at .github/copilot-cli-mcp-config.json — "+
				"Copilot CLI reads ~/.copilot/mcp-config.json (user-level, outside the project); "+
				"copy it there or set COPILOT_HOME.")
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
	spec.Register("copilot-cli", func() spec.Generator { return &copilotCLIGenerator{} })
}
