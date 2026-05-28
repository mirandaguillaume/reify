package cmd

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"
)

func TestDoctorExportYAML_ProducesValidSkillYAML(t *testing.T) {
	// Create a minimal Claude agent file (needs name + another field for format detection)
	dir := t.TempDir()
	agentFile := filepath.Join(dir, "my-reviewer.md")
	content := "---\nname: my-reviewer\ndescription: Reviews code carefully\n---\n## Role\nYou review code.\n"
	require.NoError(t, os.WriteFile(agentFile, []byte(content), 0644))

	cleanupRootCmd(t)
	buf := &bytes.Buffer{}
	rootCmd.SetOut(buf)
	rootCmd.SetArgs([]string{"doctor", agentFile, "--export-yaml"})
	err := rootCmd.Execute()
	require.NoError(t, err)

	out := buf.Bytes()
	assert.Greater(t, len(out), 0, "expected non-empty YAML output")

	// Must be valid YAML
	var m map[string]interface{}
	require.NoError(t, yaml.Unmarshal(out, &m), "output must be valid YAML")
	assert.Contains(t, m, "skill", "exported YAML must have 'skill' key")
	assert.Contains(t, m, "version", "exported YAML must have 'version' key")
}

func TestDoctorExportYAML_NameDerivedFromFilename(t *testing.T) {
	dir := t.TempDir()
	agentFile := filepath.Join(dir, "code-review-agent.md")
	// name + description in frontmatter so Claude parser detects it
	require.NoError(t, os.WriteFile(agentFile, []byte("---\nname: code-review-agent\ndescription: Reviews code\n---\n# Agent\nDo code review.\n"), 0644))

	cleanupRootCmd(t)
	buf := &bytes.Buffer{}
	rootCmd.SetOut(buf)
	rootCmd.SetArgs([]string{"doctor", agentFile, "--export-yaml"})
	require.NoError(t, rootCmd.Execute())

	var m map[string]interface{}
	require.NoError(t, yaml.Unmarshal(buf.Bytes(), &m))
	assert.Equal(t, "code-review-agent", m["skill"])
}

func TestDoctorExportYAML_ErrorOnDirectory(t *testing.T) {
	cleanupRootCmd(t)
	dir := t.TempDir()
	rootCmd.SetArgs([]string{"doctor", dir, "--export-yaml"})
	err := rootCmd.Execute()
	assert.Error(t, err, "--export-yaml on a directory must return an error")
}

func TestDoctorExportYAML_ErrorWithFixFlag(t *testing.T) {
	cleanupRootCmd(t)
	dir := t.TempDir()
	agentFile := filepath.Join(dir, "agent.md")
	require.NoError(t, os.WriteFile(agentFile, []byte("# Agent\n"), 0644))

	rootCmd.SetArgs([]string{"doctor", agentFile, "--export-yaml", "--fix"})
	err := rootCmd.Execute()
	assert.Error(t, err, "--export-yaml combined with --fix must return an error")
}

func TestDoctorExportYAML_ErrorWithUpdateRegistryFlag(t *testing.T) {
	cleanupRootCmd(t)
	dir := t.TempDir()
	agentFile := filepath.Join(dir, "agent.md")
	require.NoError(t, os.WriteFile(agentFile, []byte("---\nname: agent\ndescription: test\n---\n# Agent\n"), 0644))

	rootCmd.SetArgs([]string{"doctor", agentFile, "--export-yaml", "--update-registry"})
	err := rootCmd.Execute()
	assert.Error(t, err, "--export-yaml combined with --update-registry must return an error")
}
