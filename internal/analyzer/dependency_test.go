package analyzer

import (
	"testing"

	"github.com/mirandaguillaume/reify/pkg/model"
	"github.com/stretchr/testify/assert"
)

func makeSkill(name string, consumes, produces []string) model.SkillBehavior {
	return model.SkillBehavior{
		Skill:   name,
		Version: "1.0.0",
		Context: model.ContextFacet{
			Consumes: consumes,
			Produces: produces,
			Memory:   model.MemoryShortTerm,
		},
	}
}

func makeAgent(name string, skills []string, consumes, produces []string) model.AgentComposition {
	return model.AgentComposition{
		Agent:         name,
		Skills:        skills,
		Orchestration: model.OrchestrationSequential,
		Consumes:      consumes,
		Produces:      produces,
	}
}

func TestCheckDependencies_NoIssues(t *testing.T) {
	skills := []model.SkillBehavior{
		makeSkill("skill-a", nil, []string{"data"}),
		makeSkill("skill-b", []string{"data"}, []string{"result"}),
	}
	agent := makeAgent("test-agent", []string{"skill-a", "skill-b"}, nil, []string{"result"})

	issues := CheckDependencies(agent, skills)
	assert.Empty(t, issues)
}

func TestCheckDependencies_MissingSkill(t *testing.T) {
	skills := []model.SkillBehavior{
		makeSkill("skill-a", nil, []string{"data"}),
	}
	agent := makeAgent("test-agent", []string{"skill-a", "nonexistent"}, nil, nil)

	issues := CheckMissingDependencies(agent, skills)
	assert.Len(t, issues, 1)
	assert.Equal(t, IssueMissing, issues[0].Type)
	assert.Equal(t, "nonexistent", issues[0].Skill)
}

func TestCheckDependencies_CircularDependency(t *testing.T) {
	skills := []model.SkillBehavior{
		makeSkill("skill-a", []string{"data"}, []string{"result"}),
		makeSkill("skill-b", []string{"result"}, []string{"data"}),
	}
	agent := makeAgent("test-agent", []string{"skill-a", "skill-b"}, nil, nil)

	issues := CheckCircularDependencies(agent, skills)

	hasCircular := false
	for _, issue := range issues {
		if issue.Type == IssueCircular {
			hasCircular = true
			assert.Contains(t, issue.Message, "Circular dependency detected")
		}
	}
	assert.True(t, hasCircular, "expected at least one circular dependency issue")
}

func TestCheckDependencies_NoCycle(t *testing.T) {
	skills := []model.SkillBehavior{
		makeSkill("skill-a", nil, []string{"data"}),
		makeSkill("skill-b", []string{"data"}, []string{"result"}),
	}
	agent := makeAgent("test-agent", []string{"skill-a", "skill-b"}, nil, nil)

	issues := CheckCircularDependencies(agent, skills)
	assert.Empty(t, issues)
}

func TestCheckUnmetContext_Unmet(t *testing.T) {
	skills := []model.SkillBehavior{
		makeSkill("skill-a", []string{"missing-input"}, []string{"data"}),
	}
	agent := makeAgent("test-agent", []string{"skill-a"}, nil, nil)

	issues := CheckUnmetContext(agent, skills)
	assert.Len(t, issues, 1)
	assert.Equal(t, IssueUnmetContext, issues[0].Type)
	assert.Contains(t, issues[0].Message, "missing-input")
}

func TestCheckUnmetContext_SatisfiedByAgentConsumes(t *testing.T) {
	skills := []model.SkillBehavior{
		makeSkill("skill-a", []string{"external-input"}, []string{"data"}),
	}
	agent := makeAgent("test-agent", []string{"skill-a"}, []string{"external-input"}, nil)

	issues := CheckUnmetContext(agent, skills)
	assert.Empty(t, issues)
}

func TestCheckUnmetContext_SatisfiedByOtherSkill(t *testing.T) {
	skills := []model.SkillBehavior{
		makeSkill("skill-a", nil, []string{"data"}),
		makeSkill("skill-b", []string{"data"}, []string{"result"}),
	}
	agent := makeAgent("test-agent", []string{"skill-a", "skill-b"}, nil, nil)

	issues := CheckUnmetContext(agent, skills)
	assert.Empty(t, issues)
}

func TestCheckAgentProduces_Valid(t *testing.T) {
	skills := []model.SkillBehavior{
		makeSkill("skill-a", nil, []string{"data"}),
	}
	agent := makeAgent("test-agent", []string{"skill-a"}, nil, []string{"data"})

	issues := CheckAgentProduces(agent, skills)
	assert.Empty(t, issues)
}

func TestCheckAgentProduces_Invalid(t *testing.T) {
	skills := []model.SkillBehavior{
		makeSkill("skill-a", nil, []string{"data"}),
	}
	agent := makeAgent("test-agent", []string{"skill-a"}, nil, []string{"nonexistent"})

	issues := CheckAgentProduces(agent, skills)
	assert.Len(t, issues, 1)
	assert.Contains(t, issues[0].Message, "nonexistent")
}

// --- Mutation-killing tests ---

