package linter

import (
	"fmt"

	"github.com/mirandaguillaume/reify/pkg/model"
)

type hasWhenToUseRule struct{}

func (r *hasWhenToUseRule) Name() string            { return "has-when-to-use" }
func (r *hasWhenToUseRule) DefaultSeverity() Severity { return SeverityInfo }

func (r *hasWhenToUseRule) Check(skill model.SkillBehavior) *LintResult {
	if skill.WhenToUse.IsEmpty() {
		return &LintResult{
			Rule:     "has-when-to-use",
			Severity: SeverityInfo,
			Message:  fmt.Sprintf("Skill %q has no when_to_use guidance. Consider adding triggers and boundaries.", skill.Skill),
			Facet:    "when_to_use",
		}
	}
	return nil
}

func init() { Register(&hasWhenToUseRule{}) }
