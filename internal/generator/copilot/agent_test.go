package copilot_test

import (
	"strings"
	"testing"

	"github.com/mirandaguillaume/reify/internal/generator/copilot"
	"github.com/mirandaguillaume/reify/pkg/model"
	"github.com/stretchr/testify/assert"
)

func testAgent() model.AgentComposition {
	return model.AgentComposition{
		Agent:         "code-reviewer",
		Description:   "Reviews code for quality and security issues",
		Skills:        []string{"code-review", "security-scan"},
		Orchestration: model.OrchestrationSequential,
	}
}

func testResolvedSkills() []model.SkillBehavior {
	return []model.SkillBehavior{
		{
			Skill: "code-review",
			Context: model.ContextFacet{
				Consumes: []string{"source-code"},
				Produces: []string{"review-report"},
				Memory:   model.MemoryConversation,
			},
			Strategy: model.StrategyFacet{
				Approach: "analytical",
				Tools:    []string{"read", "grep"},
				Effort:   model.EffortMedium,
			},
			Security: model.SecurityFacet{
				Filesystem: model.AccessReadOnly,
				Network:    model.NetworkNone,
			},
		},
		{
			Skill: "security-scan",
			Context: model.ContextFacet{
				Consumes: []string{"source-code"},
				Produces: []string{"security-report"},
				Memory:   model.MemoryShortTerm,
			},
			Strategy: model.StrategyFacet{
				Approach: "scanning",
				Tools:    []string{"bash", "grep"},
				Effort:   model.EffortLight,
			},
			Security: model.SecurityFacet{
				Filesystem: model.AccessReadWrite,
				Network:    model.NetworkFull,
			},
		},
	}
}

func TestGenerateCopilotAgentMd_Frontmatter(t *testing.T) {
	md := copilot.GenerateCopilotAgentMd(testAgent(), testResolvedSkills(), ".github")
	assert.Contains(t, md, "---\nname: code-reviewer\n")
	assert.Contains(t, md, "description: Reviews code for quality and security issues")
}


func TestGenerateCopilotAgentMd_SequentialOrchestration(t *testing.T) {
	md := copilot.GenerateCopilotAgentMd(testAgent(), testResolvedSkills(), ".github")
	assert.Contains(t, md, "Execute 2 skills sequentially as independent subagents")
}

func TestGenerateCopilotAgentMd_ParallelOrchestration(t *testing.T) {
	agent := testAgent()
	agent.Orchestration = model.OrchestrationParallel
	md := copilot.GenerateCopilotAgentMd(agent, testResolvedSkills(), ".github")
	assert.Contains(t, md, "Launch 2 skills as parallel subagents")
}

func TestGenerateCopilotAgentMd_AdaptiveOrchestration(t *testing.T) {
	agent := testAgent()
	agent.Orchestration = model.OrchestrationAdaptive
	md := copilot.GenerateCopilotAgentMd(agent, testResolvedSkills(), ".github")
	assert.Contains(t, md, "Dispatch 2 skills as subagents, choosing execution order dynamically")
}

func TestGenerateCopilotAgentMd_SkillContextInfo(t *testing.T) {
	md := copilot.GenerateCopilotAgentMd(testAgent(), testResolvedSkills(), ".github")
	assert.Contains(t, md, "- In: source-code")
	assert.Contains(t, md, "- Out: review-report")
	assert.Contains(t, md, "- Out: security-report")
}

func TestGenerateCopilotAgentMd_OutputSection(t *testing.T) {
	md := copilot.GenerateCopilotAgentMd(testAgent(), testResolvedSkills(), ".github")
	assert.Contains(t, md, "## Output")
	assert.Contains(t, md, "review-report")
	assert.Contains(t, md, "security-report")
}

func TestGenerateCopilotAgentMd_NoDescription(t *testing.T) {
	agent := testAgent()
	agent.Description = ""
	md := copilot.GenerateCopilotAgentMd(agent, testResolvedSkills(), ".github")
	assert.NotContains(t, md, "description:")
}

func TestGenerateCopilotAgentMd_NoSkills(t *testing.T) {
	agent := testAgent()
	md := copilot.GenerateCopilotAgentMd(agent, nil, ".github")
	assert.NotContains(t, md, "tools:")
	assert.NotContains(t, md, "## Output")
}

