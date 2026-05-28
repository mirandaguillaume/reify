package linter

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"

	"github.com/mirandaguillaume/reify/pkg/model"
)

// findResult returns the first LintResult matching the given rule name, or nil.
func findResult(results []LintResult, ruleName string) *LintResult {
	for i := range results {
		if results[i].Rule == ruleName {
			return &results[i]
		}
	}
	return nil
}

// minimalSkill returns a valid skill with tools and no guardrails.
func minimalSkill() model.SkillBehavior {
	return model.SkillBehavior{
		Skill:   "test-skill",
		Version: "0.1.0",
		Context: model.ContextFacet{
			Produces: []string{"output"},
			Memory:   model.MemoryShortTerm,
		},
		Strategy: model.StrategyFacet{
			Tools:    []string{"Read"},
			Approach: "sequential",
		},
		Guardrails: nil,
		Observability: model.ObservabilityFacet{
			TraceLevel: model.TraceLevelMinimal,
		},
		Security: model.SecurityFacet{
			Filesystem: model.AccessNone,
			Network:    model.NetworkNone,
		},
		Negotiation: model.NegotiationFacet{
			FileConflicts: model.NegotiationYield,
		},
	}
}

// parseGuardrails parses a YAML list of guardrail rules for test use.
func parseGuardrails(t *testing.T, yamlContent string) []model.GuardrailRule {
	t.Helper()
	var rules []model.GuardrailRule
	err := yaml.Unmarshal([]byte(yamlContent), &rules)
	require.NoError(t, err)
	return rules
}

func TestNoEmptyTools_NoTools_Warning(t *testing.T) {
	skill := minimalSkill()
	skill.Strategy.Tools = nil

	results := LintSkill(skill)
	result := findResult(results, "no-empty-tools")

	assert.NotNil(t, result)
	assert.Equal(t, SeverityWarning, result.Severity)
	assert.Equal(t, "strategy", result.Facet)
	assert.Contains(t, result.Message, "test-skill")
}

func TestNoEmptyTools_WithTools_NoIssue(t *testing.T) {
	skill := minimalSkill()

	results := LintSkill(skill)
	result := findResult(results, "no-empty-tools")

	assert.Nil(t, result)
}

func TestHasGuardrails_NoGuardrails_Warning(t *testing.T) {
	skill := minimalSkill()

	results := LintSkill(skill)
	result := findResult(results, "has-guardrails")

	assert.NotNil(t, result)
	assert.Equal(t, SeverityWarning, result.Severity)
	assert.Equal(t, "guardrails", result.Facet)
	assert.Contains(t, result.Message, "test-skill")
}

func TestObservableOutputs_ProducesNoMetrics_Warning(t *testing.T) {
	skill := minimalSkill()
	skill.Context.Produces = []string{"report.md"}
	skill.Observability.Metrics = nil

	results := LintSkill(skill)
	result := findResult(results, "observable-outputs")

	assert.NotNil(t, result)
	assert.Equal(t, SeverityWarning, result.Severity)
	assert.Equal(t, "observability", result.Facet)
	assert.Contains(t, result.Message, "test-skill")
}

func TestObservableOutputs_NoProduces_NoIssue(t *testing.T) {
	skill := minimalSkill()
	skill.Context.Produces = nil
	skill.Observability.Metrics = nil

	results := LintSkill(skill)
	result := findResult(results, "observable-outputs")

	assert.Nil(t, result)
}

func TestSecurityNeedsGuardrails_FullAccess_NoGuardrails_Error(t *testing.T) {
	skill := minimalSkill()
	skill.Security.Filesystem = model.AccessFull
	skill.Guardrails = nil

	results := LintSkill(skill)
	result := findResult(results, "security-needs-guardrails")

	assert.NotNil(t, result)
	assert.Equal(t, SeverityError, result.Severity)
	assert.Equal(t, "security", result.Facet)
	assert.Contains(t, result.Message, "full")
}

func TestSecurityNeedsGuardrails_ReadWrite_NoGuardrails_Error(t *testing.T) {
	skill := minimalSkill()
	skill.Security.Filesystem = model.AccessReadWrite
	skill.Guardrails = nil

	results := LintSkill(skill)
	result := findResult(results, "security-needs-guardrails")

	assert.NotNil(t, result)
	assert.Equal(t, SeverityError, result.Severity)
	assert.Contains(t, result.Message, "read-write")
}

func TestSecurityNeedsGuardrails_FullAccess_WithGuardrails_NoIssue(t *testing.T) {
	skill := minimalSkill()
	skill.Security.Filesystem = model.AccessFull
	skill.Guardrails = parseGuardrails(t, `- "timeout: 30s"`)

	results := LintSkill(skill)
	result := findResult(results, "security-needs-guardrails")

	assert.Nil(t, result)
}

