package export

import (
	"strings"
	"testing"

	"github.com/mirandaguillaume/reify/internal/doctor/parser"
	"github.com/mirandaguillaume/reify/pkg/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"
)

func TestToSkillYAML_NameFromFrontmatter(t *testing.T) {
	analysis := &parser.AgentAnalysis{
		Format:      "claude",
		Frontmatter: map[string]interface{}{"name": "my-reviewer"},
	}
	data, err := ToSkillYAML(analysis, "agent.md")
	require.NoError(t, err)

	var skill model.SkillBehavior
	require.NoError(t, yaml.Unmarshal(data, &skill))
	assert.Equal(t, "my-reviewer", skill.Skill)
}

func TestToSkillYAML_NameFallbackToFilename(t *testing.T) {
	analysis := &parser.AgentAnalysis{
		Format:      "claude",
		Frontmatter: map[string]interface{}{},
	}
	data, err := ToSkillYAML(analysis, "/some/path/code_reviewer.md")
	require.NoError(t, err)

	var skill model.SkillBehavior
	require.NoError(t, yaml.Unmarshal(data, &skill))
	assert.Equal(t, "code-reviewer", skill.Skill)
}

func TestToSkillYAML_DescriptionFromFrontmatter(t *testing.T) {
	analysis := &parser.AgentAnalysis{
		Format:      "claude",
		Frontmatter: map[string]interface{}{"description": "Reviews PRs carefully"},
	}
	data, err := ToSkillYAML(analysis, "agent.md")
	require.NoError(t, err)

	var skill model.SkillBehavior
	require.NoError(t, yaml.Unmarshal(data, &skill))
	assert.Equal(t, "Reviews PRs carefully", skill.Strategy.Steps[0])
}

func TestToSkillYAML_DescriptionFallbackToFirstSection(t *testing.T) {
	analysis := &parser.AgentAnalysis{
		Format:      "claude",
		Frontmatter: map[string]interface{}{},
		Sections:    []parser.Section{{Header: "## Role", Content: "I am a helpful assistant", Level: 2}},
	}
	data, err := ToSkillYAML(analysis, "agent.md")
	require.NoError(t, err)

	var skill model.SkillBehavior
	require.NoError(t, yaml.Unmarshal(data, &skill))
	assert.True(t, len(skill.Strategy.Steps) > 0, "expected strategy.steps to include section content")
	assert.Contains(t, skill.Strategy.Steps[0], "I am a helpful assistant")
}

func TestToSkillYAML_ValidYAMLPassesValidation(t *testing.T) {
	analysis := &parser.AgentAnalysis{
		Format:      "claude",
		Frontmatter: map[string]interface{}{"name": "test-skill"},
		Tools:       []string{"Read", "Write"},
	}
	data, err := ToSkillYAML(analysis, "agent.md")
	require.NoError(t, err)

	var skill model.SkillBehavior
	require.NoError(t, yaml.Unmarshal(data, &skill))

	errs := model.ValidateSkill(skill)
	assert.Empty(t, errs, "exported YAML must pass ValidateSkill: %v", errs)
}

func TestToSkillYAML_ToolsFromAnalysis(t *testing.T) {
	analysis := &parser.AgentAnalysis{
		Format:      "claude",
		Frontmatter: map[string]interface{}{"name": "tool-user"},
		Tools:       []string{"Read", "Bash", "Write"},
	}
	data, err := ToSkillYAML(analysis, "agent.md")
	require.NoError(t, err)

	var skill model.SkillBehavior
	require.NoError(t, yaml.Unmarshal(data, &skill))
	assert.Equal(t, []string{"Read", "Bash", "Write"}, skill.Strategy.Tools)
}

func TestToSkillYAML_DescriptionTruncatedAt500(t *testing.T) {
	longContent := strings.Repeat("x", 600)
	analysis := &parser.AgentAnalysis{
		Format:      "claude",
		Frontmatter: map[string]interface{}{"name": "longdoc"},
		Sections:    []parser.Section{{Header: "## Role", Content: longContent, Level: 2}},
	}
	data, err := ToSkillYAML(analysis, "agent.md")
	require.NoError(t, err)

	var skill model.SkillBehavior
	require.NoError(t, yaml.Unmarshal(data, &skill))
	assert.LessOrEqual(t, len(skill.Strategy.Steps[0]), 500)
}

// P2: nil analysis must not panic and must produce a valid skill with name from path.
func TestToSkillYAML_NilAnalysis(t *testing.T) {
	data, err := ToSkillYAML(nil, "/tmp/my-agent.md")
	require.NoError(t, err)

	var skill model.SkillBehavior
	require.NoError(t, yaml.Unmarshal(data, &skill))
	assert.Equal(t, "my-agent", skill.Skill)
}

// P5: multi-dot names must not be over-stripped (a.b.c.md → a.b.c, not a).
func TestNameFromPath_MultiDotPreservesIntermediateDots(t *testing.T) {
	assert.Equal(t, "a.b.c", nameFromPath("a.b.c.md"))
	assert.Equal(t, "code-review", nameFromPath("code-review.skill.yaml"))
}

// P1: any reasonable file path must produce a non-empty slug.
func TestNameFromPath_ProducesNonEmptySlug(t *testing.T) {
	assert.NotEmpty(t, nameFromPath("agent.md"))
	assert.NotEmpty(t, nameFromPath(".env"))
	assert.NotEmpty(t, nameFromPath("/some/path/my_agent.yaml"))
}

// P6: UTF-8 multi-byte rune at the 500-byte boundary must not be split.
func TestToSkillYAML_UTF8TruncationDoesNotSplitRune(t *testing.T) {
	// "é" is 2 bytes in UTF-8. Build content so the 500-byte slice would split mid-rune.
	// 499 ASCII chars + "é" (2 bytes) = 501 bytes; truncating at 500 bytes would split "é".
	content := strings.Repeat("a", 499) + strings.Repeat("é", 10)
	analysis := &parser.AgentAnalysis{
		Format:      "claude",
		Frontmatter: map[string]interface{}{"name": "utf8-test"},
		Sections:    []parser.Section{{Header: "## Role", Content: content, Level: 2}},
	}
	data, err := ToSkillYAML(analysis, "agent.md")
	require.NoError(t, err)

	var skill model.SkillBehavior
	require.NoError(t, yaml.Unmarshal(data, &skill))
	// Result must be valid UTF-8 (yaml.Unmarshal would have failed otherwise) and ≤ 500 runes
	assert.LessOrEqual(t, len([]rune(skill.Strategy.Steps[0])), 500)
}
