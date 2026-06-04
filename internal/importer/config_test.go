package importer_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/mirandaguillaume/reify/internal/importer"
	"github.com/mirandaguillaume/reify/pkg/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// writeFile is a helper that creates intermediate dirs and writes content.
func writeFile(t *testing.T, path, content string) {
	t.Helper()
	require.NoError(t, os.MkdirAll(filepath.Dir(path), 0o755))
	require.NoError(t, os.WriteFile(path, []byte(content), 0o644))
}

// TestImportProjectConfig_Hooks verifies that the nested Claude Code
// settings.json hook shape is flattened into []model.Hook.
// Events are sorted alphabetically; within an event array order is preserved;
// within a matcher the inner hooks order is preserved.
// Only entries with type=="command" are included.
func TestImportProjectConfig_Hooks(t *testing.T) {
	dir := t.TempDir()

	settings := map[string]any{
		"hooks": map[string]any{
			"PreToolUse": []any{
				map[string]any{
					"matcher": "Edit",
					"hooks": []any{
						map[string]any{"type": "command", "command": "gofmt -w ."},
					},
				},
			},
			"PostToolUse": []any{
				map[string]any{
					"matcher": "Bash",
					"hooks": []any{
						map[string]any{"type": "command", "command": "echo done"},
						map[string]any{"type": "command", "command": "echo again"},
					},
				},
			},
		},
	}
	raw, err := json.Marshal(settings)
	require.NoError(t, err)
	writeFile(t, filepath.Join(dir, "settings.json"), string(raw))

	cfg, err := importer.ImportProjectConfig(dir, "")
	require.NoError(t, err)

	// Events sorted alphabetically: PostToolUse < PreToolUse.
	want := []model.Hook{
		{Event: "PostToolUse", Matcher: "Bash", Command: "echo done"},
		{Event: "PostToolUse", Matcher: "Bash", Command: "echo again"},
		{Event: "PreToolUse", Matcher: "Edit", Command: "gofmt -w ."},
	}
	assert.Equal(t, want, cfg.Hooks)
}

// TestImportProjectConfig_Hooks_NonCommandIgnored ensures that hook entries
// whose type is not "command" are silently skipped.
func TestImportProjectConfig_Hooks_NonCommandIgnored(t *testing.T) {
	dir := t.TempDir()

	settings := map[string]any{
		"hooks": map[string]any{
			"PreToolUse": []any{
				map[string]any{
					"matcher": "Edit",
					"hooks": []any{
						map[string]any{"type": "other", "command": "should-not-appear"},
						map[string]any{"type": "command", "command": "kept"},
					},
				},
			},
		},
	}
	raw, _ := json.Marshal(settings)
	writeFile(t, filepath.Join(dir, "settings.json"), string(raw))

	cfg, err := importer.ImportProjectConfig(dir, "")
	require.NoError(t, err)
	require.Len(t, cfg.Hooks, 1)
	assert.Equal(t, "kept", cfg.Hooks[0].Command)
}

// TestImportProjectConfig_MCP verifies that .mcp.json is unmarshalled into
// the MCPServers map.
func TestImportProjectConfig_MCP(t *testing.T) {
	dir := t.TempDir()
	mcpFile := filepath.Join(dir, ".mcp.json")

	mcp := map[string]any{
		"mcpServers": map[string]any{
			"git": map[string]any{
				"command": "npx",
				"args":    []any{"-y", "x"},
				"env":     map[string]any{"T": "1"},
			},
			"fs": map[string]any{
				"command": "mcp-fs",
			},
		},
	}
	raw, err := json.Marshal(mcp)
	require.NoError(t, err)
	writeFile(t, mcpFile, string(raw))

	cfg, err := importer.ImportProjectConfig(dir, mcpFile)
	require.NoError(t, err)

	require.Len(t, cfg.MCPServers, 2)

	git := cfg.MCPServers["git"]
	assert.Equal(t, "npx", git.Command)
	assert.Equal(t, []string{"-y", "x"}, git.Args)
	assert.Equal(t, map[string]string{"T": "1"}, git.Env)

	fs := cfg.MCPServers["fs"]
	assert.Equal(t, "mcp-fs", fs.Command)
	assert.Nil(t, fs.Args)
	assert.Nil(t, fs.Env)
}