func TestResolveCopilotAgentTools(t *testing.T) {
	tools := copilot.ResolveCopilotAgentTools(testResolvedSkills())
	// Should contain tools from both skills, merged and ordered
	assert.Contains(t, tools, "search")
	assert.Contains(t, tools, "read")
	assert.Contains(t, tools, "execute")
}

func TestGenerateCopilotAgentMd_SubagentFormat(t *testing.T) {
	md := copilot.GenerateCopilotAgentMd(testAgent(), testResolvedSkills(), ".github")
	assert.Contains(t, md, "Launch a subagent")
	assert.Contains(t, md, "Model: sonnet")
	assert.Contains(t, md, "Model: haiku")
}

func TestGenerateCopilotAgentMd_SkillPath(t *testing.T) {
	md := copilot.GenerateCopilotAgentMd(testAgent(), testResolvedSkills(), ".github")
	assert.Contains(t, md, "Skill: `.github/skills/code-review/SKILL.md`")
}

func TestGenerateCopilotAgentMd_OrchestratorTool(t *testing.T) {
	md := copilot.GenerateCopilotAgentMd(testAgent(), testResolvedSkills(), ".github")
	assert.Contains(t, md, `tools: ["task"]`)
}

// --- Compact format tests ---

func TestGenerateCompactCopilotAgentMd_Frontmatter(t *testing.T) {
	md := copilot.GenerateCompactCopilotAgentMd(testAgent(), testResolvedSkills())
	assert.Contains(t, md, "---\nname: code-reviewer\n")
	assert.Contains(t, md, "description: Reviews code for quality and security issues")
	assert.Contains(t, md, "tools: [")
}

func TestGenerateCompactCopilotAgentMd_NoStepHeaders(t *testing.T) {
	md := copilot.GenerateCompactCopilotAgentMd(testAgent(), testResolvedSkills())
	assert.NotContains(t, md, "### Step")
	assert.NotContains(t, md, "Read `.github/skills/")
	assert.NotContains(t, md, "follow its instructions")
}

func TestGenerateCompactCopilotAgentMd_InlinedSkills(t *testing.T) {
	md := copilot.GenerateCompactCopilotAgentMd(testAgent(), testResolvedSkills())
	assert.Contains(t, md, "**code-review**")
	assert.Contains(t, md, "**security-scan**")
	assert.Contains(t, md, "FS: read-only")
	assert.Contains(t, md, "FS: read-write")
}

func TestGenerateCompactCopilotAgentMd_FewerWords(t *testing.T) {
	standard := copilot.GenerateCopilotAgentMd(testAgent(), testResolvedSkills(), ".github")
	compact := copilot.GenerateCompactCopilotAgentMd(testAgent(), testResolvedSkills())
	stdWords := len(strings.Fields(standard))
	cmpWords := len(strings.Fields(compact))
	assert.Less(t, cmpWords, stdWords, "compact should have fewer words")
}

func TestGenerateCompactCopilotAgentMd_OutputSection(t *testing.T) {
	md := copilot.GenerateCompactCopilotAgentMd(testAgent(), testResolvedSkills())
	assert.Contains(t, md, "## Output")
	assert.Contains(t, md, "review-report")
	assert.Contains(t, md, "security-report")
}

// --- Staged agent tests ---

func testStagedAgent() model.AgentComposition {
	return model.AgentComposition{
		Agent:       "code-reviewer",
		Description: "Multi-stage code review pipeline",
		Consumes:    []string{"pr_url", "file_tree"},
		Produces:    []string{"review_comment"},
		Stages: []model.Stage{
			{Name: "preflight", Strategy: model.OrchestrationSequential, Skills: []string{"eligibility-checker", "summarizer"}},
			{Name: "analysis", Strategy: model.OrchestrationParallel, Skills: []string{"bug-scanner", "history-reviewer"}},
			{Name: "publish", Strategy: model.OrchestrationSequential, Skills: []string{"commenter"}},
		},
	}
}

