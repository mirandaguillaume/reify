package doctor

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDiscoverAgentFiles_MixedDirectory(t *testing.T) {
	root := t.TempDir()

	// Claude agent file (should be discovered)
	claudeDir := filepath.Join(root, ".claude", "agents")
	require.NoError(t, os.MkdirAll(claudeDir, 0755))
	require.NoError(t, os.WriteFile(
		filepath.Join(claudeDir, "reviewer.md"),
		[]byte("---\nname: reviewer\ntools: Read\n---\n# Agent\nBody content here."),
		0644,
	))

	// Copilot agent file (should be discovered)
	copilotDir := filepath.Join(root, ".github", "agents")
	require.NoError(t, os.MkdirAll(copilotDir, 0755))
	require.NoError(t, os.WriteFile(
		filepath.Join(copilotDir, "dash.agent.md"),
		[]byte("---\ndescription: dash agent\n---\n# Dash\nBody."),
		0644,
	))

	// Non-agent file (should NOT be discovered)
	require.NoError(t, os.WriteFile(
		filepath.Join(root, "README.md"),
		[]byte("# My Project\nThis is not an agent."),
		0644,
	))

	// Random YAML (should NOT be discovered)
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

	// Nested Claude agents
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

	// .git directory should be skipped entirely
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