func TestHasWhenToUse_NoWhenToUse_Info(t *testing.T) {
	skill := minimalSkill()

	results := LintSkill(skill)
	result := findResult(results, "has-when-to-use")

	assert.NotNil(t, result)
	assert.Equal(t, SeverityInfo, result.Severity)
	assert.Equal(t, "when_to_use", result.Facet)
}

func TestHasWhenToUse_WithTriggers_NoIssue(t *testing.T) {
	skill := minimalSkill()
	skill.WhenToUse = model.WhenToUseFacet{Triggers: []string{"bug"}}

	results := LintSkill(skill)
	result := findResult(results, "has-when-to-use")

	assert.Nil(t, result)
}

func TestSingleProducesOutput_ZeroProduces_Error(t *testing.T) {
	skill := minimalSkill()
	skill.Context.Produces = nil

	results := LintSkill(skill)
	result := findResult(results, "single-produces-output")

	assert.NotNil(t, result)
	assert.Equal(t, SeverityError, result.Severity)
	assert.Equal(t, "context", result.Facet)
	assert.Contains(t, result.Message, "test-skill")
	assert.Contains(t, result.Message, "0")
}

func TestSingleProducesOutput_MultipleProduces_Error(t *testing.T) {
	skill := minimalSkill()
	skill.Context.Produces = []string{"output1", "output2"}

	results := LintSkill(skill)
	result := findResult(results, "single-produces-output")

	assert.NotNil(t, result)
	assert.Equal(t, SeverityError, result.Severity)
	assert.Contains(t, result.Message, "test-skill")
	assert.Contains(t, result.Message, "2")
}

func TestSingleProducesOutput_OneProduces_NoIssue(t *testing.T) {
	skill := minimalSkill()

	results := LintSkill(skill)
	result := findResult(results, "single-produces-output")

	assert.Nil(t, result)
}

func TestProducesMatchesDescription_ConjunctionAnd_Error(t *testing.T) {
	skill := minimalSkill()
	skill.Strategy.Approach = "parse the file and generate output"

	results := LintSkill(skill)
	result := findResult(results, "produces-matches-description")

	assert.NotNil(t, result)
	assert.Equal(t, SeverityError, result.Severity)
	assert.Equal(t, "strategy", result.Facet)
	assert.Contains(t, result.Message, " and ")
}

func TestProducesMatchesDescription_ConjunctionThen_Error(t *testing.T) {
	skill := minimalSkill()
	skill.Strategy.Approach = "scan the code then report findings"

	results := LintSkill(skill)
	result := findResult(results, "produces-matches-description")

	assert.NotNil(t, result)
	assert.Equal(t, "produces-matches-description", result.Rule)
	assert.Contains(t, result.Message, " then ")
}

func TestProducesMatchesDescription_ConjunctionAmpersand_Error(t *testing.T) {
	skill := minimalSkill()
	skill.Strategy.Approach = "lint & format the code"

	results := LintSkill(skill)
	result := findResult(results, "produces-matches-description")

	assert.NotNil(t, result)
	assert.Equal(t, "produces-matches-description", result.Rule)
	assert.Contains(t, result.Message, " & ")
}

func TestProducesMatchesDescription_CaseInsensitive_Error(t *testing.T) {
	skill := minimalSkill()
	skill.Strategy.Approach = "Parse the file AND generate output"

	results := LintSkill(skill)
	result := findResult(results, "produces-matches-description")

	assert.NotNil(t, result)
	assert.Equal(t, "produces-matches-description", result.Rule)
}

func TestProducesMatchesDescription_NoConjunction_NoIssue(t *testing.T) {
	skill := minimalSkill()
	skill.Strategy.Approach = "sequential processing of inputs"

	results := LintSkill(skill)
	result := findResult(results, "produces-matches-description")

	assert.Nil(t, result)
}

func TestProducesMatchesDescription_CompoundObject_NoIssue(t *testing.T) {
	skill := minimalSkill()
	skill.Strategy.Approach = "analyze code complexity and maintainability"

	results := LintSkill(skill)
	result := findResult(results, "produces-matches-description")

	assert.Nil(t, result, "compound noun objects joined by 'and' should not flag")
}

func TestSkillNameMatchesOutput_AndPattern_Error(t *testing.T) {
	skill := minimalSkill()
	skill.Skill = "lint-and-format"

	results := LintSkill(skill)
	result := findResult(results, "skill-name-matches-output")

	assert.NotNil(t, result)
	assert.Equal(t, SeverityError, result.Severity)
	assert.Equal(t, "context", result.Facet)
	assert.Contains(t, result.Message, "-and-")
}

func TestSkillNameMatchesOutput_ThenPattern_Error(t *testing.T) {
	skill := minimalSkill()
	skill.Skill = "scan-then-report"

	results := LintSkill(skill)
	result := findResult(results, "skill-name-matches-output")

	assert.NotNil(t, result)
	assert.Equal(t, "skill-name-matches-output", result.Rule)
	assert.Contains(t, result.Message, "-then-")
}

