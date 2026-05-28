package model_test

import (
	"testing"

	"github.com/mirandaguillaume/reify/pkg/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"
)

func TestSkillBehaviorYAMLParsing(t *testing.T) {
	input := `
skill: code-review
version: "1.0"
context:
  consumes:
    - pull-request
    - diff
  produces:
    - review-comment
  memory: conversation
strategy:
  tools:
    - github-api
    - linter
  approach: systematic
  steps:
    - read diff
    - check style
    - post comments
guardrails:
  - never approve without tests
  - max_comments: 10
observability:
  trace_level: standard
  metrics:
    - review_time
    - comments_count
security:
  filesystem: read-only
  network: allowlist
  secrets:
    - GITHUB_TOKEN
  sandbox: container
negotiation:
  file_conflicts: merge
  priority: 5
`
	var sb model.SkillBehavior
	err := yaml.Unmarshal([]byte(input), &sb)
	require.NoError(t, err)

	assert.Equal(t, "code-review", sb.Skill)
	assert.Equal(t, "1.0", sb.Version)

	// Context
	assert.Equal(t, []string{"pull-request", "diff"}, sb.Context.Consumes)
	assert.Equal(t, []string{"review-comment"}, sb.Context.Produces)
	assert.Equal(t, model.MemoryConversation, sb.Context.Memory)

	// Strategy
	assert.Equal(t, []string{"github-api", "linter"}, sb.Strategy.Tools)
	assert.Equal(t, "systematic", sb.Strategy.Approach)
	assert.Equal(t, []string{"read diff", "check style", "post comments"}, sb.Strategy.Steps)

	// Guardrails
	require.Len(t, sb.Guardrails, 2)

	// Observability
	assert.Equal(t, model.TraceLevelStandard, sb.Observability.TraceLevel)
	assert.Equal(t, []string{"review_time", "comments_count"}, sb.Observability.Metrics)

	// Security
	assert.Equal(t, model.AccessReadOnly, sb.Security.Filesystem)
	assert.Equal(t, model.NetworkAllowlist, sb.Security.Network)
	assert.Equal(t, []string{"GITHUB_TOKEN"}, sb.Security.Secrets)
	assert.Equal(t, model.SandboxContainer, sb.Security.Sandbox)

	// Negotiation
	assert.Equal(t, model.NegotiationMerge, sb.Negotiation.FileConflicts)
	assert.Equal(t, 5, sb.Negotiation.Priority)
}

func TestGuardrailRuleString(t *testing.T) {
	input := `- never approve without tests`
	var rules []model.GuardrailRule
	err := yaml.Unmarshal([]byte(input), &rules)
	require.NoError(t, err)
	require.Len(t, rules, 1)

	val, ok := rules[0].StringValue()
	assert.True(t, ok)
	assert.Equal(t, "never approve without tests", val)

	_, ok = rules[0].MapValue()
	assert.False(t, ok)

	assert.True(t, rules[0].ContainsString("approve"))
	assert.False(t, rules[0].ContainsString("xyz"))
}

func TestGuardrailRuleMap(t *testing.T) {
	input := `- max_comments: 10`
	var rules []model.GuardrailRule
	err := yaml.Unmarshal([]byte(input), &rules)
	require.NoError(t, err)
	require.Len(t, rules, 1)

	_, ok := rules[0].StringValue()
	assert.False(t, ok)

	m, ok := rules[0].MapValue()
	assert.True(t, ok)
	assert.Equal(t, 10, m["max_comments"])

	assert.True(t, rules[0].HasKey("max_comments"))
	assert.False(t, rules[0].HasKey("xyz"))
}

func TestMemoryTypeConstants(t *testing.T) {
	assert.Equal(t, model.MemoryType("short-term"), model.MemoryShortTerm)
	assert.Equal(t, model.MemoryType("conversation"), model.MemoryConversation)
	assert.Equal(t, model.MemoryType("long-term"), model.MemoryLongTerm)
}

func TestTraceLevelConstants(t *testing.T) {
	assert.Equal(t, model.TraceLevel("minimal"), model.TraceLevelMinimal)
	assert.Equal(t, model.TraceLevel("standard"), model.TraceLevelStandard)
	assert.Equal(t, model.TraceLevel("detailed"), model.TraceLevelDetailed)
}

func TestAccessLevelConstants(t *testing.T) {
	assert.Equal(t, model.AccessLevel("none"), model.AccessNone)
	assert.Equal(t, model.AccessLevel("read-only"), model.AccessReadOnly)
	assert.Equal(t, model.AccessLevel("read-write"), model.AccessReadWrite)
	assert.Equal(t, model.AccessLevel("full"), model.AccessFull)
}

func TestNetworkAccessConstants(t *testing.T) {
	assert.Equal(t, model.NetworkAccess("none"), model.NetworkNone)
	assert.Equal(t, model.NetworkAccess("allowlist"), model.NetworkAllowlist)
	assert.Equal(t, model.NetworkAccess("full"), model.NetworkFull)
}

func TestNegotiationStrategyConstants(t *testing.T) {
	assert.Equal(t, model.NegotiationStrategy("yield"), model.NegotiationYield)
	assert.Equal(t, model.NegotiationStrategy("override"), model.NegotiationOverride)
	assert.Equal(t, model.NegotiationStrategy("merge"), model.NegotiationMerge)
}

