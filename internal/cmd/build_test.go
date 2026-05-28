package cmd_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/mirandaguillaume/reify/internal/cmd"
	_ "github.com/mirandaguillaume/reify/internal/generator/claude"
	_ "github.com/mirandaguillaume/reify/internal/generator/copilot"
	"github.com/mirandaguillaume/reify/internal/scanner"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const testSkillYAML = `skill: test-skill
version: "1.0.0"
context:
  consumes: [input]
  produces: [output]
  memory: short-term
strategy:
  tools: [read_file, grep]
  approach: sequential
guardrails:
  - "timeout: 60s"

observability:
  trace_level: standard
  metrics: [duration]
security:
  filesystem: read-only
  network: none
  secrets: []
negotiation:
  file_conflicts: yield
  priority: 0
`

const testAgentYAML = `agent: test-agent
skills: [test-skill]
orchestration: sequential
description: "Test agent"
`

func TestRunBuild_Claude(t *testing.T) {
	skillsDir := t.TempDir()
	agentsDir := t.TempDir()
	outputDir := t.TempDir()

	os.WriteFile(filepath.Join(skillsDir, "test-skill.skill.yaml"), []byte(testSkillYAML), 0644)
	os.WriteFile(filepath.Join(agentsDir, "test-agent.agent.yaml"), []byte(testAgentYAML), 0644)

	result := cmd.RunBuild(skillsDir, agentsDir, outputDir, "claude", scanner.EnrichNone)
	assert.True(t, result.Success)
	assert.Equal(t, 1, result.SkillsGenerated)
	assert.Equal(t, 1, result.AgentsGenerated)
	assert.FileExists(t, filepath.Join(outputDir, "skills", "test-skill", "SKILL.md"))
	assert.FileExists(t, filepath.Join(outputDir, "agents", "test-agent.md"))
}

func TestRunBuild_Copilot(t *testing.T) {
	skillsDir := t.TempDir()
	agentsDir := t.TempDir()
	outputDir := t.TempDir()

	os.WriteFile(filepath.Join(skillsDir, "test-skill.skill.yaml"), []byte(testSkillYAML), 0644)
	os.WriteFile(filepath.Join(agentsDir, "test-agent.agent.yaml"), []byte(testAgentYAML), 0644)

	result := cmd.RunBuild(skillsDir, agentsDir, outputDir, "copilot", scanner.EnrichNone)
	assert.True(t, result.Success)
	assert.Equal(t, "copilot", result.Target)
	assert.FileExists(t, filepath.Join(outputDir, "skills", "test-skill", "SKILL.md"))
	assert.FileExists(t, filepath.Join(outputDir, "agents", "test-agent.agent.md"))
	assert.FileExists(t, filepath.Join(outputDir, "copilot-instructions.md"))
}

func TestRunBuild_UnknownTarget(t *testing.T) {
	result := cmd.RunBuild(".", ".", ".", "unknown", scanner.EnrichNone)
	assert.False(t, result.Success)
	assert.Contains(t, result.Error, "unknown build target")
}

func TestRunBuild_EmptyDirs(t *testing.T) {
	result := cmd.RunBuild(t.TempDir(), t.TempDir(), t.TempDir(), "claude", scanner.EnrichNone)
	assert.True(t, result.Success)
	assert.Equal(t, 0, result.SkillsGenerated)
}

func TestGetOutputDir(t *testing.T) {
	assert.Equal(t, ".claude", cmd.GetOutputDir("claude", ""))
	assert.Equal(t, ".github", cmd.GetOutputDir("copilot", ""))
	assert.Equal(t, "custom", cmd.GetOutputDir("claude", "custom"))
}

