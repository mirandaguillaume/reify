package generator_test

import (
	"testing"

	"github.com/mirandaguillaume/reify/internal/generator"
	"github.com/mirandaguillaume/reify/pkg/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"
)

func TestToTitle(t *testing.T) {
	assert.Equal(t, "My Skill Name", generator.ToTitle("my-skill-name"))
	assert.Equal(t, "Simple", generator.ToTitle("simple"))
	assert.Equal(t, "", generator.ToTitle(""))
}

func TestCountWords(t *testing.T) {
	assert.Equal(t, 3, generator.CountWords("one two three"))
	assert.Equal(t, 0, generator.CountWords(""))
	assert.Equal(t, 0, generator.CountWords("   "))
}

func TestBuildSkillDescription(t *testing.T) {
	skill := model.SkillBehavior{
		Strategy: model.StrategyFacet{Approach: "analytical"},
		Context: model.ContextFacet{
			Consumes: []string{"source-code"},
			Produces: []string{"report"},
		},
	}
	desc := generator.BuildSkillDescription(skill)
	assert.Equal(t, "analytical", desc)
}

func TestBuildSkillDescription_NoConsumesProduces(t *testing.T) {
	skill := model.SkillBehavior{
		Strategy: model.StrategyFacet{Approach: "generative"},
	}
	desc := generator.BuildSkillDescription(skill)
	assert.Equal(t, "generative", desc)
}

func parseGuardrails(t *testing.T, yamlStr string) []model.GuardrailRule {
	var rules []model.GuardrailRule
	err := yaml.Unmarshal([]byte(yamlStr), &rules)
	require.NoError(t, err)
	return rules
}

func TestFormatCompactSkill_Full(t *testing.T) {
	skill := model.SkillBehavior{
		Skill: "ts-linter",
		Context: model.ContextFacet{
			Consumes: []string{"file_tree", "source_code"},
			Produces: []string{"lint_results"},
			Memory:   model.MemoryShortTerm,
		},
		Strategy: model.StrategyFacet{
			Approach: "static-analysis",
			Steps:    []string{"run linter", "parse output"},
		},
		Security: model.SecurityFacet{
			Filesystem: model.AccessReadOnly,
			Network:    model.NetworkNone,
		},
		Guardrails: parseGuardrails(t, `- fail on errors`),
	}
	out := generator.FormatCompactSkill(skill)
	assert.Contains(t, out, "**ts-linter**")
	assert.Contains(t, out, "static-analysis")
	assert.Contains(t, out, "FS: read-only")
	assert.Contains(t, out, "Net: none")
	assert.Contains(t, out, "In: file_tree, source_code")
	assert.Contains(t, out, "Out: lint_results")
	assert.Contains(t, out, "Mem: short-term")
	assert.Contains(t, out, "Steps: 1. run linter  2. parse output")
	assert.Contains(t, out, "Guardrails: fail on errors")
}

func TestFormatCompactSkill_NoStepsNoGuardrails(t *testing.T) {
	skill := model.SkillBehavior{
		Skill: "minimal",
		Context: model.ContextFacet{
			Produces: []string{"out"},
			Memory:   model.MemoryShortTerm,
		},
		Strategy: model.StrategyFacet{Approach: "generative"},
		Security: model.SecurityFacet{
			Filesystem: model.AccessReadOnly,
			Network:    model.NetworkNone,
		},
	}
	out := generator.FormatCompactSkill(skill)
	assert.Contains(t, out, "**minimal**")
	assert.NotContains(t, out, "Steps:")
	assert.NotContains(t, out, "Guardrails:")
}

func TestFormatCompactSkill_WordCountLowerThanVerbose(t *testing.T) {
	skill := model.SkillBehavior{
		Skill: "test-skill",
		Context: model.ContextFacet{
			Consumes: []string{"input"},
			Produces: []string{"output"},
			Memory:   model.MemoryShortTerm,
		},
		Strategy: model.StrategyFacet{
			Approach: "analytical",
			Steps:    []string{"step one", "step two", "step three"},
		},
		Security: model.SecurityFacet{
			Filesystem: model.AccessReadOnly,
			Network:    model.NetworkNone,
		},
		Guardrails: parseGuardrails(t, `- be careful`),
	}
	compact := generator.FormatCompactSkill(skill)
	compactWords := generator.CountWords(compact)
	// Compact should be concise — under 40 words for a simple skill
	assert.Less(t, compactWords, 40, "compact format should be terse")
}