func testStagedResolvedSkills() []model.SkillBehavior {
	return []model.SkillBehavior{
		{
			Skill:    "eligibility-checker",
			Context:  model.ContextFacet{Consumes: []string{"pr_url"}, Produces: []string{"eligibility_status"}, Memory: model.MemoryShortTerm},
			Strategy: model.StrategyFacet{Approach: "gate-check", Tools: []string{"bash"}, Effort: model.EffortLight},
			Security: model.SecurityFacet{Filesystem: model.AccessNone, Network: model.NetworkAllowlist},
		},
		{
			Skill:    "summarizer",
			Context:  model.ContextFacet{Consumes: []string{"pr_url"}, Produces: []string{"pr_summary"}, Memory: model.MemoryShortTerm},
			Strategy: model.StrategyFacet{Approach: "diff-first", Tools: []string{"bash"}, Effort: model.EffortLight},
			Security: model.SecurityFacet{Filesystem: model.AccessNone, Network: model.NetworkAllowlist},
		},
		{
			Skill:    "bug-scanner",
			Context:  model.ContextFacet{Consumes: []string{"pr_diff"}, Produces: []string{"review_issues"}, Memory: model.MemoryShortTerm},
			Strategy: model.StrategyFacet{Approach: "diff-first", Tools: []string{"read_file"}, Effort: model.EffortMedium},
			Security: model.SecurityFacet{Filesystem: model.AccessReadOnly, Network: model.NetworkNone},
		},
		{
			Skill:    "history-reviewer",
			Context:  model.ContextFacet{Consumes: []string{"pr_diff", "git_blame"}, Produces: []string{"review_issues"}, Memory: model.MemoryShortTerm},
			Strategy: model.StrategyFacet{Approach: "history-first", Tools: []string{"bash", "read_file"}, Effort: model.EffortMedium},
			Security: model.SecurityFacet{Filesystem: model.AccessReadOnly, Network: model.NetworkNone},
		},
		{
			Skill:    "commenter",
			Context:  model.ContextFacet{Consumes: []string{"scored_issues", "pr_url"}, Produces: []string{"review_comment"}, Memory: model.MemoryShortTerm},
			Strategy: model.StrategyFacet{Approach: "output-format", Tools: []string{"bash"}, Effort: model.EffortLight},
			Security: model.SecurityFacet{Filesystem: model.AccessNone, Network: model.NetworkAllowlist},
		},
	}
}

func TestGenerateCopilotAgentMd_Staged_PipelineTable(t *testing.T) {
	md := copilot.GenerateCopilotAgentMd(testStagedAgent(), testStagedResolvedSkills(), ".github")
	assert.Contains(t, md, "## Pipeline")
	assert.Contains(t, md, "| Stage | Strategy | Skills |")
	assert.Contains(t, md, "| preflight | sequential | eligibility-checker, summarizer |")
	assert.Contains(t, md, "| analysis | parallel | bug-scanner, history-reviewer |")
	assert.Contains(t, md, "| publish | sequential | commenter |")
}

func TestGenerateCopilotAgentMd_Staged_FlatSteps(t *testing.T) {
	md := copilot.GenerateCopilotAgentMd(testStagedAgent(), testStagedResolvedSkills(), ".github")
	assert.Contains(t, md, "### Step 1: Eligibility Checker")
	assert.Contains(t, md, "### Step 2: Summarizer")
	assert.Contains(t, md, "### Step 3: Bug Scanner")
	assert.Contains(t, md, "### Step 5: Commenter")
	// h3, not h4
	assert.NotContains(t, md, "#### Step")
}

func TestGenerateCopilotAgentMd_Staged_NoOrchestrationDirectives(t *testing.T) {
	md := copilot.GenerateCopilotAgentMd(testStagedAgent(), testStagedResolvedSkills(), ".github")
	assert.NotContains(t, md, "Launch these subagents in parallel")
	assert.NotContains(t, md, "### Stage:")
	assert.NotContains(t, md, "stages sequentially")
	assert.NotContains(t, md, "across 3 stages")
}

func TestGenerateCopilotAgentMd_Staged_SkillPaths(t *testing.T) {
	md := copilot.GenerateCopilotAgentMd(testStagedAgent(), testStagedResolvedSkills(), ".github")
	assert.Contains(t, md, "Skill: `.github/skills/eligibility-checker/SKILL.md`")
	assert.Contains(t, md, "Skill: `.github/skills/commenter/SKILL.md`")
}

func TestGenerateCopilotAgentMd_Staged_Frontmatter(t *testing.T) {
	md := copilot.GenerateCopilotAgentMd(testStagedAgent(), testStagedResolvedSkills(), ".github")
	assert.Contains(t, md, "name: code-reviewer")
	assert.Contains(t, md, `tools: ["task"]`)
}
