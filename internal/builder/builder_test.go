package builder_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mirandaguillaume/reify/internal/builder"
	"github.com/mirandaguillaume/reify/internal/scanner"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const minimalSkillYAML = `skill: code-review
version: "1.0"
context:
  consumes: [source_code]
  produces: [review_report]
  memory: conversation
strategy:
  tools: [read_file, grep]
  approach: analytical
  effort: medium
  steps:
    - read the diff carefully
    - flag issues with file and line numbers
guardrails:
  - never modify source files directly
observability:
  trace_level: standard
  metrics: [tokens]
security:
  filesystem: read-only
  network: none
  secrets: []
negotiation:
  file_conflicts: yield
  priority: 1
`

const minimalAgentYAML = `agent: reviewer
description: "Reviews code changes"
consumes: [git_diff]
produces: [review_report]
orchestration: sequential
skills: [code-review]
`

// setupProject creates skillsDir/agentsDir with one minimal skill and agent.
// Returns (skillsDir, agentsDir, outputDir) all under t.TempDir().
func setupProject(t *testing.T) (string, string, string) {
	t.Helper()
	root := t.TempDir()
	skillsDir := filepath.Join(root, "skills")
	agentsDir := filepath.Join(root, "agents")
	outputDir := filepath.Join(root, "out")

	require.NoError(t, os.MkdirAll(skillsDir, 0755))
	require.NoError(t, os.MkdirAll(agentsDir, 0755))
	require.NoError(t, os.WriteFile(filepath.Join(skillsDir, "code-review.skill.yaml"), []byte(minimalSkillYAML), 0644))
	require.NoError(t, os.WriteFile(filepath.Join(agentsDir, "reviewer.agent.yaml"), []byte(minimalAgentYAML), 0644))

	return skillsDir, agentsDir, outputDir
}

func TestGetOutputDir_UsesOverride(t *testing.T) {
	assert.Equal(t, "/custom/dir", builder.GetOutputDir("claude", "/custom/dir"))
}

func TestGetOutputDir_FallsBackToGeneratorDefault(t *testing.T) {
	assert.Equal(t, ".claude", builder.GetOutputDir("claude", ""))
	assert.Equal(t, ".github", builder.GetOutputDir("copilot", ""))
	assert.Equal(t, ".cursor", builder.GetOutputDir("cursor", ""))
}

func TestGetOutputDir_UnknownTargetFallsBackToClaude(t *testing.T) {
	assert.Equal(t, ".claude", builder.GetOutputDir("nonexistent", ""))
}

func TestRunBuild_UnknownTargetReturnsError(t *testing.T) {
	r := builder.RunBuild("", "", "", "no-such-target", scanner.EnrichNone)
	assert.False(t, r.Success)
	assert.Contains(t, r.Error, "unknown build target")
}

func TestRunBuild_ProducesSkillAndAgentForClaude(t *testing.T) {
	skillsDir, agentsDir, outputDir := setupProject(t)

	r := builder.RunBuild(skillsDir, agentsDir, outputDir, "claude", scanner.EnrichNone)

	require.True(t, r.Success, "build failed: %s", r.Error)
	assert.Equal(t, 1, r.SkillsGenerated)
	assert.Equal(t, 1, r.AgentsGenerated)
	assert.Equal(t, "claude", r.Target)
	assert.Equal(t, outputDir, r.OutputDir)

	// Claude paths: skills/<name>/SKILL.md, agents/<name>.md, CLAUDE.md
	assert.FileExists(t, filepath.Join(outputDir, "skills", "code-review", "SKILL.md"))
	assert.FileExists(t, filepath.Join(outputDir, "agents", "reviewer.md"))
	assert.FileExists(t, filepath.Join(outputDir, "CLAUDE.md"))
}

func TestRunBuild_ProducesCorrectPathsForCopilot(t *testing.T) {
	skillsDir, agentsDir, outputDir := setupProject(t)

	r := builder.RunBuild(skillsDir, agentsDir, outputDir, "copilot", scanner.EnrichNone)

	require.True(t, r.Success, "build failed: %s", r.Error)
	assert.FileExists(t, filepath.Join(outputDir, "copilot-instructions.md"))
}

func TestRunBuild_ProducesCorrectPathsForCursor(t *testing.T) {
	skillsDir, agentsDir, outputDir := setupProject(t)

	r := builder.RunBuild(skillsDir, agentsDir, outputDir, "cursor", scanner.EnrichNone)

	require.True(t, r.Success, "build failed: %s", r.Error)
	// Cursor skill path: rules/<name>.mdc
	assert.FileExists(t, filepath.Join(outputDir, "rules", "code-review.mdc"))
	// Cursor instructions live above the output dir: ../.cursorrules
	assert.FileExists(t, filepath.Join(filepath.Dir(outputDir), ".cursorrules"))
}

