package cmd

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"

	"github.com/mirandaguillaume/reify/pkg/model"
)

func TestCreateSkill_Success(t *testing.T) {
	dir := t.TempDir()

	result := CreateSkill(dir, "my-skill", CreateSkillOptions{})

	assert.True(t, result.Success)
	assert.Equal(t, filepath.Join(dir, "skills", "my-skill.skill.yaml"), result.Path)
	assert.Empty(t, result.Error)

	// File should exist
	_, err := os.Stat(result.Path)
	require.NoError(t, err)

	// File should contain valid YAML with the skill name
	data, err := os.ReadFile(result.Path)
	require.NoError(t, err)

	var skill model.SkillBehavior
	err = yaml.Unmarshal(data, &skill)
	require.NoError(t, err)
	assert.Equal(t, "my-skill", skill.Skill)
	assert.Equal(t, "0.1.0", skill.Version)
	assert.Equal(t, model.MemoryShortTerm, skill.Context.Memory)
}

func TestCreateSkill_InvalidName_Uppercase(t *testing.T) {
	dir := t.TempDir()

	result := CreateSkill(dir, "MySkill", CreateSkillOptions{})

	assert.False(t, result.Success)
	assert.Contains(t, result.Error, "Invalid skill name")
}

func TestCreateSkill_InvalidName_Spaces(t *testing.T) {
	dir := t.TempDir()

	result := CreateSkill(dir, "my skill", CreateSkillOptions{})

	assert.False(t, result.Success)
	assert.Contains(t, result.Error, "Invalid skill name")
}

func TestCreateSkill_InvalidName_StartWithHyphen(t *testing.T) {
	dir := t.TempDir()

	result := CreateSkill(dir, "-invalid", CreateSkillOptions{})

	assert.False(t, result.Success)
	assert.Contains(t, result.Error, "Invalid skill name")
}

func TestCreateSkill_AlreadyExists(t *testing.T) {
	dir := t.TempDir()

	// Create the skill first
	skillDir := filepath.Join(dir, "skills")
	os.MkdirAll(skillDir, 0755)
	os.WriteFile(filepath.Join(skillDir, "existing.skill.yaml"), []byte("skill: existing\n"), 0644)

	result := CreateSkill(dir, "existing", CreateSkillOptions{})

	assert.False(t, result.Success)
	assert.Contains(t, result.Error, "already exists")
}

func TestCreateSkill_WithOptions(t *testing.T) {
	dir := t.TempDir()

	result := CreateSkill(dir, "tool-user", CreateSkillOptions{
		Tools:  []string{"Read", "Write", "Bash"},
		Memory: "long-term",
	})

	assert.True(t, result.Success)

	data, err := os.ReadFile(result.Path)
	require.NoError(t, err)

	var skill model.SkillBehavior
	err = yaml.Unmarshal(data, &skill)
	require.NoError(t, err)

	assert.Equal(t, "tool-user", skill.Skill)
	assert.Equal(t, []string{"Read", "Write", "Bash"}, skill.Strategy.Tools)
	assert.Equal(t, model.MemoryLongTerm, skill.Context.Memory)
}

func TestCreateSkill_ValidNames(t *testing.T) {
	cases := []string{"my-skill", "skill1", "a", "test_skill", "code-review"}
	for _, name := range cases {
		t.Run(name, func(t *testing.T) {
			dir := t.TempDir()
			result := CreateSkill(dir, name, CreateSkillOptions{})
			assert.True(t, result.Success, "name %q should be valid", name)
		})
	}
}
