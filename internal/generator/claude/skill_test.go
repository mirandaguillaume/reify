package claude_test

import (
	"strings"
	"testing"

	"github.com/mirandaguillaume/reify/internal/generator/claude"
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

func TestGenerateSkillMd_Frontmatter(t *testing.T) {
	md := claude.GenerateSkillMd(testSkill(), nil, "")
	assert.Contains(t, md, "---\nname: code-review\n")
	assert.Contains(t, md, "description: analytical")
}

func TestGenerateSkillMd_Title(t *testing.T) {
	md := claude.GenerateSkillMd(testSkill(), nil, "")
	assert.Contains(t, md, "# Code Review")
}

func TestGenerateSkillMd_GuardrailsFirst(t *testing.T) {
	md := claude.GenerateSkillMd(testSkill(), nil, "")
	guardrailIdx := strings.Index(md, "## Guardrails")
	stepsIdx := strings.Index(md, "## Steps")
	assert.Greater(t, stepsIdx, guardrailIdx, "Guardrails should appear before Steps")
}

func TestGenerateSkillMd_NoContextSection(t *testing.T) {
	md := claude.GenerateSkillMd(testSkill(), nil, "")
	assert.NotContains(t, md, "## Context")
	assert.NotContains(t, md, "Consumes:")
	assert.NotContains(t, md, "Memory:")
}

func TestGenerateSkillMd_NoSecuritySection(t *testing.T) {
	md := claude.GenerateSkillMd(testSkill(), nil, "")
	assert.NotContains(t, md, "## Security")
}

func TestGenerateSkillMd_NoWhenToUseSection(t *testing.T) {
	skill := testSkill()
	skill.WhenToUse = model.WhenToUseFacet{
		Triggers: []string{"Test failures"},
	}
	md := claude.GenerateSkillMd(skill, nil, "")
	assert.NotContains(t, md, "## When to Use")
}

func TestGenerateSkillMd_NoStrategyMetadata(t *testing.T) {
	md := claude.GenerateSkillMd(testSkill(), nil, "")
	assert.NotContains(t, md, "## Strategy")
	assert.NotContains(t, md, "Approach:")
	assert.NotContains(t, md, "Tools:")
}

func TestGenerateSkillMd_StepsNumbered(t *testing.T) {
	md := claude.GenerateSkillMd(testSkill(), nil, "")
	assert.Contains(t, md, "1. Read the code")
	assert.Contains(t, md, "2. Analyze patterns")
	assert.Contains(t, md, "3. Write report")
}

func TestGenerateSkillMd_NoGuardrails(t *testing.T) {
	skill := testSkill()
	skill.Guardrails = nil
	md := claude.GenerateSkillMd(skill, nil, "")
	assert.NotContains(t, md, "## Guardrails")
}

func TestGenerateSkillMd_Examples(t *testing.T) {
	skill := testSkill()
	skill.Examples = []model.CodeExample{
		{Label: "Good: verify", Code: "go test ./...", Lang: "bash"},
	}
	md := claude.GenerateSkillMd(skill, nil, "")
	assert.Contains(t, md, "## Examples")
	assert.Contains(t, md, "**Good: verify**")
	assert.Contains(t, md, "```bash")
	assert.Contains(t, md, "go test ./...")
}

func TestGenerateSkillMd_ExamplesEmpty(t *testing.T) {
	md := claude.GenerateSkillMd(testSkill(), nil, "")
	assert.NotContains(t, md, "## Examples")
}

func TestGenerateSkillMd_AntiPatterns(t *testing.T) {
	skill := testSkill()
	skill.AntiPatterns = []model.AntiPattern{
		{Excuse: "Quick fix", Reality: "Do it right"},
	}
	md := claude.GenerateSkillMd(skill, nil, "")
	assert.Contains(t, md, "## Red Flags")
	assert.Contains(t, md, "| Quick fix | Do it right |")
}

func TestGenerateSkillMd_AntiPatternsEmpty(t *testing.T) {
	md := claude.GenerateSkillMd(testSkill(), nil, "")
	assert.NotContains(t, md, "## Red Flags")
}

func TestGenerateSkillMd_ContractInlined(t *testing.T) {
	skill := testSkill()
	contracts := map[string]string{
		"review-report": "```\nseverity: {level}\n```",
	}
	md := claude.GenerateSkillMd(skill, contracts, "")
	assert.Contains(t, md, "## Output Format")
	assert.Contains(t, md, "severity: {level}")
}

func TestGenerateSkillMd_ContractAsPointer(t *testing.T) {
	skill := testSkill()
	contracts := map[string]string{
		"review-report": "```\nseverity: {level}\n```",
	}
	md := claude.GenerateSkillMd(skill, contracts, "/project/contracts")
	assert.Contains(t, md, "## Output")
	assert.Contains(t, md, "`/project/contracts/review-report")
	assert.NotContains(t, md, "severity: {level}")
}

func TestGenerateSkillMd_SectionOrder(t *testing.T) {
	skill := testSkill()
	skill.Examples = []model.CodeExample{{Label: "test", Code: "echo hi"}}
	skill.AntiPatterns = []model.AntiPattern{{Excuse: "a", Reality: "b"}}
	md := claude.GenerateSkillMd(skill, nil, "")
	guardrailIdx := strings.Index(md, "## Guardrails")
	stepsIdx := strings.Index(md, "## Steps")
	exIdx := strings.Index(md, "## Examples")
	apIdx := strings.Index(md, "## Red Flags")
	assert.Greater(t, stepsIdx, guardrailIdx, "Steps after Guardrails")
	assert.Greater(t, exIdx, stepsIdx, "Examples after Steps")
	assert.Greater(t, apIdx, exIdx, "Red Flags after Examples")
}
