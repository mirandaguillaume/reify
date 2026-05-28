package analyzer

import (
	"testing"

	"github.com/mirandaguillaume/reify/pkg/model"
	"github.com/stretchr/testify/assert"
	"gopkg.in/yaml.v3"
)

func makeLoopSkill(name string, consumes, produces []string, memory model.MemoryType, guardrails []model.GuardrailRule) model.SkillBehavior {
	return model.SkillBehavior{
		Skill:   name,
		Version: "1.0.0",
		Context: model.ContextFacet{
			Consumes: consumes,
			Produces: produces,
			Memory:   memory,
		},
		Guardrails: guardrails,
	}
}

func guardrailFromString(t *testing.T, s string) model.GuardrailRule {
	t.Helper()
	node := &yaml.Node{Kind: yaml.ScalarNode, Value: s, Tag: "!!str"}
	var gr model.GuardrailRule
	err := gr.UnmarshalYAML(node)
	assert.NoError(t, err)
	return gr
}

func guardrailFromMap(t *testing.T, key, value string) model.GuardrailRule {
	t.Helper()
	node := &yaml.Node{
		Kind: yaml.MappingNode,
		Tag:  "!!map",
		Content: []*yaml.Node{
			{Kind: yaml.ScalarNode, Value: key, Tag: "!!str"},
			{Kind: yaml.ScalarNode, Value: value, Tag: "!!str"},
		},
	}
	var gr model.GuardrailRule
	err := gr.UnmarshalYAML(node)
	assert.NoError(t, err)
	return gr
}

func TestDetectLoopRisks_SelfReference(t *testing.T) {
	skill := makeLoopSkill("self-ref", []string{"data", "extra"}, []string{"data"}, model.MemoryShortTerm, nil)

	risks := DetectLoopRisks(skill, &DefaultGuardrailChecker{})

	hasSelfRef := false
	for _, r := range risks {
		if r.Type == LoopSelfReference {
			hasSelfRef = true
			assert.Equal(t, "error", r.Severity)
			assert.Contains(t, r.Message, "data")
			assert.Contains(t, r.Message, "infinite loops")
		}
	}
	assert.True(t, hasSelfRef, "expected self-reference risk")
}

func TestDetectLoopRisks_NoTimeout(t *testing.T) {
	skill := makeLoopSkill("no-timeout", []string{"input"}, []string{"output"}, model.MemoryConversation, nil)

	risks := DetectLoopRisks(skill, &DefaultGuardrailChecker{})

	hasNoTimeout := false
	for _, r := range risks {
		if r.Type == LoopNoTimeout {
			hasNoTimeout = true
			assert.Equal(t, "warning", r.Severity)
			assert.Contains(t, r.Message, "conversation")
			assert.Contains(t, r.Message, "timeout")
		}
	}
	assert.True(t, hasNoTimeout, "expected no-timeout risk")
}

func TestDetectLoopRisks_CleanSkill(t *testing.T) {
	skill := makeLoopSkill("clean", []string{"input"}, []string{"output"}, model.MemoryShortTerm, nil)

	risks := DetectLoopRisks(skill, &DefaultGuardrailChecker{})
	assert.Empty(t, risks)
}

func TestDetectLoopRisks_TimeoutMapGuardrail(t *testing.T) {
	gr := guardrailFromMap(t, "timeout", "5min")
	skill := makeLoopSkill("with-timeout", []string{"input"}, []string{"output"}, model.MemoryConversation, []model.GuardrailRule{gr})

	risks := DetectLoopRisks(skill, &DefaultGuardrailChecker{})

	for _, r := range risks {
		assert.NotEqual(t, LoopNoTimeout, r.Type, "should not flag no-timeout when timeout guardrail exists")
	}
}

func TestDetectLoopRisks_TimeoutStringGuardrail(t *testing.T) {
	gr := guardrailFromString(t, "timeout: 5 minutes")
	skill := makeLoopSkill("with-timeout-str", []string{"input"}, []string{"output"}, model.MemoryConversation, []model.GuardrailRule{gr})

	risks := DetectLoopRisks(skill, &DefaultGuardrailChecker{})

	for _, r := range risks {
		assert.NotEqual(t, LoopNoTimeout, r.Type, "should not flag no-timeout when timeout string guardrail exists")
	}
}

func TestDetectLoopRisks_LongTermMemoryNoTimeout(t *testing.T) {
	skill := makeLoopSkill("long-term", []string{"input"}, []string{"output"}, model.MemoryLongTerm, nil)

	risks := DetectLoopRisks(skill, &DefaultGuardrailChecker{})

	hasNoTimeout := false
	for _, r := range risks {
		if r.Type == LoopNoTimeout {
			hasNoTimeout = true
			assert.Contains(t, r.Message, "long-term")
		}
	}
	assert.True(t, hasNoTimeout, "expected no-timeout risk for long-term memory")
}

// --- Mock checkers for DIP testing ---

type alwaysTrueChecker struct{}

func (c *alwaysTrueChecker) HasCapability(_ model.SkillBehavior, _ string) bool { return true }

