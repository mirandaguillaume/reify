package agents

import (
	"fmt"

	"github.com/mirandaguillaume/reify/internal/generator"
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
		files = append(files, spec.ConfigFile{Path: "mcp.json", Content: generator.RenderMCPServers(cfg.MCPServers)})
		warnings = append(warnings,
			"AGENTS.md defines no standard MCP location — tools that read it (Codex, Zed, ...) "+
				"each configure MCP separately. A reference mcp.json was written; wire it per tool.")
	}

	if len(cfg.Hooks) > 0 {
		files = append(files, spec.ConfigFile{
			Path:    "ported-hooks.md",
			Content: generator.RenderHooksProse(cfg.Hooks, "The AGENTS.md ecosystem"),
		})
		warnings = append(warnings, fmt.Sprintf(
			"%d hook(s) degraded to prose in ported-hooks.md — the AGENTS.md ecosystem has no "+
				"hook system, so they are NOT enforced; the model may ignore them.", len(cfg.Hooks)))
	}

	for _, s := range cfg.NativeSkills {
		files = append(files, spec.ConfigFile{
			Path:    "ported-skills/" + s.Name + ".md",
			Content: generator.RenderNativeSkillBody(s),
		})
	}
	if len(cfg.NativeSkills) > 0 {
		warnings = append(warnings, fmt.Sprintf(
			"%d native skill(s) degraded to prose — the AGENTS.md ecosystem has no native-skill "+
				"concept, so model-invocation control is not preserved.", len(cfg.NativeSkills)))
	}

	return files, warnings
}
