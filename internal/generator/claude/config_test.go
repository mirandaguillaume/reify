package claude_test

import (
	"encoding/json"
	"strings"
	"testing"

	_ "github.com/mirandaguillaume/reify/internal/generator/claude" // register the claude target
	"github.com/mirandaguillaume/reify/pkg/model"
	"github.com/mirandaguillaume/reify/pkg/spec"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newClaude() spec.ConfigGenerator {
	g, _ := spec.Get("claude")
	cg, ok := g.(spec.ConfigGenerator)
	if !ok {
		panic("claude generator must implement ConfigGenerator")
	}
	return cg
}

func TestClaudeConfig_Empty(t *testing.T) {
	files, warns := newClaude().GenerateConfig(model.ProjectConfig{})
	assert.Empty(t, files)
	assert.Empty(t, warns)
}

func TestClaudeConfig_Hooks(t *testing.T) {
	cfg := model.ProjectConfig{Hooks: []model.Hook{
		{Event: "PreToolUse", Matcher: "Edit", Command: "gofmt -w ."},
		{Event: "PreToolUse", Matcher: "Edit", Command: "goimports -w ."},
		{Event: "PostToolUse", Matcher: "Bash", Command: "echo done"},
	}}
	files, warns := newClaude().GenerateConfig(cfg)
	assert.Empty(t, warns, "claude supports hooks natively — no degradation warning")

	var settings *spec.ConfigFile
	for i := range files {
		if files[i].Path == "settings.json" {
			settings = &files[i]
		}
	}
	require.NotNil(t, settings, "must emit settings.json")

	// Re-parse and verify the nested Claude Code hook schema round-trips.
	var doc struct {
		Hooks map[string][]struct {
			Matcher string `json:"matcher"`
			Hooks   []struct {
				Type    string `json:"type"`
				Command string `json:"command"`
			} `json:"hooks"`
		} `json:"hooks"`
	}
	require.NoError(t, json.Unmarshal([]byte(settings.Content), &doc))
	require.Len(t, doc.Hooks["PreToolUse"], 1, "same event+matcher grouped")
	assert.Equal(t, "Edit", doc.Hooks["PreToolUse"][0].Matcher)
	require.Len(t, doc.Hooks["PreToolUse"][0].Hooks, 2, "two commands under one matcher")
	assert.Equal(t, "command", doc.Hooks["PreToolUse"][0].Hooks[0].Type)
	assert.Equal(t, "gofmt -w .", doc.Hooks["PreToolUse"][0].Hooks[0].Command)
	require.Len(t, doc.Hooks["PostToolUse"], 1)
}

func TestClaudeConfig_MCP(t *testing.T) {
	cfg := model.ProjectConfig{MCPServers: map[string]model.MCPServer{
		"git": {Command: "npx", Args: []string{"-y", "x"}, Env: map[string]string{"T": "1"}},
	}}
	files, warns := newClaude().GenerateConfig(cfg)
	assert.Empty(t, warns)

	var mcp *spec.ConfigFile
	for i := range files {
		if strings.HasSuffix(files[i].Path, ".mcp.json") {
			mcp = &files[i]
		}
	}
	require.NotNil(t, mcp, "must emit a .mcp.json (at repo root via ../)")
	assert.True(t, strings.HasPrefix(mcp.Path, "../"), "mcp file goes to repo root")

	var doc struct {
		MCPServers map[string]model.MCPServer `json:"mcpServers"`
	}
	require.NoError(t, json.Unmarshal([]byte(mcp.Content), &doc))
	assert.Equal(t, "npx", doc.MCPServers["git"].Command)
}

func TestClaudeConfig_NativeSkills(t *testing.T) {
	cfg := model.ProjectConfig{NativeSkills: []model.NativeSkill{
		{Name: "scaffold", Description: "scaffold a thing", Body: "## Steps\n1. do it", DisableModelInvocation: true},
	}}
	files, warns := newClaude().GenerateConfig(cfg)
	assert.Empty(t, warns)

	var skill *spec.ConfigFile
	for i := range files {
		if files[i].Path == "skills/scaffold/SKILL.md" {
			skill = &files[i]
		}
	}
	require.NotNil(t, skill, "must emit skills/scaffold/SKILL.md")
	assert.True(t, strings.HasPrefix(skill.Content, "---"), "frontmatter present")
	assert.Contains(t, skill.Content, "name: scaffold")
	assert.Contains(t, skill.Content, "disable-model-invocation: true")
	assert.Contains(t, skill.Content, "## Steps")
}
