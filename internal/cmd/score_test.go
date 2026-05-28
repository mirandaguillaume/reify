package cmd

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const scoreValidSkillYAML = `skill: well-designed
version: "0.1.0"
context:
  consumes:
    - user-input
  produces:
    - analysis
  memory: conversation
strategy:
  tools:
    - Read
    - Write
  approach: sequential
  steps:
    - parse input
    - analyze content
    - produce output
guardrails:
  - "timeout: 30s"
  - "max_tokens: 4096"

observability:
  trace_level: detailed
  metrics:
    - latency
    - accuracy
security:
  filesystem: read-only
  network: none
  secrets: []
  sandbox: container
negotiation:
  file_conflicts: yield
  priority: 0
`

const scoreAgentYAML = `agent: analysis-pipeline
skills:
  - well-designed
orchestration: sequential
description: "A pipeline that analyzes user input and produces structured analysis output"
`

func TestRunScore_ValidSkills_ScoresGreaterThanZero(t *testing.T) {
	dir := t.TempDir()
	err := os.WriteFile(filepath.Join(dir, "well-designed.skill.yaml"), []byte(scoreValidSkillYAML), 0644)
	require.NoError(t, err)

	report := RunScore(dir, "")

	require.Len(t, report.Skills, 1)
	assert.Greater(t, report.Skills[0].Total, 0)
	assert.Equal(t, "well-designed", report.Skills[0].Skill)

	// Verify breakdown has non-zero values
	b := report.Skills[0].Breakdown
	assert.Greater(t, b.Context, 0)
	assert.Greater(t, b.Strategy, 0)
	assert.Greater(t, b.Guardrails, 0)
	assert.Greater(t, b.Observability, 0)
	assert.Greater(t, b.Security, 0)
}

func TestRunScore_EmptyDirectory_EmptyReport(t *testing.T) {
	dir := t.TempDir()

	report := RunScore(dir, "")

	assert.Empty(t, report.Skills)
	assert.Empty(t, report.Agents)
}

func TestRunScore_NonExistentDirectory_EmptyReport(t *testing.T) {
	report := RunScore("/nonexistent/path", "")

	assert.Empty(t, report.Skills)
	assert.Empty(t, report.Agents)
}

func TestRunScore_WithAgents_AgentScores(t *testing.T) {
	skillsDir := t.TempDir()
	agentsDir := t.TempDir()

	require.NoError(t, os.WriteFile(filepath.Join(skillsDir, "well-designed.skill.yaml"), []byte(scoreValidSkillYAML), 0644))
	require.NoError(t, os.WriteFile(filepath.Join(agentsDir, "analysis-pipeline.agent.yaml"), []byte(scoreAgentYAML), 0644))

	report := RunScore(skillsDir, agentsDir)

	require.Len(t, report.Skills, 1)
	require.Len(t, report.Agents, 1)
	assert.Greater(t, report.Agents[0].Total, 0)
	assert.Equal(t, "analysis-pipeline", report.Agents[0].Agent)
}

func TestRunScore_InvalidSkillFile_Skipped(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "bad.skill.yaml"), []byte(`skill: ""`), 0644))

	report := RunScore(dir, "")

	assert.Empty(t, report.Skills, "invalid skill files should be skipped")
}
