package doctor

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	yamlloader "github.com/mirandaguillaume/reify/internal/yaml"
)

func TestSkillSpecs_NonEmpty(t *testing.T) {
	specs, err := SkillSpecs()
	require.NoError(t, err)
	assert.Len(t, specs, 4, "expected 4 embedded doctor skill specs")

	expected := []string{"format-detector", "analyzer", "context-enricher", "recommendation-builder"}
	for _, name := range expected {
		data, ok := specs[name]
		require.True(t, ok, "missing embedded skill: %s", name)
		assert.NotEmpty(t, data, "embedded skill %s is empty", name)
	}
}

func TestSkillSpecs_Parseable(t *testing.T) {
	specs, err := SkillSpecs()
	require.NoError(t, err)

	for name, data := range specs {
		skill, err := yamlloader.ParseSkillYAML(string(data))
		require.NoError(t, err, "skill %s failed to parse", name)
		assert.Equal(t, name, skill.Skill, "parsed skill name mismatch for %s", name)
	}
}

func TestAgentSpec_NonEmpty(t *testing.T) {
	data := AgentSpec()
	assert.NotEmpty(t, data, "embedded agent spec is empty")
}

func TestAgentSpec_Parseable(t *testing.T) {
	data := AgentSpec()
	agent, err := yamlloader.ParseAgentYAML(string(data))
	require.NoError(t, err)
	assert.Equal(t, "doctor", agent.Agent)
	assert.Len(t, agent.Skills, 4)
}

// TestEmbedSync verifies that the embedded copies match the canonical YAML files.
func TestEmbedSync(t *testing.T) {
	canonicalSkillDir := filepath.Join("..", "..", "skills", "doctor")
	canonicalAgentFile := filepath.Join("..", "..", "agents", "doctor.agent.yaml")

	specs, err := SkillSpecs()
	require.NoError(t, err)

	for name, embedded := range specs {
		canonical, err := os.ReadFile(filepath.Join(canonicalSkillDir, name+".skill.yaml"))
		if err != nil {
			t.Skipf("canonical file not accessible (expected in CI): %v", err)
			return
		}
		assert.Equal(t, string(canonical), string(embedded),
			"embedded %s.skill.yaml differs from canonical skills/doctor/%s.skill.yaml — run: cp skills/doctor/*.skill.yaml internal/doctor/specs/", name, name)
	}

	canonicalAgent, err := os.ReadFile(canonicalAgentFile)
	if err != nil {
		t.Skipf("canonical agent file not accessible: %v", err)
		return
	}
	assert.Equal(t, string(canonicalAgent), string(AgentSpec()),
		"embedded doctor.agent.yaml differs from canonical agents/doctor.agent.yaml — run: cp agents/doctor.agent.yaml internal/doctor/specs/")
}
