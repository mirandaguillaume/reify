package importer

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mirandaguillaume/reify/pkg/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestWriteImportResult(t *testing.T) {
	dir := t.TempDir()

	result := ImportResult{
		Success: true,
		Skills: []SkillResult{
			{
				Skill: model.SkillBehavior{
					Skill:   "code-review",
					Version: "1.0.0",
					Context: model.ContextFacet{
						Consumes: []string{"source_files"},
						Produces: []string{"review_comments"},
					},
				},
			},
		},
	}

	paths, err := WriteImportResult(result, dir)
	require.NoError(t, err)
	require.Len(t, paths, 1)

	expectedPath := filepath.Join(dir, "skills", "code-review.skill.yaml")
	assert.Equal(t, expectedPath, paths[0])

	data, err := os.ReadFile(expectedPath)
	require.NoError(t, err)
	assert.Contains(t, string(data), "code-review")
}

func TestWriteImportResult_WithAgent(t *testing.T) {
	dir := t.TempDir()

	result := ImportResult{
		Success: true,
		Skills: []SkillResult{
			{
				Skill: model.SkillBehavior{
					Skill:   "analysis",
					Version: "1.0.0",
				},
			},
		},
		Agent: &AgentResult{
			Agent: model.AgentComposition{
				Agent:         "my-agent",
				Skills:        []string{"analysis"},
				Orchestration: model.OrchestrationSequential,
			},
		},
	}

	paths, err := WriteImportResult(result, dir)
	require.NoError(t, err)
	require.Len(t, paths, 2)

	skillPath := filepath.Join(dir, "skills", "analysis.skill.yaml")
	agentPath := filepath.Join(dir, "agents", "my-agent.agent.yaml")

	assert.Equal(t, skillPath, paths[0])
	assert.Equal(t, agentPath, paths[1])

	_, err = os.Stat(skillPath)
	require.NoError(t, err, "skill file should exist")

	_, err = os.Stat(agentPath)
	require.NoError(t, err, "agent file should exist")
}

func TestWriteImportResult_WithContracts(t *testing.T) {
	dir := t.TempDir()

	result := ImportResult{
		Success: true,
		Skills: []SkillResult{
			{
				Skill: model.SkillBehavior{
					Skill:   "reviewer",
					Version: "1.0.0",
					Context: model.ContextFacet{
						Produces: []string{"review_comments"},
					},
				},
			},
		},
		Contracts: map[string]string{
			"review_comments": "Provide review as structured list.",
		},
	}

	paths, err := WriteImportResult(result, dir)
	require.NoError(t, err)
	require.Len(t, paths, 2) // 1 skill + 1 contract

	contractPath := filepath.Join(dir, "contracts", "review_comments.md")
	assert.Contains(t, paths, contractPath)

	data, err := os.ReadFile(contractPath)
	require.NoError(t, err)
	assert.Contains(t, string(data), "structured list")
}

func TestWriteImportResult_ContractConflict(t *testing.T) {
	dir := t.TempDir()

	contractsDir := filepath.Join(dir, "contracts")
	require.NoError(t, os.MkdirAll(contractsDir, 0755))
	require.NoError(t, os.WriteFile(filepath.Join(contractsDir, "existing.md"), []byte("old"), 0644))

	result := ImportResult{
		Success:   true,
		Contracts: map[string]string{"existing": "new content"},
	}

	_, err := WriteImportResult(result, dir)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "already exists")
}

func TestWriteImportResult_ConflictDetection(t *testing.T) {
	dir := t.TempDir()

	skillsDir := filepath.Join(dir, "skills")
	require.NoError(t, os.MkdirAll(skillsDir, 0755))
	conflictPath := filepath.Join(skillsDir, "existing-skill.skill.yaml")
	require.NoError(t, os.WriteFile(conflictPath, []byte("existing content\n"), 0644))

	result := ImportResult{
		Success: true,
		Skills: []SkillResult{
			{
				Skill: model.SkillBehavior{
					Skill:   "existing-skill",
					Version: "1.0.0",
				},
			},
		},
	}

	_, err := WriteImportResult(result, dir)
	require.Error(t, err)
	assert.True(t, strings.Contains(err.Error(), "already exists"), "error should mention 'already exists', got: %s", err.Error())
}
