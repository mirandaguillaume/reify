package linter

import (
	"fmt"

	"github.com/mirandaguillaume/reify/pkg/model"
)

type observableOutputsRule struct{}

func (r *observableOutputsRule) Name() string            { return "observable-outputs" }
func (r *observableOutputsRule) DefaultSeverity() Severity { return SeverityWarning }

func (r *observableOutputsRule) Check(skill model.SkillBehavior) *LintResult {
	if len(skill.Context.Produces) > 0 && len(skill.Observability.Metrics) == 0 {
		return &LintResult{
			Rule:     "observable-outputs",
			Severity: SeverityWarning,
			Message:  fmt.Sprintf("Skill %q produces data but has no observability metrics. Add metrics to track output quality.", skill.Skill),
			Facet:    "observability",
		}
	}
	return nil
}

func init() { Register(&observableOutputsRule{}) }
