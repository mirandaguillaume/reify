package linter

import (
	"fmt"

	"github.com/mirandaguillaume/reify/pkg/model"
)

type hasGuardrailsRule struct{}

func (r *hasGuardrailsRule) Name() string            { return "has-guardrails" }
func (r *hasGuardrailsRule) DefaultSeverity() Severity { return SeverityWarning }

func (r *hasGuardrailsRule) Check(skill model.SkillBehavior) *LintResult {
	if len(skill.Guardrails) == 0 {
		return &LintResult{
			Rule:     "has-guardrails",
			Severity: SeverityWarning,
			Message:  fmt.Sprintf("Skill %q has no guardrails. Consider adding limits (timeout, max_tokens, etc.).", skill.Skill),
			Facet:    "guardrails",
		}
	}
	return nil
}

func init() { Register(&hasGuardrailsRule{}) }

type securityNeedsGuardrailsRule struct{}

func (r *securityNeedsGuardrailsRule) Name() string            { return "security-needs-guardrails" }
func (r *securityNeedsGuardrailsRule) DefaultSeverity() Severity { return SeverityError }

func (r *securityNeedsGuardrailsRule) Check(skill model.SkillBehavior) *LintResult {
	hasHighAccess := skill.Security.Filesystem == model.AccessFull || skill.Security.Filesystem == model.AccessReadWrite
	if hasHighAccess && len(skill.Guardrails) == 0 {
		return &LintResult{
			Rule:     "security-needs-guardrails",
			Severity: SeverityError,
			Message:  fmt.Sprintf("Skill %q has %s filesystem access but no guardrails. This is dangerous.", skill.Skill, skill.Security.Filesystem),
			Facet:    "security",
		}
	}
	return nil
}

func init() { Register(&securityNeedsGuardrailsRule{}) }
