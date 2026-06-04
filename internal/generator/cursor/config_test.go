package cursor_test

import (
	"encoding/json"
	"strings"
	"testing"

	_ "github.com/mirandaguillaume/reify/internal/generator/cursor" // register the cursor target
	"github.com/mirandaguillaume/reify/pkg/model"
	"github.com/mirandaguillaume/reify/pkg/spec"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newCursor() spec.ConfigGenerator {
	g, _ := spec.Get("cursor")
	cg, ok := g.(spec.ConfigGenerator)
	if !ok {
		panic("cursor generator must implement ConfigGenerator")
	}
	return cg
}

func TestCursorConfig_Empty(t *testing.T) {
	files, warns := newCursor().GenerateConfig(model.ProjectConfig{})
	assert.Empty(t, files)
	assert.Empty(t, warns)
}

func TestCursorConfig_MCP_FullFidelity(t *testing.T) {
	// MCP shares the mcpServers schema across harnesses — only the path
	// differs. Cursor reads .cursor/mcp.json. No loss, so no warning.
	cfg := model.ProjectConfig{MCPServers: map[string]model.MCPServer{
		"git": {Command: "npx", Args: []string{"-y", "server-git"}, Env: map[string]string{"T": "1"}},
	}}
	files, warns := newCursor().GenerateConfig(cfg)
	assert.Empty(t, warns, "MCP is a path-map, not a degradation — no warning")

	var mcp *spec.ConfigFile
	for i := range files {
		if files[i].Path == "mcp.json" {
			mcp = &files[i]
		}
	}
	require.NotNil(t, mcp, "must emit mcp.json in the .cursor output dir")

	var doc struct {
		MCPServers map[string]model.MCPServer `json:"mcpServers"`
	}
	require.NoError(t, json.Unmarshal([]byte(mcp.Content), &doc))
	assert.Equal(t, "npx", doc.MCPServers["git"].Command)
}

func TestCursorConfig_Hooks_DegradeToProseWithWarning(t *testing.T) {
	// Cursor has no hook system: an enforced guarantee becomes a prose
	// suggestion, and we MUST warn that the guarantee is lost.
	cfg := model.ProjectConfig{Hooks: []model.Hook{
		{Event: "PostToolUse", Matcher: "Edit", Command: "gofmt -w ."},
	}}
	files, warns := newCursor().GenerateConfig(cfg)

	var prose *spec.ConfigFile
	for i := range files {
		if strings.HasPrefix(files[i].Path, "rules/") && strings.Contains(files[i].Path, "hook") {
			prose = &files[i]
		}
	}
	require.NotNil(t, prose, "hooks degrade to a rules/*.mdc prose file")
	assert.Contains(t, prose.Content, "gofmt -w .", "the command survives as prose")
	assert.Contains(t, prose.Content, "Edit", "the trigger is documented")

	require.NotEmpty(t, warns, "degrading a hook must warn")
	joined := strings.Join(warns, " ")
	assert.Contains(t, strings.ToLower(joined), "hook")
	assert.Contains(t, strings.ToLower(joined), "not enforced")
}

func TestCursorConfig_NativeSkills_DegradeToRulesWithWarning(t *testing.T) {
	cfg := model.ProjectConfig{NativeSkills: []model.NativeSkill{
		{Name: "scaffold", Description: "scaffold a thing", Body: "## Steps\n1. do it", DisableModelInvocation: true},
	}}
	files, warns := newCursor().GenerateConfig(cfg)

	var skill *spec.ConfigFile
	for i := range files {
		if files[i].Path == "rules/scaffold.mdc" {
			skill = &files[i]
		}
	}
	require.NotNil(t, skill, "native skill degrades to a cursor rule")
	assert.Contains(t, skill.Content, "## Steps", "skill body survives as prose")

	require.NotEmpty(t, warns, "degrading a native skill must warn")
	assert.Contains(t, strings.ToLower(strings.Join(warns, " ")), "skill")
}

func ruleContent(t *testing.T, cfg model.ProjectConfig, path string) string {
	t.Helper()
	files, _ := newCursor().GenerateConfig(cfg)
	for _, f := range files {
		if f.Path == path {
			return f.Content
		}
	}
	t.Fatalf("no file at %s", path)
	return ""
}

func TestCursorConfig_SkillWithDescriptionIsAgentRequested(t *testing.T) {
	// A model-invocable skill (has a description) maps to a Cursor
	// "Agent Requested" rule: description present, alwaysApply:false.
	c := ruleContent(t, model.ProjectConfig{NativeSkills: []model.NativeSkill{
		{Name: "scaffold", Description: "scaffold a thing", Body: "x"},
	}}, "rules/scaffold.mdc")
	assert.Contains(t, c, "description: scaffold a thing")
	assert.Contains(t, c, "alwaysApply: false")
}

func TestCursorConfig_SkillWithoutDescriptionFallsBackToAlways(t *testing.T) {
	// Without a description there is nothing for the agent to match on, so it
	// must NOT be left as a dead Manual rule — fall back to Always.
	c := ruleContent(t, model.ProjectConfig{NativeSkills: []model.NativeSkill{
		{Name: "scaffold", Body: "x"},
	}}, "rules/scaffold.mdc")
	assert.NotContains(t, c, "description:")
	assert.Contains(t, c, "alwaysApply: true")
}
