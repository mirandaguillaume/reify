package discovery

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// --- DiscoverAgentFiles (moved from internal/doctor/directory_test.go) ---

func TestDiscoverAgentFiles_MixedDirectory(t *testing.T) {
	root := t.TempDir()

	claudeDir := filepath.Join(root, ".claude", "agents")
	require.NoError(t, os.MkdirAll(claudeDir, 0755))
	require.NoError(t, os.WriteFile(
		filepath.Join(claudeDir, "reviewer.md"),
		[]byte("---\nname: reviewer\ntools: Read\n---\n# Agent\nBody content here."),
		0644,
	))

	copilotDir := filepath.Join(root, ".github", "agents")
	require.NoError(t, os.MkdirAll(copilotDir, 0755))
	require.NoError(t, os.WriteFile(
		filepath.Join(copilotDir, "dash.agent.md"),
		[]byte("---\ndescription: dash agent\n---\n# Dash\nBody."),
		0644,
	))

	require.NoError(t, os.WriteFile(
		filepath.Join(root, "README.md"),
		[]byte("# My Project\nThis is not an agent."),
		0644,
	))
	require.NoError(t, os.WriteFile(
		filepath.Join(root, "config.yaml"),
		[]byte("key: value\n"),
		0644,
	))

	files, err := DiscoverAgentFiles(root)
	require.NoError(t, err)
	assert.Len(t, files, 2, "should discover 2 agent files")
}

func TestDiscoverAgentFiles_EmptyDirectory(t *testing.T) {
	root := t.TempDir()
	files, err := DiscoverAgentFiles(root)
	require.NoError(t, err)
	assert.Empty(t, files)
}

func TestDiscoverAgentFiles_SingleFile(t *testing.T) {
	root := t.TempDir()
	claudeDir := filepath.Join(root, ".claude", "agents")
	require.NoError(t, os.MkdirAll(claudeDir, 0755))
	require.NoError(t, os.WriteFile(
		filepath.Join(claudeDir, "test.md"),
		[]byte("---\nname: test\ntools: Read\n---\n# Agent\nBody."),
		0644,
	))

	files, err := DiscoverAgentFiles(root)
	require.NoError(t, err)
	assert.Len(t, files, 1)
}

func TestDiscoverAgentFiles_NestedDirectories(t *testing.T) {
	root := t.TempDir()
	deep := filepath.Join(root, ".claude", "agents", "nested")
	require.NoError(t, os.MkdirAll(deep, 0755))
	require.NoError(t, os.WriteFile(
		filepath.Join(deep, "deep-agent.md"),
		[]byte("---\nname: deep\ntools: Read\n---\n# Deep Agent\nBody."),
		0644,
	))

	files, err := DiscoverAgentFiles(root)
	require.NoError(t, err)
	assert.Len(t, files, 1)
}

func TestDiscoverAgentFiles_SkipsGitDir(t *testing.T) {
	root := t.TempDir()
	gitDir := filepath.Join(root, ".git", "agents")
	require.NoError(t, os.MkdirAll(gitDir, 0755))
	require.NoError(t, os.WriteFile(
		filepath.Join(gitDir, "not-an-agent.md"),
		[]byte("---\nname: test\ntools: Read\n---\n# Not\nSkipped."),
		0644,
	))

	files, err := DiscoverAgentFiles(root)
	require.NoError(t, err)
	assert.Empty(t, files, ".git directory should be skipped")
}

func TestDiscoverAgentFiles_SkipsVendor(t *testing.T) {
	root := t.TempDir()
	vendorDir := filepath.Join(root, "vendor", ".claude", "agents")
	require.NoError(t, os.MkdirAll(vendorDir, 0755))
	require.NoError(t, os.WriteFile(
		filepath.Join(vendorDir, "vendored.md"),
		[]byte("---\nname: vendored\ntools: Read\n---\n# Vendor\nBody."),
		0644,
	))

	files, err := DiscoverAgentFiles(root)
	require.NoError(t, err)
	assert.Empty(t, files, "vendor directory should be skipped")
}