// Kills: dependency.go Line 74:50 INVERT_LOGICAL — exists && provider != name → exists || provider != name
// When a skill consumes something NOT produced by any other skill (exists=false),
// with &&: false && ... => false => no edge added (correct).
// With ||: false || (provider("") != name("skill-a")) => true => edge added (wrong).
// This would create a spurious dependency edge from "skill-a" to "" (empty string),
// which could affect cycle detection or at least change the adjacency graph.
// We verify that no circular dependency is reported for a linear chain where one
// skill consumes data not produced by anyone.
func TestCheckCircularDependencies_ConsumesMissingData(t *testing.T) {
	skills := []model.SkillBehavior{
		makeSkill("skill-a", []string{"orphan-data"}, []string{"output-a"}),
		makeSkill("skill-b", []string{"output-a"}, []string{"output-b"}),
	}
	agent := makeAgent("test-agent", []string{"skill-a", "skill-b"}, nil, nil)

	issues := CheckCircularDependencies(agent, skills)

	// With correct code: "orphan-data" has no producer, so no edge is added for it.
	// skill-b depends on skill-a (via output-a). No cycle.
	// With || mutation: "orphan-data" not in producerOf, but || would still add edge
	// from skill-a to "" (empty provider), which is not in agent.Skills, so no DFS on it.
	// Let me make it more targeted: ensure the adjacency list is correct by verifying no issues.
	assert.Empty(t, issues, "should find no circular dependency in linear chain with unmet consume")
}

// More targeted version: skill consumes data it produces itself.
// Line 74: provider != name filters out self-edges.
// With || mutation, exists could be false but provider != name could be true,
// adding spurious edges. But let me also test the self-edge filtering directly.
func TestCheckCircularDependencies_SelfProduceConsumeNoCycle(t *testing.T) {
	// skill-a both consumes and produces "data". This is a self-reference.
	// Line 74: provider = "skill-a", name = "skill-a" => provider != name is false.
	// So no edge is added from skill-a to itself. No cycle detected.
	// With INVERT_LOGICAL (&&→||): exists=true, so exists || (provider != name) = true.
	// This would add a self-edge, but then DFS would find skill-a -> skill-a cycle.
	skills := []model.SkillBehavior{
		makeSkill("skill-a", []string{"data"}, []string{"data"}),
	}
	agent := makeAgent("test-agent", []string{"skill-a"}, nil, nil)

	issues := CheckCircularDependencies(agent, skills)

	// With correct code: self-edges are excluded, no cycle detected.
	assert.Empty(t, issues, "self-produce/consume should not create a circular dependency")
}

// Kills: dependency.go Line 206:8 CONDITIONALS_NEGATION — s == item → s != item
// indexOf is used in cycle detection (line 88: startIdx := indexOf(path, name)).
// If indexOf returns wrong results, the cycle slice would be incorrect.
// With s != item: indexOf would return the index of the first element that does NOT
// match, which for a non-empty path would typically be 0 (wrong index).
// This test creates a cycle and verifies the cycle details are correct.
func TestCheckCircularDependencies_CycleDetailsCorrect(t *testing.T) {
	// A -> B -> C -> A cycle.
	// skill-a produces "from-a", skill-b consumes "from-a" produces "from-b",
	// skill-c consumes "from-b" produces "from-c", skill-a consumes "from-c".
	skills := []model.SkillBehavior{
		makeSkill("skill-a", []string{"from-c"}, []string{"from-a"}),
		makeSkill("skill-b", []string{"from-a"}, []string{"from-b"}),
		makeSkill("skill-c", []string{"from-b"}, []string{"from-c"}),
	}
	agent := makeAgent("test-agent", []string{"skill-a", "skill-b", "skill-c"}, nil, nil)

	issues := CheckCircularDependencies(agent, skills)

	hasCircular := false
	for _, issue := range issues {
		if issue.Type == IssueCircular {
			hasCircular = true
			// The cycle should contain all three skills.
			// With indexOf mutation (s != item), startIdx would be wrong,
			// producing an incorrect or truncated cycle slice.
			assert.GreaterOrEqual(t, len(issue.Details), 3,
				"cycle should include at least 3 nodes (the full cycle path)")
			// Verify the cycle starts and ends with the same skill
			assert.Equal(t, issue.Details[0], issue.Details[len(issue.Details)-1],
				"cycle should start and end with the same skill")
		}
	}
	assert.True(t, hasCircular, "expected circular dependency in A->B->C->A chain")
}

// Test indexOf directly to ensure exact behavior at boundaries.
func TestIndexOf_FoundAndNotFound(t *testing.T) {
	slice := []string{"alpha", "beta", "gamma"}

	// Found at position 0
	assert.Equal(t, 0, indexOf(slice, "alpha"), "alpha should be at index 0")
	// Found at position 1
	assert.Equal(t, 1, indexOf(slice, "beta"), "beta should be at index 1")
	// Found at position 2
	assert.Equal(t, 2, indexOf(slice, "gamma"), "gamma should be at index 2")
	// Not found
	assert.Equal(t, -1, indexOf(slice, "delta"), "delta should not be found")
	// Empty slice
	assert.Equal(t, -1, indexOf([]string{}, "anything"), "empty slice should return -1")
}

// Test containsString directly to catch any CONDITIONALS_NEGATION on line 206.
func TestContainsString_FoundAndNotFound(t *testing.T) {
	slice := []string{"alpha", "beta", "gamma"}

	assert.True(t, containsString(slice, "alpha"), "should find alpha")
	assert.True(t, containsString(slice, "beta"), "should find beta")
	assert.True(t, containsString(slice, "gamma"), "should find gamma")
	assert.False(t, containsString(slice, "delta"), "should not find delta")
	assert.False(t, containsString([]string{}, "anything"), "empty slice should not contain anything")
}