type alwaysFalseChecker struct{}

func (c *alwaysFalseChecker) HasCapability(_ model.SkillBehavior, _ string) bool { return false }

func TestDetectLoopRisks_AlwaysTrueChecker_PreventsNoTimeoutRisk(t *testing.T) {
	// Skill with conversation memory and no guardrails — would normally trigger LoopNoTimeout.
	skill := makeLoopSkill("mock-ok", []string{"input"}, []string{"output"}, model.MemoryConversation, nil)

	risks := DetectLoopRisks(skill, &alwaysTrueChecker{})

	for _, r := range risks {
		assert.NotEqual(t, LoopNoTimeout, r.Type, "alwaysTrueChecker should prevent LoopNoTimeout risk")
	}
}

func TestDetectLoopRisks_AlwaysFalseChecker_TriggersNoTimeoutRisk(t *testing.T) {
	// Skill with conversation memory — alwaysFalseChecker should trigger LoopNoTimeout
	// even if the skill has guardrails, because the checker always returns false.
	gr := guardrailFromString(t, "timeout: 5 minutes")
	skill := makeLoopSkill("mock-fail", []string{"input"}, []string{"output"}, model.MemoryConversation, []model.GuardrailRule{gr})

	risks := DetectLoopRisks(skill, &alwaysFalseChecker{})

	hasNoTimeout := false
	for _, r := range risks {
		if r.Type == LoopNoTimeout {
			hasNoTimeout = true
		}
	}
	assert.True(t, hasNoTimeout, "alwaysFalseChecker should trigger LoopNoTimeout risk")
}

// --- Mutation-killing tests ---

// Kills: loop.go Line 34:35 INVERT_LOGICAL — ok && strings.Contains(...) → ok || strings.Contains(...)
// When the guardrail is a map (ok=false for StringValue), with && the condition short-circuits to false.
// With || the condition would evaluate strings.Contains on s="" which returns false for "timeout".
// But if capability is "" then strings.Contains(anything, "") returns true.
// So to kill it: use a map-only guardrail that does NOT have the "timeout" key,
// and ensure the skill with conversation memory IS flagged.
// With &&: ok is false => skip string check => HasKey("timeout") also false => returns false => flagged.
// With ||: ok is false, but strings.Contains(strings.ToLower(""), "timeout") is false too => still false.
// Hmm, that won't differentiate. Let me reconsider.
//
// The full code at line 33-34:
//   if s, ok := g.StringValue(); ok && strings.Contains(strings.ToLower(s), capability) {
//       return true
//   }
// With INVERT_LOGICAL (&&→||):
//   if s, ok := g.StringValue(); ok || strings.Contains(strings.ToLower(s), capability) {
//       return true
//   }
// When ok is true but strings.Contains is false: && returns false, || returns true.
// So: a string guardrail that does NOT contain "timeout" keyword.
// With &&: ok=true, Contains=false => false => not returned true.
// With ||: ok=true => true => returns true immediately.
// This means: skill with conversation memory and a string guardrail like "max-retries: 3"
// (no "timeout" substring). With correct code: HasCapability returns false => LoopNoTimeout flagged.
// With mutation: HasCapability returns true => LoopNoTimeout NOT flagged.
func TestDetectLoopRisks_StringGuardrailWithoutTimeoutKeyword(t *testing.T) {
	// String guardrail that does NOT contain "timeout"
	gr := guardrailFromString(t, "max-retries: 3")
	skill := makeLoopSkill("no-timeout-str", []string{"input"}, []string{"output"},
		model.MemoryConversation, []model.GuardrailRule{gr})

	risks := DetectLoopRisks(skill, &DefaultGuardrailChecker{})

	hasNoTimeout := false
	for _, r := range risks {
		if r.Type == LoopNoTimeout {
			hasNoTimeout = true
		}
	}
	assert.True(t, hasNoTimeout,
		"should flag no-timeout risk when string guardrail does not contain 'timeout'")
}

// Additional test to differentiate && vs || more clearly:
// Map guardrail with a key that is NOT "timeout", combined with conversation memory.
// With correct code (&&): ok=false for StringValue => short-circuit, check HasKey("timeout")=false => not found.
// With || mutation: ok=false but strings.Contains(strings.ToLower(""), "timeout")=false => false.
// Then HasKey("timeout")=false => returns false. Both paths give false. This won't differentiate.
// The key differentiator is the string guardrail case above.
func TestDetectLoopRisks_MapGuardrailWithoutTimeoutKey(t *testing.T) {
	// Map guardrail without "timeout" key
	gr := guardrailFromMap(t, "retries", "3")
	skill := makeLoopSkill("map-no-timeout", []string{"input"}, []string{"output"},
		model.MemoryConversation, []model.GuardrailRule{gr})

	risks := DetectLoopRisks(skill, &DefaultGuardrailChecker{})

	hasNoTimeout := false
	for _, r := range risks {
		if r.Type == LoopNoTimeout {
			hasNoTimeout = true
		}
	}
	assert.True(t, hasNoTimeout,
		"should flag no-timeout when map guardrail lacks timeout key")
}