func TestDiscoverAgentFiles_ReifySkills(t *testing.T) {
	root := t.TempDir()
	skillsDir := filepath.Join(root, "skills")
	require.NoError(t, os.MkdirAll(skillsDir, 0755))
	require.NoError(t, os.WriteFile(
		filepath.Join(skillsDir, "review.skill.yaml"),
		[]byte("skill: review\ncontext:\n  consumes: [x]\n  produces: [y]\nstrategy:\n  tools: [read]\n  steps:\n    - do"),
		0644,
	))

	files, err := DiscoverAgentFiles(root)
	require.NoError(t, err)
	assert.Len(t, files, 1)
}

// --- Resolve (new in F7) ---

func TestResolve_SingleFile(t *testing.T) {
	root := t.TempDir()
	file := filepath.Join(root, "CLAUDE.md")
	require.NoError(t, os.WriteFile(file, []byte("# Project\nSome content."), 0644))

	got, err := Resolve(file)
	require.NoError(t, err)
	assert.Equal(t, []string{file}, got, "single file should return as-is without format check")
}

func TestResolve_SingleFile_FormatNotChecked(t *testing.T) {
	// Resolve trusts the caller — even a random file is returned. The caller
	// (classify, check, doctor) decides what to do if format detection fails.
	root := t.TempDir()
	file := filepath.Join(root, "random.txt")
	require.NoError(t, os.WriteFile(file, []byte("just text"), 0644))

	got, err := Resolve(file)
	require.NoError(t, err)
	assert.Equal(t, []string{file}, got)
}

func TestResolve_Directory(t *testing.T) {
	root := t.TempDir()
	claudeDir := filepath.Join(root, ".claude", "agents")
	require.NoError(t, os.MkdirAll(claudeDir, 0755))
	require.NoError(t, os.WriteFile(
		filepath.Join(claudeDir, "a.md"),
		[]byte("---\nname: a\n---\n# A"),
		0644,
	))
	require.NoError(t, os.WriteFile(
		filepath.Join(claudeDir, "b.md"),
		[]byte("---\nname: b\n---\n# B"),
		0644,
	))

	got, err := Resolve(root)
	require.NoError(t, err)
	assert.Len(t, got, 2)
}

func TestResolve_RepoRoot_FindsClaudeAndCopilot(t *testing.T) {
	// This is the F7 regression case: a repo with .claude/ (no root CLAUDE.md)
	// must yield agent files. Simulates anthropics/claude-code shape.
	root := t.TempDir()

	claudeDir := filepath.Join(root, ".claude", "agents")
	require.NoError(t, os.MkdirAll(claudeDir, 0755))
	require.NoError(t, os.WriteFile(
		filepath.Join(claudeDir, "reviewer.md"),
		[]byte("---\nname: reviewer\n---\n# Reviewer\nBody."),
		0644,
	))

	copilotDir := filepath.Join(root, ".github", "agents")
	require.NoError(t, os.MkdirAll(copilotDir, 0755))
	require.NoError(t, os.WriteFile(
		filepath.Join(copilotDir, "dash.agent.md"),
		[]byte("---\ndescription: dash\n---\n# Dash"),
		0644,
	))

	// Decoy non-agent files in the root.
	require.NoError(t, os.WriteFile(filepath.Join(root, "README.md"), []byte("# Project"), 0644))
	require.NoError(t, os.WriteFile(filepath.Join(root, "go.mod"), []byte("module x\n"), 0644))

	got, err := Resolve(root)
	require.NoError(t, err)
	assert.Len(t, got, 2, "should find both .claude/ and .github/ agent files from repo root")
}

func TestResolve_MissingPath(t *testing.T) {
	_, err := Resolve(filepath.Join(t.TempDir(), "does-not-exist"))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "cannot access")
}

func TestResolve_EmptyDirectory(t *testing.T) {
	got, err := Resolve(t.TempDir())
	require.NoError(t, err)
	assert.Empty(t, got, "empty directory yields empty slice, not an error")
}
