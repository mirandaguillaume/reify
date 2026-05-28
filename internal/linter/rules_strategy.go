package linter

import (
	"fmt"

	"github.com/mirandaguillaume/reify/pkg/model"
)

type noEmptyToolsRule struct{}

func (r *noEmptyToolsRule) Name() string            { return "no-empty-tools" }
func (r *noEmptyToolsRule) DefaultSeverity() Severity { return SeverityWarning }

func (r *noEmptyToolsRule) Check(skill model.SkillBehavior) *LintResult {
	if len(skill.Strategy.Tools) == 0 {
		return &LintResult{
			Rule:     "no-empty-tools",
			Severity: SeverityWarning,
			Message:  fmt.Sprintf("Skill %q has no tools defined. An agent without tools has limited capability.", skill.Skill),
			Facet:    "strategy",
		}
	}
	return nil
}

func init() { Register(&noEmptyToolsRule{}) }