// setupEnrichProject creates a temp project with a Go module, skill, and source files
// so that the scanner has something to scan.
func setupEnrichProject(t *testing.T) (skillsDir, agentsDir, outputDir string) {
	t.Helper()
	root := t.TempDir()

	skillsDir = filepath.Join(root, "skills")
	agentsDir = filepath.Join(root, "agents")
	outputDir = filepath.Join(root, "output")
	require.NoError(t, os.MkdirAll(skillsDir, 0755))
	require.NoError(t, os.MkdirAll(agentsDir, 0755))

	os.WriteFile(filepath.Join(skillsDir, "test-skill.skill.yaml"), []byte(testSkillYAML), 0644)

	// Create a go.mod and source files so the scanner detects a Go project.
	os.WriteFile(filepath.Join(root, "go.mod"), []byte("module example.com/test\n\ngo 1.22\n"), 0644)
	require.NoError(t, os.MkdirAll(filepath.Join(root, "cmd"), 0755))
	os.WriteFile(filepath.Join(root, "cmd/main.go"), []byte("package main\n"), 0644)
	require.NoError(t, os.MkdirAll(filepath.Join(root, "pkg/model"), 0755))
	os.WriteFile(filepath.Join(root, "pkg/model/types.go"), []byte("package model\n"), 0644)

	// RunBuild scans from cwd, so we need to chdir.
	oldDir, _ := os.Getwd()
	require.NoError(t, os.Chdir(root))
	t.Cleanup(func() { os.Chdir(oldDir) })

	return skillsDir, agentsDir, outputDir
}

func TestRunBuild_EnrichIndex(t *testing.T) {
	skillsDir, agentsDir, outputDir := setupEnrichProject(t)

	result := cmd.RunBuild(skillsDir, agentsDir, outputDir, "claude", scanner.EnrichIndex)
	assert.True(t, result.Success)
	assert.Equal(t, 1, result.SkillsGenerated)

	// Context files should exist.
	assert.FileExists(t, filepath.Join(outputDir, "context", "index.md"))
	assert.FileExists(t, filepath.Join(outputDir, "context", "stack.md"))

	// index.md should have pipe-delimited entries.
	indexData, err := os.ReadFile(filepath.Join(outputDir, "context", "index.md"))
	require.NoError(t, err)
	assert.Contains(t, string(indexData), "# Codebase Index")
	assert.Contains(t, string(indexData), "|cmd:{main.go}")

	// stack.md should have language info.
	stackData, err := os.ReadFile(filepath.Join(outputDir, "context", "stack.md"))
	require.NoError(t, err)
	assert.Contains(t, string(stackData), "Go")

	// SKILL.md should have the pointer appended.
	skillData, err := os.ReadFile(filepath.Join(outputDir, "skills", "test-skill", "SKILL.md"))
	require.NoError(t, err)
	assert.Contains(t, string(skillData), "## Codebase Context")
	assert.Contains(t, string(skillData), "context/index.md")
	assert.Contains(t, string(skillData), "context/stack.md")
}

func TestRunBuild_EnrichFull(t *testing.T) {
	skillsDir, agentsDir, outputDir := setupEnrichProject(t)

	result := cmd.RunBuild(skillsDir, agentsDir, outputDir, "claude", scanner.EnrichFull)
	assert.True(t, result.Success)

	// No context/ directory in full mode.
	_, err := os.Stat(filepath.Join(outputDir, "context"))
	assert.True(t, os.IsNotExist(err), "context dir should not exist in full mode")

	// SKILL.md should have inline content.
	skillData, err := os.ReadFile(filepath.Join(outputDir, "skills", "test-skill", "SKILL.md"))
	require.NoError(t, err)
	assert.Contains(t, string(skillData), "## Codebase Context")
	assert.Contains(t, string(skillData), "### Project Structure")
	assert.Contains(t, string(skillData), "### Stack")
	assert.Contains(t, string(skillData), "Go")
}

func TestRunBuild_NoEnrich(t *testing.T) {
	skillsDir, agentsDir, outputDir := setupEnrichProject(t)

	result := cmd.RunBuild(skillsDir, agentsDir, outputDir, "claude", scanner.EnrichNone)
	assert.True(t, result.Success)

	// No context/ directory.
	_, err := os.Stat(filepath.Join(outputDir, "context"))
	assert.True(t, os.IsNotExist(err), "context dir should not exist without enrich")

	// SKILL.md should NOT have codebase context.
	skillData, err := os.ReadFile(filepath.Join(outputDir, "skills", "test-skill", "SKILL.md"))
	require.NoError(t, err)
	assert.NotContains(t, string(skillData), "## Codebase Context")
}

