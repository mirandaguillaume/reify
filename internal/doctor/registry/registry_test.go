package registry

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLoad_Embedded(t *testing.T) {
	reg, err := Load("")
	require.NoError(t, err)
	assert.NotNil(t, reg)
	assert.NotEmpty(t, reg.Version)
}

func TestAll_Returns24Entries(t *testing.T) {
	reg, err := Load("")
	require.NoError(t, err)

	entries := reg.All()
	assert.Len(t, entries, 24, "registry must have exactly 12 entries")
}

func TestAll_EntriesHaveRequiredFields(t *testing.T) {
	reg, err := Load("")
	require.NoError(t, err)

	for _, e := range reg.All() {
		assert.NotEmpty(t, e.ID, "entry missing id")
		assert.NotEmpty(t, e.Title, "entry %s missing title", e.ID)
		assert.NotEmpty(t, e.Citation, "entry %s missing citation", e.ID)
		assert.NotEmpty(t, e.Paper, "entry %s missing paper", e.ID)
		assert.NotEmpty(t, e.URL, "entry %s missing url", e.ID)
		assert.NotEmpty(t, e.Finding, "entry %s missing finding", e.ID)
		assert.NotEmpty(t, e.Confidence, "entry %s missing confidence", e.ID)
		assert.NotEmpty(t, e.DetectionPrompt, "entry %s missing detection_prompt", e.ID)
		assert.NotEmpty(t, e.SuggestionPrompt, "entry %s missing suggestion_prompt", e.ID)
	}
}

func TestAll_ExpectedIDs(t *testing.T) {
	reg, err := Load("")
	require.NoError(t, err)

	expectedIDs := []string{
		"guardrails", "security", "ordering", "decomposition",
		"context", "redundancy", "tool_usage", "specificity",
		"testing", "examples", "error_handling", "scope",
	}

	entries := reg.All()
	var ids []string
	for _, e := range entries {
		ids = append(ids, e.ID)
	}

	for _, expected := range expectedIDs {
		assert.Contains(t, ids, expected, "missing expected entry: %s", expected)
	}
}

func TestGet_ExistingEntry(t *testing.T) {
	reg, err := Load("")
	require.NoError(t, err)

	entry, ok := reg.Get("guardrails")
	assert.True(t, ok)
	assert.Equal(t, "guardrails", entry.ID)
	assert.Contains(t, entry.Citation, "Liu")
}

func TestGet_NonExistent(t *testing.T) {
	reg, err := Load("")
	require.NoError(t, err)

	_, ok := reg.Get("nonexistent")
	assert.False(t, ok)
}

func TestLoad_LocalOverride(t *testing.T) {
	// Create a temp dir with .reify/research-registry.yaml
	tmpDir := t.TempDir()
	reifyDir := filepath.Join(tmpDir, ".reify")
	require.NoError(t, os.MkdirAll(reifyDir, 0o755))

	overrideYAML := `version: "test-override"
recommendations:
  - id: custom_check
    title: "Custom check"
    citation: "Test, 2026"
    paper: "Test Paper"
    url: "https://example.com"
    finding: "Custom finding"
    confidence: high
    detection_prompt: "Is there a custom check?"
    suggestion_prompt: "Add a custom check."
`
	require.NoError(t, os.WriteFile(
		filepath.Join(reifyDir, "research-registry.yaml"),
		[]byte(overrideYAML),
		0o644,
	))

	reg, err := Load(tmpDir)
	require.NoError(t, err)
	assert.Equal(t, "test-override", reg.Version)

	entries := reg.All()
	assert.Len(t, entries, 1)
	assert.Equal(t, "custom_check", entries[0].ID)
}

func TestLoad_LocalOverride_InvalidYAML(t *testing.T) {
	tmpDir := t.TempDir()
	reifyDir := filepath.Join(tmpDir, ".reify")
	require.NoError(t, os.MkdirAll(reifyDir, 0o755))

	require.NoError(t, os.WriteFile(
		filepath.Join(reifyDir, "research-registry.yaml"),
		[]byte("{{invalid yaml"),
		0o644,
	))

	_, err := Load(tmpDir)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "parse local registry")
}

func TestLoad_NoOverride_UsesEmbedded(t *testing.T) {
	// Use a temp dir with no .reify/ — should fall back to embedded
	tmpDir := t.TempDir()

	reg, err := Load(tmpDir)
	require.NoError(t, err)
	assert.Len(t, reg.All(), 24)
}

func TestVersion(t *testing.T) {
	reg, err := Load("")
	require.NoError(t, err)
	assert.Equal(t, "2026.03.2", reg.Version)
}

func TestSource_Embedded(t *testing.T) {
	tmpDir := t.TempDir()
	reg, err := Load(tmpDir)
	require.NoError(t, err)
	assert.Equal(t, "embedded", reg.Source)
}

func TestSource_LocalOverride(t *testing.T) {
	tmpDir := t.TempDir()
	reifyDir := filepath.Join(tmpDir, ".reify")
	require.NoError(t, os.MkdirAll(reifyDir, 0o755))

	overrideYAML := `version: "local"
recommendations:
  - id: test
    title: "Test"
    citation: "Test"
    paper: "Test"
    url: "https://example.com"
    finding: "Test"
    confidence: high
    detection_prompt: "Test?"
    suggestion_prompt: "Test."
`
	require.NoError(t, os.WriteFile(
		filepath.Join(reifyDir, "research-registry.yaml"),
		[]byte(overrideYAML),
		0o644,
	))

	reg, err := Load(tmpDir)
	require.NoError(t, err)
	assert.Equal(t, "local", reg.Source)
	assert.Contains(t, reg.SourcePath, ".reify/research-registry.yaml")
}

func TestConfidenceLevels(t *testing.T) {
	reg, err := Load("")
	require.NoError(t, err)

	validConfidences := map[string]bool{"high": true, "moderate": true, "low": true}
	for _, e := range reg.All() {
		assert.True(t, validConfidences[e.Confidence],
			"entry %s has invalid confidence %q", e.ID, e.Confidence)
	}
}
