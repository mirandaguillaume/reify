package copilot_test

import (
	"encoding/json"
	"strings"
	"testing"

	_ "github.com/mirandaguillaume/reify/internal/generator/copilot" // register copilot targets
	"github.com/mirandaguillaume/reify/pkg/model"
	"github.com/mirandaguillaume/reify/pkg/spec"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func cfgGen(t *testing.T, target string) spec.ConfigGenerator {
	t.Helper()
	g, err := spec.Get(target)
	require.NoError(t, err, "target %q must be registered", target)
	cg, ok := g.(spec.ConfigGenerator)
	require.True(t, ok, "target %q must implement ConfigGenerator", target)
	return cg
}

func TestCopilotVSCode_Registered(t *testing.T) {
	g, err := spec.Get("copilot-vscode")
	require.NoError(t, err)
	assert.Equal(t, "copilot-vscode", g.Target())
	// Instructions are shared with the base copilot generator.
	assert.Equal(t, ".github", g.DefaultOutputDir())
}

func TestCopilotVSCode_MCP_UsesServersSchema(t *testing.T) {
	// VS Code's .vscode/mcp.json uses the "servers" key (NOT mcpServers) and
	// requires "type": "stdio" per server. This is a schema-map, not just a
	// path-map — so it is full fidelity with no warning.
	cfg := model.ProjectConfig{MCPServers: map[string]model.MCPServer{
		"git": {Command: "npx", Args: []string{"-y", "server-git"}, Env: map[string]string{"T": "1"}},
	}}
	files, warns := cfgGen(t, "copilot-vscode").GenerateConfig(cfg)
	assert.Empty(t, warns, "MCP schema-map to VS Code is lossless")

	var mcp *spec.ConfigFile
	for i := range files {
		if strings.HasSuffix(files[i].Path, ".vscode/mcp.json") {
			mcp = &files[i]
		}
	}
	require.NotNil(t, mcp, "must emit ../.vscode/mcp.json")
	assert.True(t, strings.HasPrefix(mcp.Path, "../"), "vscode dir is at repo root, not under .github")

	var doc struct {
		Servers    map[string]map[string]any `json:"servers"`
		MCPServers map[string]any            `json:"mcpServers"`
	}
	require.NoError(t, json.Unmarshal([]byte(mcp.Content), &doc))
	assert.Empty(t, doc.MCPServers, "must NOT use the mcpServers key")
	require.Contains(t, doc.Servers, "git")
	assert.Equal(t, "stdio", doc.Servers["git"]["type"], "stdio type is required by VS Code")
	assert.Equal(t, "npx", doc.Servers["git"]["command"])
}

func TestCopilotVSCode_HooksDegradeWithWarning(t *testing.T) {
	cfg := model.ProjectConfig{Hooks: []model.Hook{
		{Event: "PostToolUse", Matcher: "Edit", Command: "gofmt -w ."},
	}}
	files, warns := cfgGen(t, "copilot-vscode").GenerateConfig(cfg)
	require.NotEmpty(t, warns)
	assert.Contains(t, strings.ToLower(strings.Join(warns, " ")), "not enforced")

	var found bool
	for _, f := range files {
		if strings.Contains(f.Path, "hook") {
			found = true
			assert.Contains(t, f.Content, "gofmt -w .")
		}
	}
	assert.True(t, found, "hooks degrade to a prose file")
}
