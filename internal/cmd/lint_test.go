package cmd

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/mirandaguillaume/reify/internal/linter"
)

const validSkillYAML = `skill: clean-skill
version: "0.1.0"
context:
  consumes: []
  produces:
    - output
  memory: short-term
strategy:
  tools:
    - Read
  approach: sequential
guardrails:
  - "timeout: 30s"

observability:
  trace_level: minimal
  metrics:
    - latency
security:
  filesystem: none
  network: none
  secrets: []
negotiation:
  file_conflicts: yield
  priority: 0
when_to_use:
  triggers:
    - "testing"
`

const noGuardrailsSkillYAML = `skill: risky-skill
version: "0.1.0"
context:
  consumes: []
  produces:
    - output
  memory: short-term
strategy:
  tools:
    - Read
  approach: sequential
guardrails: []

observability:
  trace_level: minimal
  metrics: []
security:
  filesystem: none
  network: none
  secrets: []
negotiation:
  file_conflicts: yield
  priority: 0
`

const invalidSkillYAML = `skill: ""
version: ""
`

func TestLintDirectory_ValidSkillFiles_NoErrors(t *testing.T) {
	dir := t.TempDir()
	err := os.WriteFile(filepath.Join(dir, "clean.skill.yaml"), []byte(validSkillYAML), 0644)
	require.NoError(t, err)

	result := LintDirectory(dir)

	assert.Equal(t, 1, result.TotalFiles)
	assert.Equal(t, 0, result.Errors)
	assert.Equal(t, 0, result.TotalIssues)
	assert.Empty(t, result.Results)
}

func TestLintDirectory_NoGuardrails_Warnings(t *testing.T) {
	dir := t.TempDir()
	err := os.WriteFile(filepath.Join(dir, "risky.skill.yaml"), []byte(noGuardrailsSkillYAML), 0644)
	require.NoError(t, err)

	result := LintDirectory(dir)

	assert.Equal(t, 1, result.TotalFiles)
	assert.Greater(t, result.Warnings, 0)
	assert.Equal(t, 0, result.Errors)

	issues, ok := result.Results["risky.skill.yaml"]
	assert.True(t, ok)
	// Should have at least the has-guardrails warning
	found := false
	for _, issue := range issues {
		if issue.Rule == "has-guardrails" {
			found = true
			assert.Equal(t, linter.SeverityWarning, issue.Severity)
		}
	}
	assert.True(t, found, "expected has-guardrails warning")
}

func TestLintDirectory_InvalidSkillFile_ParseError(t *testing.T) {
	dir := t.TempDir()
	err := os.WriteFile(filepath.Join(dir, "bad.skill.yaml"), []byte(invalidSkillYAML), 0644)
	require.NoError(t, err)

	result := LintDirectory(dir)

	assert.Equal(t, 1, result.TotalFiles)
	assert.Equal(t, 1, result.Errors)
	assert.Equal(t, 1, result.TotalIssues)

	issues, ok := result.Results["bad.skill.yaml"]
	assert.True(t, ok)
	require.Len(t, issues, 1)
	assert.Equal(t, "valid-schema", issues[0].Rule)
	assert.Equal(t, linter.SeverityError, issues[0].Severity)
}

func TestLintDirectory_EmptyDirectory_ZeroFiles(t *testing.T) {
	dir := t.TempDir()

	result := LintDirectory(dir)

	assert.Equal(t, 0, result.TotalFiles)
	assert.Equal(t, 0, result.TotalIssues)
	assert.Empty(t, result.Results)
}

func TestLintDirectory_NonExistentDirectory_ZeroFiles(t *testing.T) {
	result := LintDirectory("/nonexistent/path/to/skills")

	assert.Equal(t, 0, result.TotalFiles)
	assert.Equal(t, 0, result.TotalIssues)
	assert.Empty(t, result.Results)
}
