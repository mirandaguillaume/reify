package analyzer

import (
	"fmt"
	"strings"

	"github.com/mirandaguillaume/reify/pkg/model"
)

// LoopRiskType represents the type of loop risk.
type LoopRiskType string

const (
	LoopSelfReference LoopRiskType = "self-reference"
	LoopNoTimeout     LoopRiskType = "no-timeout"
)

// LoopRisk represents a potential loop risk in a skill.
type LoopRisk struct {
	Type     LoopRiskType
	Skill    string
	Message  string
	Severity string // "warning" or "error"
}

// DefaultGuardrailChecker implements GuardrailChecker by scanning guardrail
// rules for a capability keyword (string contains) or map key match.
type DefaultGuardrailChecker struct{}

// HasCapability returns true when any guardrail rule in the skill matches the
// given capability either as a substring in a string rule or as a map key.
func (c *DefaultGuardrailChecker) HasCapability(skill model.SkillBehavior, capability string) bool {
	for _, g := range skill.Guardrails {
		if s, ok := g.StringValue(); ok && strings.Contains(strings.ToLower(s), capability) {
			return true
		}
		if g.HasKey(capability) {
			return true
		}
	}
	return false
}

// filterOverlap returns elements that appear in both slices.
func filterOverlap(a, b []string) []string {
	var overlap []string
	bSet := make(map[string]bool)
	for _, s := range b {
		bSet[s] = true
	}
	for _, s := range a {
		if bSet[s] {
			overlap = append(overlap, s)
		}
	}
	return overlap
}

// DetectLoopRisks analyzes a skill for potential loop risks:
// self-referencing data and missing timeout guardrails.
func DetectLoopRisks(skill model.SkillBehavior, checker model.GuardrailChecker) []LoopRisk {
	var risks []LoopRisk

	// Self-reference: consumes and produces the same data
	overlap := filterOverlap(skill.Context.Consumes, skill.Context.Produces)
	if len(overlap) > 0 {
		risks = append(risks, LoopRisk{
			Type:     LoopSelfReference,
			Skill:    skill.Skill,
			Message:  fmt.Sprintf("Skill consumes and produces the same data: [%s]. This can cause infinite loops.", strings.Join(overlap, ", ")),
			Severity: "error",
		})
	}

	// No timeout with persistent memory
	if skill.Context.Memory != model.MemoryShortTerm && !checker.HasCapability(skill, "timeout") {
		risks = append(risks, LoopRisk{
			Type:     LoopNoTimeout,
			Skill:    skill.Skill,
			Message:  fmt.Sprintf("Skill uses %s memory but has no timeout guardrail. Risk of unbounded execution.", skill.Context.Memory),
			Severity: "warning",
		})
	}

	return risks
}