func TestRunBuild_AgentReferencesMissingSkillWarns(t *testing.T) {
	skillsDir, agentsDir, outputDir := setupProject(t)
	// Add an agent that references a non-existent skill.
	brokenAgent := `agent: broken
description: "Has missing skill"
skills: [does-not-exist]
orchestration: sequential
`
	require.NoError(t, os.WriteFile(filepath.Join(agentsDir, "broken.agent.yaml"), []byte(brokenAgent), 0644))

	r := builder.RunBuild(skillsDir, agentsDir, outputDir, "claude", scanner.EnrichNone)

	require.True(t, r.Success)
	found := false
	for _, w := range r.Warnings {
		if strings.Contains(w, "unresolved skills") && strings.Contains(w, "does-not-exist") {
			found = true
			break
		}
	}
	assert.True(t, found, "expected warning about unresolved skill, got: %v", r.Warnings)
}

func TestRunBuild_MissingSkillsDirIsTolerated(t *testing.T) {
	root := t.TempDir()
	outputDir := filepath.Join(root, "out")

	// Both dirs absent: build still succeeds (zero skills/agents generated).
	r := builder.RunBuild(filepath.Join(root, "nope"), filepath.Join(root, "nope2"), outputDir, "claude", scanner.EnrichNone)

	assert.True(t, r.Success, "build should tolerate missing dirs, got error: %s", r.Error)
	assert.Equal(t, 0, r.SkillsGenerated)
	assert.Equal(t, 0, r.AgentsGenerated)
}

func TestRunBuild_InvalidSkillYAMLReturnsError(t *testing.T) {
	root := t.TempDir()
	skillsDir := filepath.Join(root, "skills")
	require.NoError(t, os.MkdirAll(skillsDir, 0755))
	require.NoError(t, os.WriteFile(filepath.Join(skillsDir, "bad.skill.yaml"), []byte("not: valid: yaml: at: all:"), 0644))

	r := builder.RunBuild(skillsDir, "", filepath.Join(root, "out"), "claude", scanner.EnrichNone)

	assert.False(t, r.Success)
	assert.Contains(t, r.Error, "Parse error")
}

func TestRunBuild_IgnoresNonYAMLFiles(t *testing.T) {
	skillsDir, agentsDir, outputDir := setupProject(t)
	// Add noise files that should be skipped.
	require.NoError(t, os.WriteFile(filepath.Join(skillsDir, "README.md"), []byte("not a skill"), 0644))
	require.NoError(t, os.WriteFile(filepath.Join(skillsDir, "data.yaml"), []byte("missing .skill suffix"), 0644))
	require.NoError(t, os.MkdirAll(filepath.Join(skillsDir, "subdir"), 0755))

	r := builder.RunBuild(skillsDir, agentsDir, outputDir, "claude", scanner.EnrichNone)

	require.True(t, r.Success, r.Error)
	assert.Equal(t, 1, r.SkillsGenerated, "only *.skill.yaml files should be processed")
}

func TestRunBuild_CompactSkipsSkillFiles(t *testing.T) {
	skillsDir, agentsDir, outputDir := setupProject(t)

	// Compact mode inlines skills into the agent file → no individual skill files written.
	r := builder.RunBuild(skillsDir, agentsDir, outputDir, "claude", scanner.EnrichNone)
	require.True(t, r.Success)
	defaultCount := r.SkillsGenerated

	// Re-run with compact via the options API.
	skillsDir2, agentsDir2, outputDir2 := setupProject(t)
	r2 := builder.RunBuildWithOptions(skillsDir2, agentsDir2, outputDir2, "claude", scanner.EnrichNone, true)
	require.True(t, r2.Success, r2.Error)
	assert.Equal(t, 1, defaultCount)
	assert.Equal(t, 0, r2.SkillsGenerated, "compact mode should not emit per-skill files")
	assert.NoFileExists(t, filepath.Join(outputDir2, "skills", "code-review", "SKILL.md"))
}

func TestRunBuild_BuildResultFieldsPopulated(t *testing.T) {
	skillsDir, agentsDir, outputDir := setupProject(t)
	r := builder.RunBuild(skillsDir, agentsDir, outputDir, "claude", scanner.EnrichNone)

	assert.True(t, r.Success)
	assert.Empty(t, r.Error)
	assert.Equal(t, "claude", r.Target)
	assert.Equal(t, outputDir, r.OutputDir)
}