func TestSkillNameMatchesOutput_AmpersandPattern_Error(t *testing.T) {
	skill := minimalSkill()
	skill.Skill = "lint-&-format"

	results := LintSkill(skill)
	result := findResult(results, "skill-name-matches-output")

	assert.NotNil(t, result)
	assert.Equal(t, "skill-name-matches-output", result.Rule)
	assert.Contains(t, result.Message, "-&-")
}

func TestSkillNameMatchesOutput_CleanName_NoIssue(t *testing.T) {
	skill := minimalSkill()

	results := LintSkill(skill)
	result := findResult(results, "skill-name-matches-output")

	assert.Nil(t, result)
}

// --- steps-conjunction-check ---

func TestStepsConjunctionCheck_StepWithAnd_Error(t *testing.T) {
	skill := minimalSkill()
	skill.Strategy.Steps = []string{"Read issue and search codebase"}

	results := LintSkill(skill)
	result := findResult(results, "steps-conjunction-check")

	assert.NotNil(t, result)
	assert.Equal(t, SeverityError, result.Severity)
	assert.Equal(t, "strategy", result.Facet)
	assert.Contains(t, result.Message, " and ")
}

func TestStepsConjunctionCheck_StepWithThen_Error(t *testing.T) {
	skill := minimalSkill()
	skill.Strategy.Steps = []string{"Parse data then validate output"}

	results := LintSkill(skill)
	result := findResult(results, "steps-conjunction-check")

	assert.NotNil(t, result)
	assert.Contains(t, result.Message, " then ")
}

func TestStepsConjunctionCheck_CaseInsensitive_Error(t *testing.T) {
	skill := minimalSkill()
	skill.Strategy.Steps = []string{"Read the file AND extract metadata"}

	results := LintSkill(skill)
	result := findResult(results, "steps-conjunction-check")

	assert.NotNil(t, result)
}

func TestStepsConjunctionCheck_CleanSteps_NoError(t *testing.T) {
	skill := minimalSkill()
	skill.Strategy.Steps = []string{"Read the issue description", "Locate affected files", "Identify the root cause"}

	results := LintSkill(skill)
	result := findResult(results, "steps-conjunction-check")

	assert.Nil(t, result)
}

func TestStepsConjunctionCheck_CompoundObject_NoError(t *testing.T) {
	skill := minimalSkill()
	skill.Strategy.Steps = []string{
		"Search codebase to locate relevant files and functions",
		"Analyze code readability and naming conventions",
		"Check authentication and authorization patterns",
	}

	results := LintSkill(skill)
	result := findResult(results, "steps-conjunction-check")

	assert.Nil(t, result, "compound noun objects joined by 'and' should not flag")
}

func TestStepsConjunctionCheck_VerbAndVerb_Error(t *testing.T) {
	skill := minimalSkill()
	skill.Strategy.Steps = []string{"Scan codebase and validate the results"}

	results := LintSkill(skill)
	result := findResult(results, "steps-conjunction-check")

	assert.NotNil(t, result, "verb + and + verb should flag")
	assert.Contains(t, result.Message, " and ")
}

func TestStepsConjunctionCheck_NoSteps_NoError(t *testing.T) {
	skill := minimalSkill()
	skill.Strategy.Steps = nil

	results := LintSkill(skill)
	result := findResult(results, "steps-conjunction-check")

	assert.Nil(t, result)
}

// --- max-steps-count ---

func TestMaxStepsCount_6Steps_Warning(t *testing.T) {
	skill := minimalSkill()
	skill.Strategy.Steps = []string{"s1", "s2", "s3", "s4", "s5", "s6"}

	results := LintSkill(skill)
	result := findResult(results, "max-steps-count")

	assert.NotNil(t, result)
	assert.Equal(t, SeverityWarning, result.Severity)
	assert.Equal(t, "strategy", result.Facet)
	assert.Contains(t, result.Message, "6 steps")
}

func TestMaxStepsCount_5Steps_NoIssue(t *testing.T) {
	skill := minimalSkill()
	skill.Strategy.Steps = []string{"s1", "s2", "s3", "s4", "s5"}

	results := LintSkill(skill)
	result := findResult(results, "max-steps-count")

	assert.Nil(t, result)
}

func TestMaxStepsCount_0Steps_NoIssue(t *testing.T) {
	skill := minimalSkill()
	skill.Strategy.Steps = nil

	results := LintSkill(skill)
	result := findResult(results, "max-steps-count")

	assert.Nil(t, result)
}

func TestLintSkill_CleanSkill_EmptyResults(t *testing.T) {
	skill := minimalSkill()
	skill.Guardrails = parseGuardrails(t, `- "timeout: 30s"`)
	skill.Observability.Metrics = []string{"latency"}
	skill.WhenToUse = model.WhenToUseFacet{Triggers: []string{"test"}}

	results := LintSkill(skill)

	assert.Empty(t, results)
}
