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

func TestCopilotVSCode_RemoteMCPKeepsURL(t *testing.T) {
	// A remote (HTTP) server must keep its url and type — not be flattened to
	// an empty-command stdio entry (the silent-corruption regression).
	cfg := model.ProjectConfig{MCPServers: map[string]model.MCPServer{
		"figma": {Type: "http", URL: "https://mcp.figma.com/mcp"},
	}}
	files, _ := cfgGen(t, "copilot-vscode").GenerateConfig(cfg)
	var content string
	for _, f := range files {
		if strings.HasSuffix(f.Path, ".vscode/mcp.json") {
			content = f.Content
		}
	}
	require.NotEmpty(t, content)
	var doc struct {
		Servers map[string]map[string]any `json:"servers"`
	}
	require.NoError(t, json.Unmarshal([]byte(content), &doc))
	assert.Equal(t, "http", doc.Servers["figma"]["type"])
	assert.Equal(t, "https://mcp.figma.com/mcp", doc.Servers["figma"]["url"])
	_, hasCmd := doc.Servers["figma"]["command"]
	assert.False(t, hasCmd, "remote server must not carry a command")
}

func TestCopilotCLI_RemoteMCPKeepsURL(t *testing.T) {
	cfg := model.ProjectConfig{MCPServers: map[string]model.MCPServer{
		"figma": {Type: "http", URL: "https://mcp.figma.com/mcp"},
	}}
	files, _ := cfgGen(t, "copilot-cli").GenerateConfig(cfg)
	var content string
	for _, f := range files {
		if f.Path == "copilot-cli-mcp-config.json" {
			content = f.Content
		}
	}
	require.NotEmpty(t, content)
	var doc struct {
		MCPServers map[string]map[string]any `json:"mcpServers"`
	}
	require.NoError(t, json.Unmarshal([]byte(content), &doc))
	assert.Equal(t, "http", doc.MCPServers["figma"]["type"])
	assert.Equal(t, "https://mcp.figma.com/mcp", doc.MCPServers["figma"]["url"])
}

func TestCopilotVSCode_WarnsOnDroppedExtraFields(t *testing.T) {
	// A claude-specific / unmodelled field has no place in the VS Code schema,
	// so it is dropped — but the loss must be announced, not silent.
	cfg := model.ProjectConfig{MCPServers: map[string]model.MCPServer{
		"fs": {Command: "mcp-fs", Extra: map[string]json.RawMessage{"cwd": json.RawMessage(`"x"`)}},
	}}
	files, warns := cfgGen(t, "copilot-vscode").GenerateConfig(cfg)

	var content string
	for _, f := range files {
		if strings.HasSuffix(f.Path, ".vscode/mcp.json") {
			content = f.Content
		}
	}
	assert.NotContains(t, content, "cwd", "unmodelled field is not carried to the VS Code schema")

	joined := strings.ToLower(strings.Join(warns, " "))
	assert.Contains(t, joined, "cwd", "the dropped field is named in a warning")
	assert.Contains(t, joined, "not carried")
}

func TestCopilotVSCode_NoExtraNoWarning(t *testing.T) {
	cfg := model.ProjectConfig{MCPServers: map[string]model.MCPServer{
		"git": {Command: "npx"},
	}}
	_, warns := cfgGen(t, "copilot-vscode").GenerateConfig(cfg)
	for _, w := range warns {
		assert.NotContains(t, strings.ToLower(w), "not carried", "no drop warning when there is no Extra")
	}
}

func TestCopilotSurfaces_ShareInstructionsOutputDir(t *testing.T) {
	// All three surfaces inherit the base copilot instructions output (.github).
	for _, target := range []string{"copilot-vscode", "copilot-cli", "copilot-jetbrains"} {
		g := cfgGen(t, target)
		assert.Equal(t, ".github", g.(interface{ DefaultOutputDir() string }).DefaultOutputDir(), target)
	}
}
