//go:build validation

// Package claude validation tests verify reify's emitted Claude Code config
// against EXTERNAL sources of truth — not reify's own assumptions:
//
//   - settings.json (hooks) is validated against the official SchemaStore
//     "Claude Code Settings" JSON schema (vendored in testdata/).
//   - .mcp.json is fed to the real `claude` CLI, which must recognize the
//     ported server (harness acceptance, not self-consistency).
//
// These break the circularity of the unit tests (which only assert reify
// emits what its author coded). They are gated behind the `validation` build
// tag and skip gracefully when the validator / claude CLI is absent, so they
// never block normal CI:
//
//	go test -tags validation ./internal/generator/claude/
package claude_test

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	_ "github.com/mirandaguillaume/reify/internal/generator/claude"
	"github.com/mirandaguillaume/reify/pkg/model"
	"github.com/mirandaguillaume/reify/pkg/spec"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// realProjectConfig is a non-trivial config: a hook and an MCP server.
func realProjectConfig() model.ProjectConfig {
	return model.ProjectConfig{
		Hooks: []model.Hook{
			{Event: "PostToolUse", Matcher: "Edit", Command: "gofmt -w ."},
		},
		MCPServers: map[string]model.MCPServer{
			"git": {Command: "npx", Args: []string{"-y", "server-git"}},
		},
	}
}

// emitConfig writes the claude ConfigGenerator output into outputDir, honoring
// each file's relative path (including ../ for root files), exactly as the
// builder does.
func emitConfig(t *testing.T, outputDir string) []spec.ConfigFile {
	t.Helper()
	g, err := spec.Get("claude")
	require.NoError(t, err)
	cg, ok := g.(spec.ConfigGenerator)
	require.True(t, ok)
	files, _ := cg.GenerateConfig(realProjectConfig())
	for _, f := range files {
		full := filepath.Join(outputDir, filepath.FromSlash(f.Path))
		require.NoError(t, os.MkdirAll(filepath.Dir(full), 0o755))
		require.NoError(t, os.WriteFile(full, []byte(f.Content), 0o644))
	}
	return files
}

// jsonSchemaValidator returns a command prefix for validating an instance
// against a schema, or nil if no validator is installed.
func jsonSchemaValidator(instance, schema string) []string {
	if p, err := exec.LookPath("check-jsonschema"); err == nil {
		return []string{p, "--schemafile", schema, instance}
	}
	if p, err := exec.LookPath("jsonschema"); err == nil {
		return []string{p, "--instance", instance, schema}
	}
	return nil
}

// TestValidation_ClaudeSettingsConformsToOfficialSchema validates the emitted
// settings.json against the vendored official SchemaStore schema.
func TestValidation_ClaudeSettingsConformsToOfficialSchema(t *testing.T) {
	dir := t.TempDir()
	out := filepath.Join(dir, "out")
	emitConfig(t, out)

	settings := filepath.Join(out, "settings.json")
	require.FileExists(t, settings)

	schema, err := filepath.Abs("testdata/claude-code-settings.schema.json")
	require.NoError(t, err)
	require.FileExists(t, schema)

	cmd := jsonSchemaValidator(settings, schema)
	if cmd == nil {
		t.Skip("no JSON schema validator installed (check-jsonschema or jsonschema)")
	}
	outBytes, err := exec.Command(cmd[0], cmd[1:]...).CombinedOutput()
	assert.NoError(t, err, "emitted settings.json must conform to the official Claude Code Settings schema:\n%s", outBytes)
}

// TestValidation_ClaudeMCPAcceptedByRealCLI feeds the emitted .mcp.json to the
// real claude CLI and asserts it recognizes the ported server.
func TestValidation_ClaudeMCPAcceptedByRealCLI(t *testing.T) {
	claude, err := exec.LookPath("claude")
	if err != nil {
		t.Skip("claude CLI not installed")
	}
	dir := t.TempDir()
	emitConfig(t, filepath.Join(dir, "out")) // .mcp.json lands at dir/.mcp.json via ../
	require.FileExists(t, filepath.Join(dir, ".mcp.json"))

	cmd := exec.Command(claude, "mcp", "get", "git")
	cmd.Dir = dir
	outBytes, err := cmd.CombinedOutput()
	require.NoError(t, err, "claude mcp get failed:\n%s", outBytes)
	got := string(outBytes)
	assert.Contains(t, got, "git", "claude must recognize the ported server")
	assert.Contains(t, strings.ToLower(got), ".mcp.json", "claude must attribute it to the .mcp.json reify wrote")
}
