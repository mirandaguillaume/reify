package copilot_test

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/mirandaguillaume/reify/pkg/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCopilotCLI_MCPUsesMcpServersSchemaWithUserLevelWarning(t *testing.T) {
	cfg := model.ProjectConfig{MCPServers: map[string]model.MCPServer{
		"git": {Command: "npx", Args: []string{"-y", "server-git"}},
	}}
	files, warns := cfgGen(t, "copilot-cli").GenerateConfig(cfg)

	var content string
	for _, f := range files {
		if f.Path == "copilot-cli-mcp-config.json" {
			content = f.Content
		}
	}
	require.NotEmpty(t, content, "CLI emits a portable mcp-config.json artifact")

	var doc struct {
		MCPServers map[string]map[string]any `json:"mcpServers"`
	}
	require.NoError(t, json.Unmarshal([]byte(content), &doc))
	require.Contains(t, doc.MCPServers, "git")
	assert.Equal(t, "local", doc.MCPServers["git"]["type"], "CLI requires type:local")
	assert.NotNil(t, doc.MCPServers["git"]["tools"], "CLI requires a tools list")

	// Even MCP warns here: the real location is user-level, out of project scope.
	assert.Contains(t, strings.ToLower(strings.Join(warns, " ")), "user-level")
}

func TestCopilotJetBrains_MCPIsUIManagedWarning(t *testing.T) {
	cfg := model.ProjectConfig{MCPServers: map[string]model.MCPServer{
		"git": {Command: "npx"},
	}}
	files, warns := cfgGen(t, "copilot-jetbrains").GenerateConfig(cfg)

	var found bool
	for _, f := range files {
		if f.Path == "copilot-jetbrains-mcp.json" {
			found = true
		}
	}
	assert.True(t, found, "JetBrains emits a reference snippet")
	assert.Contains(t, strings.ToLower(strings.Join(warns, " ")), "ui-managed")
}

func TestCopilotSurfaces_ShareInstructionsOutputDir(t *testing.T) {
	// All three surfaces inherit the base copilot instructions output (.github).
	for _, target := range []string{"copilot-vscode", "copilot-cli", "copilot-jetbrains"} {
		g := cfgGen(t, target)
		assert.Equal(t, ".github", g.(interface{ DefaultOutputDir() string }).DefaultOutputDir(), target)
	}
}
