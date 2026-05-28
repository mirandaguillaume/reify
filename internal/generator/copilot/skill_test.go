package copilot_test

import (
	"strings"
	"testing"

	"github.com/mirandaguillaume/reify/internal/generator/copilot"
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

func TestGenerateCopilotSkillMd_Frontmatter(t *testing.T) {
	md := copilot.GenerateCopilotSkillMd(testSkill(), nil)
	assert.Contains(t, md, "---\nname: code-review\n")
	assert.Contains(t, md, "description: analytical")
}

func TestGenerateCopilotSkillMd_Title(t *testing.T) {
	md := copilot.GenerateCopilotSkillMd(testSkill(), nil)
	assert.Contains(t, md, "# Code Review")
}

func TestGenerateCopilotSkillMd_GuardrailsBeforeContext(t *testing.T) {
	md := copilot.GenerateCopilotSkillMd(testSkill(), nil)
	guardrailIdx := strings.Index(md, "## Guardrails")
	contextIdx := strings.Index(md, "## Context")
	assert.Greater(t, contextIdx, guardrailIdx, "Guardrails should appear before Context")
}

func TestGenerateCopilotSkillMd_SecurityLast(t *testing.T) {
	md := copilot.GenerateCopilotSkillMd(testSkill(), nil)
	securityIdx := strings.Index(md, "## Security")
	strategyIdx := strings.Index(md, "## Strategy")
	assert.Greater(t, securityIdx, strategyIdx, "Security should appear after Strategy")
}

func TestGenerateCopilotSkillMd_ContextSection(t *testing.T) {
	md := copilot.GenerateCopilotSkillMd(testSkill(), nil)
	assert.Contains(t, md, "Consumes: source-code, diff")
	assert.Contains(t, md, "Produces: review-report")
	assert.Contains(t, md, "Memory: conversation")
}

func TestGenerateCopilotSkillMd_StrategySection(t *testing.T) {
	md := copilot.GenerateCopilotSkillMd(testSkill(), nil)
	assert.Contains(t, md, "Approach: analytical")
	assert.Contains(t, md, "Tools: read, grep")
}

func TestGenerateCopilotSkillMd_StepsNumbered(t *testing.T) {
	md := copilot.GenerateCopilotSkillMd(testSkill(), nil)
	assert.Contains(t, md, "1. Read the code")
	assert.Contains(t, md, "2. Analyze patterns")
	assert.Contains(t, md, "3. Write report")
}

func TestGenerateCopilotSkillMd_NoDependenciesSection(t *testing.T) {
	md := copilot.GenerateCopilotSkillMd(testSkill(), nil)
	assert.NotContains(t, md, "## Dependencies")
}

func TestGenerateCopilotSkillMd_Security(t *testing.T) {
	md := copilot.GenerateCopilotSkillMd(testSkill(), nil)
	assert.Contains(t, md, "- Filesystem: read-only")
	assert.Contains(t, md, "- Network: none")
	assert.Contains(t, md, "- Secrets: GITHUB_TOKEN")
	assert.Contains(t, md, "- Sandbox: docker")
}

func TestGenerateCopilotSkillMd_NoGuardrails(t *testing.T) {
	skill := testSkill()
	skill.Guardrails = nil
	md := copilot.GenerateCopilotSkillMd(skill, nil)
	assert.NotContains(t, md, "## Guardrails")
}

func TestGenerateCopilotSkillMd_DescriptionTruncation(t *testing.T) {
	skill := testSkill()
	// Create a skill whose approach description exceeds 1024 chars
	skill.Strategy.Approach = strings.Repeat("a very long approach description ", 40)
	md := copilot.GenerateCopilotSkillMd(skill, nil)

	// Extract the description from frontmatter
	parts := strings.SplitN(md, "---", 3)
	frontmatter := parts[1]
	for _, line := range strings.Split(frontmatter, "\n") {
		if strings.HasPrefix(line, "description: ") {
			desc := strings.TrimPrefix(line, "description: ")
			assert.LessOrEqual(t, len(desc), 1024)
			assert.True(t, strings.HasSuffix(desc, "..."))
			break
		}
	}
}

func TestGenerateCopilotSkillMd_ShortDescriptionNotTruncated(t *testing.T) {
	md := copilot.GenerateCopilotSkillMd(testSkill(), nil)

	parts := strings.SplitN(md, "---", 3)
	frontmatter := parts[1]
	for _, line := range strings.Split(frontmatter, "\n") {
		if strings.HasPrefix(line, "description: ") {
			desc := strings.TrimPrefix(line, "description: ")
			assert.False(t, strings.HasSuffix(desc, "..."))
			break
		}
	}
}

func TestGenerateCopilotSkillMd_WhenToUse(t *testing.T) {
	skill := testSkill()
	skill.WhenToUse = model.WhenToUseFacet{
		Triggers: []string{"Test failures"},
		DontUse:  []string{"Typos"},
	}
	md := copilot.GenerateCopilotSkillMd(skill, nil)
	assert.Contains(t, md, "## When to Use")
	assert.Contains(t, md, "- Test failures")
	assert.Contains(t, md, "- Typos")
}

func TestGenerateCopilotSkillMd_Examples(t *testing.T) {
	skill := testSkill()
	skill.Examples = []model.CodeExample{
		{Label: "Good: verify", Code: "go test ./...", Lang: "bash"},
	}
	md := copilot.GenerateCopilotSkillMd(skill, nil)
	assert.Contains(t, md, "## Examples")
	assert.Contains(t, md, "```bash")
}

func TestGenerateCopilotSkillMd_AntiPatterns(t *testing.T) {
	skill := testSkill()
	skill.AntiPatterns = []model.AntiPattern{
		{Excuse: "Quick fix", Reality: "Do it right"},
	}
	md := copilot.GenerateCopilotSkillMd(skill, nil)
	assert.Contains(t, md, "## Red Flags")
	assert.Contains(t, md, "| Quick fix | Do it right |")
}

func TestGenerateCopilotSkillMd_ExamplesAndAntiPatternsBeforeSecurity(t *testing.T) {
	skill := testSkill()
	skill.Examples = []model.CodeExample{{Label: "test", Code: "echo"}}
	skill.AntiPatterns = []model.AntiPattern{{Excuse: "a", Reality: "b"}}
	md := copilot.GenerateCopilotSkillMd(skill, nil)
	exIdx := strings.Index(md, "## Examples")
	secIdx := strings.Index(md, "## Security")
	assert.Greater(t, secIdx, exIdx, "Security should appear after Examples")
}
