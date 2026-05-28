package reify

import (
	"testing"

	"github.com/mirandaguillaume/reify/pkg/model"
	"github.com/stretchr/testify/assert"
)

func makeTestSkill() model.SkillBehavior {
	return model.SkillBehavior{
		Skill:   "ts-linter",
		Version: "1.0.0",
		Context: model.ContextFacet{
			Consumes: []string{"file_tree", "source_code"},
			Produces: []string{"lint_results"},
		},
		Strategy: model.StrategyFacet{
			Approach: "static-analysis",
			Steps:    []string{"check for common anti-patterns", "produce structured lint report"},
		},
		Security: model.SecurityFacet{
			Filesystem: model.AccessReadOnly,
			Network:    model.NetworkNone,
		},
	}
}

func TestBuildPromptTemplate_ContainsSkillName(t *testing.T) {
	skill := makeTestSkill()
	prompt := BuildPromptTemplate(skill)
	assert.Contains(t, prompt, "ts-linter")
}

func TestBuildPromptTemplate_ContainsSteps(t *testing.T) {
	skill := makeTestSkill()
	prompt := BuildPromptTemplate(skill)
	assert.Contains(t, prompt, "1. check for common anti-patterns")
	assert.Contains(t, prompt, "2. produce structured lint report")
}

func TestBuildPromptTemplate_ContainsInputPlaceholders(t *testing.T) {
	skill := makeTestSkill()
	prompt := BuildPromptTemplate(skill)
	assert.Contains(t, prompt, "{{ .file_tree }}")
	assert.Contains(t, prompt, "{{ .source_code }}")
}

func TestBuildPromptTemplate_ContainsProduces(t *testing.T) {
	skill := makeTestSkill()
	prompt := BuildPromptTemplate(skill)
	assert.Contains(t, prompt, "lint_results")
}

func TestBuildPromptTemplate_ContainsApproach(t *testing.T) {
	skill := makeTestSkill()
	prompt := BuildPromptTemplate(skill)
	assert.Contains(t, prompt, "static-analysis")
}

func TestBuildPromptTemplate_EmptyOptionalFields(t *testing.T) {
	skill := model.SkillBehavior{
		Skill: "minimal-skill",
		Context: model.ContextFacet{
			Consumes: []string{"source_code"},
			Produces: []string{"report"},
		},
	}
	prompt := BuildPromptTemplate(skill)
	// Input header must be present (Consumes is non-empty)
	assert.Contains(t, prompt, "## Input")
	// Output header must be present (Produces is non-empty)
	assert.Contains(t, prompt, "## Output")
	// No dangling Produce: line with empty value
	assert.NotContains(t, prompt, "Produce: \n")
	// No empty ## Input header without content
	assert.NotContains(t, prompt, "## Input\n## ")
}

func TestBuildPromptTemplate_EmptyConsumes(t *testing.T) {
	skill := model.SkillBehavior{
		Skill: "no-consumes-skill",
		Context: model.ContextFacet{
			Consumes: []string{},
			Produces: []string{"report"},
		},
	}
	prompt := BuildPromptTemplate(skill)
	assert.NotContains(t, prompt, "## Input")
}

func TestBuildPromptTemplate_EmptyProduces(t *testing.T) {
	skill := model.SkillBehavior{
		Skill: "no-produces-skill",
		Context: model.ContextFacet{
			Consumes: []string{"source_code"},
			Produces: []string{},
		},
	}
	prompt := BuildPromptTemplate(skill)
	assert.NotContains(t, prompt, "## Output")
}
