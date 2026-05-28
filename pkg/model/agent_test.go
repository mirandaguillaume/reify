package model_test

import (
	"testing"

	"github.com/mirandaguillaume/reify/pkg/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"
)

func TestAgentCompositionYAMLParsing(t *testing.T) {
	input := `
agent: review-bot
skills:
  - code-review
  - lint
orchestration: parallel-then-merge
description: Reviews PRs with linting and code analysis
`
	var ac model.AgentComposition
	err := yaml.Unmarshal([]byte(input), &ac)
	require.NoError(t, err)

	assert.Equal(t, "review-bot", ac.Agent)
	assert.Equal(t, []string{"code-review", "lint"}, ac.Skills)
	assert.Equal(t, model.OrchestrationParallelThenMerge, ac.Orchestration)
	assert.Equal(t, "Reviews PRs with linting and code analysis", ac.Description)
}

func TestAgentCompositionOptionalDescription(t *testing.T) {
	input := `
agent: simple-bot
skills:
  - greet
orchestration: sequential
`
	var ac model.AgentComposition
	err := yaml.Unmarshal([]byte(input), &ac)
	require.NoError(t, err)

	assert.Equal(t, "simple-bot", ac.Agent)
	assert.Equal(t, []string{"greet"}, ac.Skills)
	assert.Equal(t, model.OrchestrationSequential, ac.Orchestration)
	assert.Empty(t, ac.Description)
}

func TestAgentCompositionWithStages_YAMLParsing(t *testing.T) {
	input := `agent: code-reviewer
description: Multi-stage review pipeline
consumes: [pr_url, file_tree]
produces: [review_comment]
stages:
  - name: preflight
    strategy: sequential
    skills: [eligibility-checker, summarizer]
  - name: analysis
    strategy: parallel
    skills: [bug-scanner, history-reviewer]
  - name: publish
    strategy: sequential
    skills: [commenter]
`
	var ac model.AgentComposition
	err := yaml.Unmarshal([]byte(input), &ac)
	require.NoError(t, err)

	assert.Equal(t, "code-reviewer", ac.Agent)
	assert.Empty(t, ac.Skills)
	assert.Empty(t, ac.Orchestration)
	require.Len(t, ac.Stages, 3)
	assert.Equal(t, "preflight", ac.Stages[0].Name)
	assert.Equal(t, model.OrchestrationSequential, ac.Stages[0].Strategy)
	assert.Equal(t, []string{"eligibility-checker", "summarizer"}, ac.Stages[0].Skills)
	assert.Equal(t, "analysis", ac.Stages[1].Name)
	assert.Equal(t, model.OrchestrationParallel, ac.Stages[1].Strategy)
	assert.Equal(t, "publish", ac.Stages[2].Name)
}

func TestAgentCompositionLegacyFlat_StillWorks(t *testing.T) {
	input := `agent: simple-bot
skills: [lint, test]
orchestration: sequential
`
	var ac model.AgentComposition
	err := yaml.Unmarshal([]byte(input), &ac)
	require.NoError(t, err)

	assert.Equal(t, []string{"lint", "test"}, ac.Skills)
	assert.Empty(t, ac.Stages)
}

func TestAgentComposition_AllSkills_Flat(t *testing.T) {
	ac := model.AgentComposition{Skills: []string{"a", "b"}}
	assert.Equal(t, []string{"a", "b"}, ac.AllSkills())
}

func TestAgentComposition_AllSkills_Staged(t *testing.T) {
	ac := model.AgentComposition{
		Stages: []model.Stage{
			{Name: "s1", Skills: []string{"a", "b"}},
			{Name: "s2", Skills: []string{"c"}},
		},
	}
	assert.Equal(t, []string{"a", "b", "c"}, ac.AllSkills())
}

func TestAgentComposition_AllSkills_Empty(t *testing.T) {
	ac := model.AgentComposition{}
	assert.Empty(t, ac.AllSkills())
}

func TestOrchestrationStrategyConstants(t *testing.T) {
	assert.Equal(t, model.OrchestrationStrategy("sequential"), model.OrchestrationSequential)
	assert.Equal(t, model.OrchestrationStrategy("parallel"), model.OrchestrationParallel)
	assert.Equal(t, model.OrchestrationStrategy("parallel-then-merge"), model.OrchestrationParallelThenMerge)
	assert.Equal(t, model.OrchestrationStrategy("adaptive"), model.OrchestrationAdaptive)
}
