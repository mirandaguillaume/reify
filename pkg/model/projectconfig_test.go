package model_test

import (
	"encoding/json"
	"testing"

	"github.com/mirandaguillaume/reify/pkg/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// MCPServer must round-trip the standard mcpServers schema shared by
// Claude Code, Cursor, VS Code, etc.
func TestMCPServerUnmarshalsStandardSchema(t *testing.T) {
	raw := `{
      "mcpServers": {
        "git": {"command": "npx", "args": ["-y", "@modelcontextprotocol/server-git"], "env": {"TOKEN": "x"}},
        "fs":  {"command": "mcp-fs"}
      }
    }`
	var doc struct {
		MCPServers map[string]model.MCPServer `json:"mcpServers"`
	}
	require.NoError(t, json.Unmarshal([]byte(raw), &doc))
	require.Len(t, doc.MCPServers, 2)
	git := doc.MCPServers["git"]
	assert.Equal(t, "npx", git.Command)
	assert.Equal(t, []string{"-y", "@modelcontextprotocol/server-git"}, git.Args)
	assert.Equal(t, "x", git.Env["TOKEN"])
	assert.Equal(t, "mcp-fs", doc.MCPServers["fs"].Command)
}

func TestMCPServer_IsRemote(t *testing.T) {
	assert.False(t, model.MCPServer{Command: "npx"}.IsRemote(), "stdio command server is local")
	assert.True(t, model.MCPServer{URL: "https://x/mcp"}.IsRemote(), "url marks remote")
	assert.True(t, model.MCPServer{Type: "http"}.IsRemote(), "http type marks remote")
	assert.True(t, model.MCPServer{Type: "sse"}.IsRemote(), "sse type marks remote")
}

// An HTTP server from a real .mcp.json must round-trip without losing its url
// — the regression guard for the silent-corruption bug (a remote server was
// previously flattened to an empty-command stdio entry).
func TestMCPServer_HTTPRoundTrip(t *testing.T) {
	var s model.MCPServer
	require.NoError(t, json.Unmarshal([]byte(`{"type":"http","url":"https://mcp.figma.com/mcp","headers":{"X":"1"}}`), &s))
	assert.True(t, s.IsRemote())
	assert.Equal(t, "https://mcp.figma.com/mcp", s.URL)

	out, err := json.Marshal(s)
	require.NoError(t, err)
	assert.Contains(t, string(out), `"url":"https://mcp.figma.com/mcp"`)
	assert.NotContains(t, string(out), `"command"`, "no empty command for a remote server")
}

// Hook mirrors the Claude Code settings.json hooks shape (event ->
// [{matcher, hooks:[{type,command}]}]).
func TestProjectConfigHoldsHooks(t *testing.T) {
	pc := model.ProjectConfig{
		Hooks: []model.Hook{
			{Event: "PreToolUse", Matcher: "Edit", Command: "gofmt -w ."},
			{Event: "PostToolUse", Matcher: "Bash", Command: "echo done"},
		},
	}
	assert.Len(t, pc.Hooks, 2)
	assert.Equal(t, "PreToolUse", pc.Hooks[0].Event)
	assert.Equal(t, "gofmt -w .", pc.Hooks[0].Command)
}

func TestProjectConfigHoldsNativeSkills(t *testing.T) {
	pc := model.ProjectConfig{
		NativeSkills: []model.NativeSkill{
			{Name: "scaffold", Description: "scaffold a thing", Body: "## Steps\n1. do it", DisableModelInvocation: true},
		},
	}
	require.Len(t, pc.NativeSkills, 1)
	assert.Equal(t, "scaffold", pc.NativeSkills[0].Name)
	assert.True(t, pc.NativeSkills[0].DisableModelInvocation)
}

func TestProjectConfigEmptyIsZeroValue(t *testing.T) {
	var pc model.ProjectConfig
	assert.Empty(t, pc.Hooks)
	assert.Empty(t, pc.MCPServers)
	assert.Empty(t, pc.NativeSkills)
	assert.True(t, pc.IsEmpty())
}

func TestProjectConfigIsEmpty(t *testing.T) {
	assert.False(t, model.ProjectConfig{Hooks: []model.Hook{{Event: "PreToolUse"}}}.IsEmpty())
	assert.False(t, model.ProjectConfig{MCPServers: map[string]model.MCPServer{"x": {}}}.IsEmpty())
	assert.False(t, model.ProjectConfig{NativeSkills: []model.NativeSkill{{Name: "x"}}}.IsEmpty())
}
