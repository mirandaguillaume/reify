package cmd_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/mirandaguillaume/reify/internal/builder"
	"github.com/mirandaguillaume/reify/internal/importer"
	_ "github.com/mirandaguillaume/reify/internal/generator/claude" // register claude target
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// writeRichSource lays out a source project that exercises every modelled
// shape: stdio + http + an unknown-field MCP server, multi-event hooks, and a
// native skill.
func writeRichSource(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	claude := filepath.Join(root, ".claude", "skills", "scaffold")
	require.NoError(t, os.MkdirAll(claude, 0o755))

	settings := `{
  "hooks": {
    "PreToolUse": [{"matcher": "Bash", "hooks": [{"type": "command", "command": "echo pre"}]}],
    "PostToolUse": [{"matcher": "Edit", "hooks": [{"type": "command", "command": "gofmt -w ."}]}]
  }
}`
	require.NoError(t, os.WriteFile(filepath.Join(root, ".claude", "settings.json"), []byte(settings), 0o644))

	// stdio, http (remote), and a server with an unmodelled field (cwd).
	mcp := `{"mcpServers":{
      "git":   {"command":"npx","args":["-y","server-git"],"env":{"T":"1"}},
      "figma": {"type":"http","url":"https://mcp.figma.com/mcp","headers":{"A":"b"}},
      "fs":    {"command":"mcp-fs","cwd":"${workspaceFolder}"}
    }}`
	require.NoError(t, os.WriteFile(filepath.Join(root, ".mcp.json"), []byte(mcp), 0o644))

	require.NoError(t, os.WriteFile(filepath.Join(claude, "SKILL.md"),
		[]byte("---\nname: scaffold\ndescription: scaffold a thing\n---\n## Steps\n1. do it\n"), 0o644))
	return root
}

// TestRoundTrip_ClaudeIsLossless proves import∘build∘import == import for the
// claude target: a project imported, re-emitted, and re-imported yields the
// same ProjectConfig. This guards the whole class of silent-drop bugs (the
// HTTP-MCP corruption was one instance) — including unmodelled fields carried
// via Extra.
func TestRoundTrip_ClaudeIsLossless(t *testing.T) {
	src := writeRichSource(t)
	cfg1, err := importer.ImportProjectConfig(filepath.Join(src, ".claude"), filepath.Join(src, ".mcp.json"))
	require.NoError(t, err)

	// Build the claude config into a second project (settings.json in
	// proj2/.claude, .mcp.json at proj2/ via ../, skills under .claude/skills).
	proj2 := t.TempDir()
	out := filepath.Join(proj2, ".claude")
	warns, err := builder.EmitProjectConfig("claude", out, cfg1)
	require.NoError(t, err)
	assert.Empty(t, warns, "claude ports natively — no degradation")

	cfg2, err := importer.ImportProjectConfig(out, filepath.Join(proj2, ".mcp.json"))
	require.NoError(t, err)

	// MCP must be byte-for-byte faithful, including the http url and the
	// unmodelled cwd carried in Extra.
	assert.Equal(t, cfg1.MCPServers, cfg2.MCPServers, "MCP servers must round-trip losslessly")
	require.Contains(t, cfg2.MCPServers, "figma")
	assert.Equal(t, "https://mcp.figma.com/mcp", cfg2.MCPServers["figma"].URL)
	require.Contains(t, cfg2.MCPServers["fs"].Extra, "cwd", "unmodelled field survived")

	// Hooks must match (set-wise; flattening order is event-sorted both ways).
	assert.ElementsMatch(t, cfg1.Hooks, cfg2.Hooks, "hooks must round-trip")

	// Native skills: same names + key fields.
	require.Len(t, cfg2.NativeSkills, len(cfg1.NativeSkills))
	if len(cfg1.NativeSkills) > 0 {
		assert.Equal(t, cfg1.NativeSkills[0].Name, cfg2.NativeSkills[0].Name)
		assert.Equal(t, cfg1.NativeSkills[0].Description, cfg2.NativeSkills[0].Description)
	}
}
