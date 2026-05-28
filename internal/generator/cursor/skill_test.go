package cursor_test

import (
	"strings"
	"testing"

	"github.com/mirandaguillaume/reify/internal/generator/cursor"
	"github.com/mirandaguillaume/reify/pkg/model"
	"github.com/stretchr/testify/assert"
	"gopkg.in/yaml.v3"
)

func makeGuardrailString(s string) model.GuardrailRule {
	var g model.GuardrailRule
	node := &yaml.Node{Kind: yaml.ScalarNode, Value: s}
	_ = g.UnmarshalYAML(node)
	return g
}

func testSkill() model.SkillBehavior {
	return model.SkillBehavior{
		Skill:   "code-review",
		Version: "1.0",
		Context: model.ContextFacet{
			Consumes: []string{"source-code", "diff"},
			Produces: []string{"review-report"},
			Memory:   model.MemoryConversation,
		},
		Strategy: model.StrategyFacet{
			Approach: "analytical",
			Tools:    []string{"read", "grep"},
			Steps:    []string{"Read the code", "Analyze patterns", "Write report"},
		},
		Guardrails: []model.GuardrailRule{
			makeGuardrailString("Never modify source files directly"),
			makeGuardrailString("Always explain reasoning"),
		},
		Security: model.SecurityFacet{
			Filesystem: model.AccessReadOnly,
			Network:    model.NetworkNone,
			Secrets:    []string{"GITHUB_TOKEN"},
			Sandbox:    "docker",
		},
	}
}

func TestGenerateCursorSkillMdc_Frontmatter(t *testing.T) {
	md := cursor.GenerateCursorSkillMdc(testSkill())
	assert.True(t, strings.HasPrefix(md, "---\n"))
	assert.Contains(t, md, "description: ")
	assert.Contains(t, md, "alwaysApply: false")
	// Two `---` lines bound the frontmatter.
	assert.Equal(t, 2, strings.Count(md, "---\n"))
}

func TestGenerateCursorSkillMdc_Title(t *testing.T) {
	md := cursor.GenerateCursorSkillMdc(testSkill())
	assert.Contains(t, md, "# Code Review")
}

func TestGenerateCursorSkillMdc_RulesBeforeSteps(t *testing.T) {
	md := cursor.GenerateCursorSkillMdc(testSkill())
	rulesIdx := strings.Index(md, "## Rules")
	stepsIdx := strings.Index(md, "## Steps")
	assert.Greater(t, rulesIdx, 0, "Rules section should be present")
	assert.Greater(t, stepsIdx, rulesIdx, "Rules (guardrails) should appear before Steps — primacy bias")
}

func TestGenerateCursorSkillMdc_StepsNumbered(t *testing.T) {
	md := cursor.GenerateCursorSkillMdc(testSkill())
	assert.Contains(t, md, "1. Read the code")
	assert.Contains(t, md, "2. Analyze patterns")
	assert.Contains(t, md, "3. Write report")
}

func TestGenerateCursorSkillMdc_Tools(t *testing.T) {
	md := cursor.GenerateCursorSkillMdc(testSkill())
	assert.Contains(t, md, "## Tools")
	assert.Contains(t, md, "read, grep")
}

func TestGenerateCursorSkillMdc_NoGuardrails(t *testing.T) {
	skill := testSkill()
	skill.Guardrails = nil
	md := cursor.GenerateCursorSkillMdc(skill)
	assert.NotContains(t, md, "## Rules")
}

func TestGenerateCursorSkillMdc_NoSteps(t *testing.T) {
	skill := testSkill()
	skill.Strategy.Steps = nil
	md := cursor.GenerateCursorSkillMdc(skill)
	assert.NotContains(t, md, "## Steps")
}

func TestGenerateCursorSkillMdc_NoTools(t *testing.T) {
	skill := testSkill()
	skill.Strategy.Tools = nil
	md := cursor.GenerateCursorSkillMdc(skill)
	assert.NotContains(t, md, "## Tools")
}

func TestGenerateCursorSkillMdc_OmitsNonCursorSections(t *testing.T) {
	// Cursor .mdc is intentionally minimal — no Context, Security, Examples, etc.
	md := cursor.GenerateCursorSkillMdc(testSkill())
	assert.NotContains(t, md, "## Context")
	assert.NotContains(t, md, "## Security")
	assert.NotContains(t, md, "## Strategy")
	assert.NotContains(t, md, "## Examples")
}

func TestInferGlobs_ViaFrontmatter(t *testing.T) {
	cases := []struct {
		label     string
		skillName string
		tools     []string
		want      string
	}{
		{"typescript-name", "typescript-helper", nil, "**/*.ts, **/*.tsx"},
		{"typescript-tool", "anything-else", []string{"typescript"}, "**/*.ts, **/*.tsx"},
		{"python-name", "python-fmt", nil, "**/*.py"},
		{"python-tool", "anything", []string{"python"}, "**/*.py"},
		{"go-name", "go-vet", nil, "**/*.go"},
		{"golang-tool", "anything", []string{"golang"}, "**/*.go"},
		{"test-name", "snapshot-tester", nil, "**/*_test.*, **/*.test.*, **/*.spec.*"},
		{"css-name", "css-linter", nil, "**/*.css, **/*.scss, **/*.sass"},
		{"style-name", "style-guide", nil, "**/*.css, **/*.scss, **/*.sass"},
		{"no-match", "review", []string{"read", "grep"}, ""},
	}
	for _, tc := range cases {
		t.Run(tc.label, func(t *testing.T) {
			skill := model.SkillBehavior{
				Skill:    tc.skillName,
				Strategy: model.StrategyFacet{Tools: tc.tools},
			}
			md := cursor.GenerateCursorSkillMdc(skill)
			if tc.want == "" {
				assert.NotContains(t, md, "globs: ", "no-match case should omit globs line")
			} else {
				assert.Contains(t, md, "globs: "+tc.want)
			}
		})
	}
}

func TestInferGlobs_TypescriptBeatsTest(t *testing.T) {
	// Priority verification: "typescript-test" matches typescript first, not test.
	skill := model.SkillBehavior{Skill: "typescript-test-helper"}
	md := cursor.GenerateCursorSkillMdc(skill)
	assert.Contains(t, md, "globs: **/*.ts, **/*.tsx")
	assert.NotContains(t, md, "**/*_test.*")
}
