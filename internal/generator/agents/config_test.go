package agents_test

import (
	"strings"
	"testing"

	_ "github.com/mirandaguillaume/reify/internal/generator/agents" // register the agents target
	"github.com/mirandaguillaume/reify/pkg/model"
	"github.com/mirandaguillaume/reify/pkg/spec"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func agentsConfig(t *testing.T) spec.ConfigGenerator {
	t.Helper()
	g, err := spec.Get("agents")
	require.NoError(t, err)
	cg, ok := g.(spec.ConfigGenerator)
	require.True(t, ok, "agents generator must implement ConfigGenerator")
	return cg
}

func TestAgentsConfig_Empty(t *testing.T) {
	files, warns := agentsConfig(t).GenerateConfig(model.ProjectConfig{})
	assert.Empty(t, files)
	assert.Empty(t, warns)
}

func TestAgentsConfig_EverythingDegradesWithWarnings(t *testing.T) {
	cfg := model.ProjectConfig{
		MCPServers:   map[string]model.MCPServer{"git": {Command: "npx"}},
		Hooks:        []model.Hook{{Event: "PostToolUse", Matcher: "Edit", Command: "gofmt -w ."}},
		NativeSkills: []model.NativeSkill{{Name: "scaffold", Body: "## Steps\n1. do it"}},
	}
	files, warns := agentsConfig(t).GenerateConfig(cfg)

	paths := map[string]string{}
	for _, f := range files {
		paths[f.Path] = f.Content
	}
	assert.Contains(t, paths, "mcp.json")
	assert.Contains(t, paths, "ported-hooks.md")
	assert.Contains(t, paths, "ported-skills/scaffold.md")
	assert.Contains(t, paths["ported-hooks.md"], "gofmt -w .")

	// AGENTS.md has no native channel for any pillar — all three warn.
	require.Len(t, warns, 3)
	joined := strings.ToLower(strings.Join(warns, " "))
	assert.Contains(t, joined, "mcp")
	assert.Contains(t, joined, "hook")
	assert.Contains(t, joined, "skill")
}