// Skill YAML that declares codebase_index in consumes (codebase-navigation skill).
const testSkillWithIndexYAML = `skill: nav-skill
version: "1.0.0"
context:
  consumes: [codebase_index]
  produces: [codebase_navigation]
  memory: short-term
strategy:
  tools: [read_file, grep]
  approach: sequential
guardrails:
  - "timeout: 60s"

observability:
  trace_level: standard
  metrics: [duration]
security:
  filesystem: read-only
  network: none
  secrets: []
negotiation:
  file_conflicts: yield
  priority: 0
`

// Skill YAML that does NOT declare codebase_index.
const testSkillWithoutIndexYAML = `skill: lint-skill
version: "1.0.0"
context:
  consumes: [source_code]
  produces: [lint_results]
  memory: short-term
strategy:
  tools: [read_file]
  approach: sequential
guardrails:
  - "timeout: 30s"

observability:
  trace_level: minimal
  metrics: []
security:
  filesystem: read-only
  network: none
  secrets: []
negotiation:
  file_conflicts: yield
  priority: 0
`

func TestRunBuild_AutoEnrichViaConsumes(t *testing.T) {
	root := t.TempDir()
	skillsDir := filepath.Join(root, "skills")
	agentsDir := filepath.Join(root, "agents")
	outputDir := filepath.Join(root, "output")
	require.NoError(t, os.MkdirAll(skillsDir, 0755))
	require.NoError(t, os.MkdirAll(agentsDir, 0755))

	// Two skills: one wants codebase_index, one doesn't.
	os.WriteFile(filepath.Join(skillsDir, "nav-skill.skill.yaml"), []byte(testSkillWithIndexYAML), 0644)
	os.WriteFile(filepath.Join(skillsDir, "lint-skill.skill.yaml"), []byte(testSkillWithoutIndexYAML), 0644)

	// Create minimal Go project for scanner.
	os.WriteFile(filepath.Join(root, "go.mod"), []byte("module example.com/test\n\ngo 1.22\n"), 0644)
	require.NoError(t, os.MkdirAll(filepath.Join(root, "cmd"), 0755))
	os.WriteFile(filepath.Join(root, "cmd/main.go"), []byte("package main\n"), 0644)
	require.NoError(t, os.MkdirAll(filepath.Join(root, "pkg/model"), 0755))
	os.WriteFile(filepath.Join(root, "pkg/model/types.go"), []byte("package model\n"), 0644)

	oldDir, _ := os.Getwd()
	require.NoError(t, os.Chdir(root))
	t.Cleanup(func() { os.Chdir(oldDir) })

	// No --enrich flag (EnrichNone) — auto-detection via consumes.
	result := cmd.RunBuild(skillsDir, agentsDir, outputDir, "claude", scanner.EnrichNone)
	assert.True(t, result.Success)
	assert.Equal(t, 2, result.SkillsGenerated)

	// Context files should be generated (because nav-skill needs them).
	assert.FileExists(t, filepath.Join(outputDir, "context", "index.md"))
	assert.FileExists(t, filepath.Join(outputDir, "context", "stack.md"))

	// nav-skill SHOULD have the codebase context pointer.
	navData, err := os.ReadFile(filepath.Join(outputDir, "skills", "nav-skill", "SKILL.md"))
	require.NoError(t, err)
	assert.Contains(t, string(navData), "## Codebase Context")
	assert.Contains(t, string(navData), "context/index.md")

	// lint-skill should NOT have the codebase context pointer.
	lintData, err := os.ReadFile(filepath.Join(outputDir, "skills", "lint-skill", "SKILL.md"))
	require.NoError(t, err)
	assert.NotContains(t, string(lintData), "## Codebase Context")
}