func TestWhenToUseFacetParsing(t *testing.T) {
	input := `
skill: debug
version: "1.0"
context:
  consumes: []
  produces: []
  memory: short-term
strategy:
  tools: []
  approach: sequential
guardrails: []

observability:
  trace_level: minimal
  metrics: []
security:
  filesystem: none
  network: none
  secrets: []
negotiation:
  file_conflicts: yield
  priority: 0
when_to_use:
  triggers:
    - "Test failures"
    - "Bug reports"
  dont_use:
    - "Simple typo fixes"
  especially:
    - "After 3+ failed attempts"
`
	var sb model.SkillBehavior
	err := yaml.Unmarshal([]byte(input), &sb)
	require.NoError(t, err)

	assert.Equal(t, []string{"Test failures", "Bug reports"}, sb.WhenToUse.Triggers)
	assert.Equal(t, []string{"Simple typo fixes"}, sb.WhenToUse.DontUse)
	assert.Equal(t, []string{"After 3+ failed attempts"}, sb.WhenToUse.Especially)
	assert.False(t, sb.WhenToUse.IsEmpty())
}

func TestWhenToUseFacetEmpty(t *testing.T) {
	w := model.WhenToUseFacet{}
	assert.True(t, w.IsEmpty())
}

// --- Mutation-killing tests for WhenToUseFacet.IsEmpty (line 168) ---

func TestWhenToUseFacet_OnlyTriggers(t *testing.T) {
	// Mutation: removing `len(w.Triggers) == 0` from the AND chain would make this return true.
	w := model.WhenToUseFacet{Triggers: []string{"bug fix"}}
	assert.False(t, w.IsEmpty(), "having only Triggers should not be empty")
}

func TestWhenToUseFacet_OnlyDontUse(t *testing.T) {
	// Mutation: removing `len(w.DontUse) == 0` from the AND chain would make this return true.
	w := model.WhenToUseFacet{DontUse: []string{"trivial changes"}}
	assert.False(t, w.IsEmpty(), "having only DontUse should not be empty")
}

func TestWhenToUseFacet_OnlyEspecially(t *testing.T) {
	// Mutation: removing `len(w.Especially) == 0` from the AND chain would make this return true.
	w := model.WhenToUseFacet{Especially: []string{"complex refactors"}}
	assert.False(t, w.IsEmpty(), "having only Especially should not be empty")
}

func TestWhenToUseFacet_AllEmpty(t *testing.T) {
	w := model.WhenToUseFacet{
		Triggers:   []string{},
		DontUse:    []string{},
		Especially: []string{},
	}
	assert.True(t, w.IsEmpty(), "all empty slices should be empty")
}

func TestWhenToUseFacet_AllNil(t *testing.T) {
	w := model.WhenToUseFacet{
		Triggers:   nil,
		DontUse:    nil,
		Especially: nil,
	}
	assert.True(t, w.IsEmpty(), "all nil slices should be empty")
}

func TestWhenToUseFacet_TwoOfThreePopulated(t *testing.T) {
	w := model.WhenToUseFacet{
		Triggers: []string{"x"},
		DontUse:  []string{"y"},
	}
	assert.False(t, w.IsEmpty())
}

func TestWhenToUseFacet_AllPopulated(t *testing.T) {
	w := model.WhenToUseFacet{
		Triggers:   []string{"a"},
		DontUse:    []string{"b"},
		Especially: []string{"c"},
	}
	assert.False(t, w.IsEmpty())
}

func TestAntiPatternsParsing(t *testing.T) {
	input := `
skill: debug
version: "1.0"
context:
  consumes: []
  produces: []
  memory: short-term
strategy:
  tools: []
  approach: sequential
guardrails: []

observability:
  trace_level: minimal
  metrics: []
security:
  filesystem: none
  network: none
  secrets: []
negotiation:
  file_conflicts: yield
  priority: 0
anti_patterns:
  - excuse: "Quick fix for now"
    reality: "First fix sets the pattern"
  - excuse: "It works on my machine"
    reality: "Test in CI"
`
	var sb model.SkillBehavior
	err := yaml.Unmarshal([]byte(input), &sb)
	require.NoError(t, err)

	require.Len(t, sb.AntiPatterns, 2)
	assert.Equal(t, "Quick fix for now", sb.AntiPatterns[0].Excuse)
	assert.Equal(t, "First fix sets the pattern", sb.AntiPatterns[0].Reality)
}

func TestCodeExamplesParsing(t *testing.T) {
	input := `
skill: debug
version: "1.0"
context:
  consumes: []
  produces: []
  memory: short-term
strategy:
  tools: []
  approach: sequential
guardrails: []

observability:
  trace_level: minimal
  metrics: []
security:
  filesystem: none
  network: none
  secrets: []
negotiation:
  file_conflicts: yield
  priority: 0
examples:
  - label: "Good: explicit verification"
    code: "go test ./..."
    lang: bash
  - label: "Bad: skip tests"
    code: "git push --no-verify"
`
	var sb model.SkillBehavior
	err := yaml.Unmarshal([]byte(input), &sb)
	require.NoError(t, err)

	require.Len(t, sb.Examples, 2)
	assert.Equal(t, "Good: explicit verification", sb.Examples[0].Label)
	assert.Equal(t, "go test ./...", sb.Examples[0].Code)
	assert.Equal(t, "bash", sb.Examples[0].Lang)
	assert.Equal(t, "", sb.Examples[1].Lang)
}
