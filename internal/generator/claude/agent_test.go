package claude_test

import (
	"strings"
	"testing"

	"github.com/mirandaguillaume/reify/internal/generator/claude"
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

func TestGenerateAgentMd_Frontmatter(t *testing.T) {
	md := claude.GenerateAgentMd(testAgent(), testResolvedSkills(), ".claude")
	assert.Contains(t, md, "---\nname: code-reviewer\n")
	assert.Contains(t, md, "description: Reviews code for quality and security issues")
}

func TestGenerateAgentMd_SequentialOrchestration(t *testing.T) {
	md := claude.GenerateAgentMd(testAgent(), testResolvedSkills(), ".claude")
	assert.Contains(t, md, "sequentially as independent subagents")
}

func TestGenerateAgentMd_ParallelOrchestration(t *testing.T) {
	agent := testAgent()
	agent.Orchestration = model.OrchestrationParallel
	md := claude.GenerateAgentMd(agent, testResolvedSkills(), ".claude")
	assert.Contains(t, md, "parallel subagents")
}

func TestGenerateAgentMd_AdaptiveOrchestration(t *testing.T) {
	agent := testAgent()
	agent.Orchestration = model.OrchestrationAdaptive
	md := claude.GenerateAgentMd(agent, testResolvedSkills(), ".claude")
	assert.Contains(t, md, "subagents, choosing execution order")
}

func TestGenerateAgentMd_SkillReferences(t *testing.T) {
	md := claude.GenerateAgentMd(testAgent(), testResolvedSkills(), ".claude")
	assert.Contains(t, md, "### Step 1: Code Review")
	assert.Contains(t, md, "### Step 2: Security Scan")
}

func TestGenerateAgentMd_SkillContextInfo(t *testing.T) {
	md := claude.GenerateAgentMd(testAgent(), testResolvedSkills(), ".claude")
	assert.Contains(t, md, "- In: source-code")
	assert.Contains(t, md, "- Out: review-report")
	assert.Contains(t, md, "- Out: security-report")
}

func TestGenerateAgentMd_OutputSection(t *testing.T) {
	md := claude.GenerateAgentMd(testAgent(), testResolvedSkills(), ".claude")
	assert.Contains(t, md, "## Output")
	assert.Contains(t, md, "review-report")
	assert.Contains(t, md, "security-report")
}

func TestGenerateAgentMd_NoDescription(t *testing.T) {
	agent := testAgent()
	agent.Description = ""
	md := claude.GenerateAgentMd(agent, testResolvedSkills(), ".claude")
	assert.NotContains(t, md, "description:")
}

func TestGenerateAgentMd_NoSkills(t *testing.T) {
	agent := testAgent()
	md := claude.GenerateAgentMd(agent, nil, ".claude")
	assert.NotContains(t, md, "tools:")
	assert.NotContains(t, md, "## Output")
}

func TestGenerateAgentMd_OrchestratorToolIsTask(t *testing.T) {
	md := claude.GenerateAgentMd(testAgent(), testResolvedSkills(), ".claude")
	assert.Contains(t, md, "tools: Task")
	assert.NotContains(t, md, "Bash")
	assert.NotContains(t, md, "Grep")
}

func TestGenerateAgentMd_SubagentModel(t *testing.T) {
	md := claude.GenerateAgentMd(testAgent(), testResolvedSkills(), ".claude")
	assert.Contains(t, md, "Model: sonnet")
	assert.Contains(t, md, "Model: haiku")
}

func TestGenerateAgentMd_SubagentSkillPath(t *testing.T) {
	md := claude.GenerateAgentMd(testAgent(), testResolvedSkills(), ".claude")
	assert.Contains(t, md, "Skill: `.claude/skills/code-review/SKILL.md`")
	assert.Contains(t, md, "Skill: `.claude/skills/security-scan/SKILL.md`")
}

func TestGenerateAgentMd_LaunchSubagent(t *testing.T) {
	md := claude.GenerateAgentMd(testAgent(), testResolvedSkills(), ".claude")
	assert.Contains(t, md, "Launch a subagent")
}

func TestResolveAgentTools(t *testing.T) {
	tools := claude.ResolveAgentTools(testResolvedSkills())
	// Should contain tools from both skills, merged and ordered
	assert.Contains(t, tools, "Grep")
	assert.Contains(t, tools, "Read")
	assert.Contains(t, tools, "Bash")
}

// --- Compact format tests ---

func TestGenerateCompactAgentMd_Frontmatter(t *testing.T) {
	md := claude.GenerateCompactAgentMd(testAgent(), testResolvedSkills())
	assert.Contains(t, md, "---\nname: code-reviewer\n")
	assert.Contains(t, md, "description: Reviews code for quality and security issues")
	assert.Contains(t, md, "tools: ")
}

func TestGenerateCompactAgentMd_NoStepHeaders(t *testing.T) {
	md := claude.GenerateCompactAgentMd(testAgent(), testResolvedSkills())
	assert.NotContains(t, md, "### Step")
	assert.NotContains(t, md, "Read `.claude/skills/")
	assert.NotContains(t, md, "follow its instructions")
}

func TestGenerateCompactAgentMd_InlinedSkills(t *testing.T) {
	md := claude.GenerateCompactAgentMd(testAgent(), testResolvedSkills())
	assert.Contains(t, md, "**code-review**")
	assert.Contains(t, md, "**security-scan**")
	assert.Contains(t, md, "FS: read-only")
	assert.Contains(t, md, "FS: read-write")
}

func TestGenerateCompactAgentMd_FewerWords(t *testing.T) {
	standard := claude.GenerateAgentMd(testAgent(), testResolvedSkills(), ".claude")
	compact := claude.GenerateCompactAgentMd(testAgent(), testResolvedSkills())
	stdWords := len(strings.Fields(standard))
	cmpWords := len(strings.Fields(compact))
	assert.Less(t, cmpWords, stdWords, "compact should have fewer words")
}

func TestGenerateCompactAgentMd_OutputSection(t *testing.T) {
	md := claude.GenerateCompactAgentMd(testAgent(), testResolvedSkills())
	assert.Contains(t, md, "## Output")
	assert.Contains(t, md, "review-report")
	assert.Contains(t, md, "security-report")
}

func TestGenerateCompactAgentMd_Orchestration(t *testing.T) {
	md := claude.GenerateCompactAgentMd(testAgent(), testResolvedSkills())
	assert.Contains(t, md, "Execute 2 skills in order.")
}

func TestGenerateCompactAgentMd_NoSkills(t *testing.T) {
	agent := testAgent()
	md := claude.GenerateCompactAgentMd(agent, nil)
	assert.NotContains(t, md, "tools:")
	assert.NotContains(t, md, "## Output")
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

func TestGenerateAgentMd_Staged_PipelineTable(t *testing.T) {
	md := claude.GenerateAgentMd(testStagedAgent(), testStagedResolvedSkills(), ".claude")
	assert.Contains(t, md, "## Pipeline")
	assert.Contains(t, md, "| Stage | Strategy | Skills |")
	assert.Contains(t, md, "| preflight | sequential | eligibility-checker, summarizer |")
	assert.Contains(t, md, "| analysis | parallel | bug-scanner, history-reviewer |")
	assert.Contains(t, md, "| publish | sequential | commenter |")
}

func TestGenerateAgentMd_Staged_FlatSteps(t *testing.T) {
	md := claude.GenerateAgentMd(testStagedAgent(), testStagedResolvedSkills(), ".claude")
	assert.Contains(t, md, "### Step 1: Eligibility Checker")
	assert.Contains(t, md, "### Step 2: Summarizer")
	assert.Contains(t, md, "### Step 3: Bug Scanner")
	assert.Contains(t, md, "### Step 5: Commenter")
	// h3, not h4
	assert.NotContains(t, md, "#### Step")
}

func TestGenerateAgentMd_Staged_NoOrchestrationDirectives(t *testing.T) {
	md := claude.GenerateAgentMd(testStagedAgent(), testStagedResolvedSkills(), ".claude")
	assert.NotContains(t, md, "Launch these subagents in parallel")
	assert.NotContains(t, md, "### Stage:")
	assert.NotContains(t, md, "stages sequentially")
	assert.NotContains(t, md, "across 3 stages")
}

func TestGenerateAgentMd_Staged_Frontmatter(t *testing.T) {
	md := claude.GenerateAgentMd(testStagedAgent(), testStagedResolvedSkills(), ".claude")
	assert.Contains(t, md, "name: code-reviewer")
	assert.Contains(t, md, "tools: Task")
}

func TestGenerateAgentMd_Staged_OutputSection(t *testing.T) {
	md := claude.GenerateAgentMd(testStagedAgent(), testStagedResolvedSkills(), ".claude")
	assert.Contains(t, md, "## Output")
	assert.Contains(t, md, "review_comment")
}

func TestGenerateAgentMd_Staged_SkillPaths(t *testing.T) {
	md := claude.GenerateAgentMd(testStagedAgent(), testStagedResolvedSkills(), ".claude")
	assert.Contains(t, md, "Skill: `.claude/skills/eligibility-checker/SKILL.md`")
	assert.Contains(t, md, "Skill: `.claude/skills/commenter/SKILL.md`")
}

func TestGenerateAgentMd_Staged_SubagentModel(t *testing.T) {
	md := claude.GenerateAgentMd(testStagedAgent(), testStagedResolvedSkills(), ".claude")
	assert.Contains(t, md, "Model: haiku")  // light effort skills
	assert.Contains(t, md, "Model: sonnet") // medium effort skills
}
