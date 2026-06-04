package aider

import (
	"fmt"

	"github.com/mirandaguillaume/reify/internal/generator"
	"github.com/mirandaguillaume/reify/pkg/model"
	"github.com/mirandaguillaume/reify/pkg/spec"
)

// GenerateConfig ports project runtime config to aider, which has no native
// hook, MCP, or native-skill mechanism — so every pillar degrades to prose
// (under the .reify-aider staging dir) with a warning. MCP is emitted as a
// reference snippet only.
func (g *aiderGenerator) GenerateConfig(cfg model.ProjectConfig) ([]spec.ConfigFile, []string) {
	if cfg.IsEmpty() {
		return nil, nil
	}
	var files []spec.ConfigFile
	var warnings []string

	if len(cfg.MCPServers) > 0 {
		files = append(files, spec.ConfigFile{Path: "mcp.json", Content: generator.RenderMCPServers(cfg.MCPServers)})
		warnings = append(warnings,
			"aider has no MCP mechanism — a reference mcp.json was written under .reify-aider/, "+
				"but aider will not load it. Wire these servers manually if your aider setup supports MCP.")
	}

	if len(cfg.Hooks) > 0 {
		files = append(files, spec.ConfigFile{Path: "ported-hooks.md", Content: generator.RenderHooksProse(cfg.Hooks, "aider")})
		warnings = append(warnings, fmt.Sprintf(
			"%d hook(s) degraded to prose — aider has no hook system, so they are NOT enforced; "+
				"the model may ignore them.", len(cfg.Hooks)))
	}

	for _, s := range cfg.NativeSkills {
		files = append(files, spec.ConfigFile{Path: "ported-skills/" + s.Name + ".md", Content: generator.RenderNativeSkillBody(s)})
	}
	if len(cfg.NativeSkills) > 0 {
		warnings = append(warnings, fmt.Sprintf(
			"%d native skill(s) degraded to prose — aider has no native-skill concept, so "+
				"model-invocation control is not preserved.", len(cfg.NativeSkills)))
	}

	return files, warnings
}
