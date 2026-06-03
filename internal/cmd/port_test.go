package cmd_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mirandaguillaume/reify/internal/cmd"
	_ "github.com/mirandaguillaume/reify/internal/generator/cursor" // register cursor target
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// writeSourceProject lays out a minimal source project with a Claude Code
// settings.json (one hook) and a .mcp.json (one server).
func writeSourceProject(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	claude := filepath.Join(root, ".claude")
	require.NoError(t, os.MkdirAll(claude, 0o755))

	settings := `{
  "hooks": {
    "PostToolUse": [
      {"matcher": "Edit", "hooks": [{"type": "command", "command": "gofmt -w ."}]}
    ]
  }
}`
	require.NoError(t, os.WriteFile(filepath.Join(claude, "settings.json"), []byte(settings), 0o644))

	mcp := `{"mcpServers": {"git": {"command": "npx", "args": ["-y", "server-git"]}}}`
	require.NoError(t, os.WriteFile(filepath.Join(root, ".mcp.json"), []byte(mcp), 0o644))

	return root
}

func TestPortProjectConfig_CursorDegradesHooksKeepsMCP(t *testing.T) {
	src := writeSourceProject(t)
	out := t.TempDir()

	warns, err := cmd.PortProjectConfig(src, "cursor", out)
	require.NoError(t, err)

	// MCP ported at full fidelity to .cursor/mcp.json (out is the .cursor dir).
	mcp, err := os.ReadFile(filepath.Join(out, "mcp.json"))
	require.NoError(t, err, "MCP must be ported to mcp.json")
	assert.Contains(t, string(mcp), `"git"`)
	assert.Contains(t, string(mcp), "npx")

	// Hook degraded to a prose rule.
	prose, err := os.ReadFile(filepath.Join(out, "rules", "ported-hooks.mdc"))
	require.NoError(t, err, "hook must degrade to a prose rule")
	assert.Contains(t, string(prose), "gofmt -w .")

	// And a warning says the guarantee is gone.
	require.NotEmpty(t, warns)
	assert.Contains(t, strings.ToLower(strings.Join(warns, " ")), "not enforced")
}

func TestPortProjectConfig_EmptySourceIsNoOp(t *testing.T) {
	// A project with no .claude and no .mcp.json ports nothing, without error.
	warns, err := cmd.PortProjectConfig(t.TempDir(), "cursor", t.TempDir())
	require.NoError(t, err)
	assert.Empty(t, warns)
}

func TestPortProjectConfig_ClaudeRoundTrip(t *testing.T) {
	// Porting a Claude source back to the claude target preserves the hook
	// natively (settings.json), so there is no degradation warning.
	src := writeSourceProject(t)
	out := t.TempDir()

	warns, err := cmd.PortProjectConfig(src, "claude", out)
	require.NoError(t, err)
	assert.Empty(t, warns, "claude supports hooks/MCP natively — no degradation")

	_, err = os.Stat(filepath.Join(out, "settings.json"))
	require.NoError(t, err, "hooks re-emitted to settings.json")
}