// TestImportProjectConfig_NativeSkills verifies frontmatter parsing and
// body capture for skills found under skills/<name>/SKILL.md.
func TestImportProjectConfig_NativeSkills(t *testing.T) {
	dir := t.TempDir()

	skillContent := `---
name: scaffold-provider
description: Scaffold a new provider adapter
disable-model-invocation: true
---

## Steps
1. do it
`
	writeFile(t, filepath.Join(dir, "skills", "scaffold-provider", "SKILL.md"), skillContent)

	cfg, err := importer.ImportProjectConfig(dir, "")
	require.NoError(t, err)

	require.Len(t, cfg.NativeSkills, 1)
	s := cfg.NativeSkills[0]
	assert.Equal(t, "scaffold-provider", s.Name)
	assert.Equal(t, "Scaffold a new provider adapter", s.Description)
	assert.True(t, s.DisableModelInvocation)
	assert.Contains(t, s.Body, "## Steps")
}

// TestImportProjectConfig_NativeSkills_NameFallback verifies that when the
// frontmatter omits `name`, the directory name is used instead.
func TestImportProjectConfig_NativeSkills_NameFallback(t *testing.T) {
	dir := t.TempDir()

	skillContent := `---
description: No name in frontmatter
---

Body here.
`
	writeFile(t, filepath.Join(dir, "skills", "my-tool", "SKILL.md"), skillContent)

	cfg, err := importer.ImportProjectConfig(dir, "")
	require.NoError(t, err)

	require.Len(t, cfg.NativeSkills, 1)
	assert.Equal(t, "my-tool", cfg.NativeSkills[0].Name)
}

// TestImportProjectConfig_NativeSkills_Sorted verifies that multiple skills
// are returned sorted by Name for determinism.
func TestImportProjectConfig_NativeSkills_Sorted(t *testing.T) {
	dir := t.TempDir()

	for _, name := range []string{"zebra", "alpha", "middle"} {
		content := "---\nname: " + name + "\n---\nbody\n"
		writeFile(t, filepath.Join(dir, "skills", name, "SKILL.md"), content)
	}

	cfg, err := importer.ImportProjectConfig(dir, "")
	require.NoError(t, err)

	require.Len(t, cfg.NativeSkills, 3)
	assert.Equal(t, "alpha", cfg.NativeSkills[0].Name)
	assert.Equal(t, "middle", cfg.NativeSkills[1].Name)
	assert.Equal(t, "zebra", cfg.NativeSkills[2].Name)
}

// TestImportProjectConfig_AllMissing verifies that an empty claudeDir and a
// non-existent mcpFile both yield an empty ProjectConfig with no error.
func TestImportProjectConfig_AllMissing(t *testing.T) {
	dir := t.TempDir()
	mcpFile := filepath.Join(dir, "nonexistent.mcp.json")

	cfg, err := importer.ImportProjectConfig(dir, mcpFile)
	require.NoError(t, err)
	assert.True(t, cfg.IsEmpty())
}

// TestImportProjectConfig_MalformedJSON verifies that a settings.json with
// invalid JSON returns an error.
func TestImportProjectConfig_MalformedJSON(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "settings.json"), `{ not valid json `)

	_, err := importer.ImportProjectConfig(dir, "")
	assert.Error(t, err)
}

// TestImportProjectConfig_MalformedMCPJSON verifies that a malformed .mcp.json
// returns an error.
func TestImportProjectConfig_MalformedMCPJSON(t *testing.T) {
	dir := t.TempDir()
	mcpFile := filepath.Join(dir, ".mcp.json")
	writeFile(t, mcpFile, `{ bad`)

	_, err := importer.ImportProjectConfig(dir, mcpFile)
	assert.Error(t, err)
}
