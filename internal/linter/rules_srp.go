package linter

import (
	"fmt"
	"strings"

	"github.com/mirandaguillaume/reify/pkg/model"
)

// actionVerbs are common imperative verbs that start a new action clause.
// When "and"/"et" precedes one of these, it signals two distinct actions
// rather than a compound object (e.g., "files and directories").
var actionVerbs = map[string]bool{
	"add": true, "analyze": true, "apply": true, "assess": true,
	"build": true, "check": true, "compile": true, "configure": true,
	"create": true, "debug": true, "delete": true, "deploy": true,
	"detect": true, "document": true, "download": true, "edit": true,
	"ensure": true, "evaluate": true, "execute": true, "export": true,
	"extract": true, "fetch": true, "filter": true, "find": true,
	"fix": true, "format": true, "generate": true, "identify": true,
	"implement": true, "import": true, "inspect": true, "install": true,
	"lint": true, "list": true, "load": true, "locate": true,
	"log": true, "map": true, "merge": true, "modify": true,
	"monitor": true, "move": true, "notify": true, "open": true,
	"optimize": true, "output": true, "parse": true, "patch": true,
	"process": true, "produce": true, "profile": true, "publish": true,
	"query": true, "read": true, "refactor": true, "remove": true,
	"rename": true, "replace": true, "report": true, "resolve": true,
	"review": true, "rewrite": true, "run": true, "save": true,
	"scan": true, "search": true, "send": true, "set": true,
	"submit": true, "test": true, "trace": true, "transform": true,
	"understand": true, "update": true, "upload": true, "validate": true,
	"verify": true, "write": true,
}

// sequencingConjunctions always indicate two separate actions.
var sequencingConjunctions = []string{" then ", " & "}

// dualActionConjunctions only flag when followed by an action verb.
var dualActionConjunctions = []string{" and "}

// containsDualAction checks whether text contains a conjunction joining two
// action clauses. Returns the matched conjunction or empty string.
func containsDualAction(text string) string {
	lower := strings.ToLower(text)

	// Sequencing conjunctions always indicate dual action.
	for _, conj := range sequencingConjunctions {
		if strings.Contains(lower, conj) {
			return conj
		}
	}

	// "and"/"et" only flag when followed by an action verb.
	for _, conj := range dualActionConjunctions {
		idx := strings.Index(lower, conj)
		if idx < 0 {
			continue
		}
		after := strings.TrimSpace(lower[idx+len(conj):])
		firstWord := strings.Fields(after)
		if len(firstWord) > 0 && actionVerbs[firstWord[0]] {
			return conj
		}
	}
	return ""
}

type singleProducesOutputRule struct{}

func (r *singleProducesOutputRule) Name() string            { return "single-produces-output" }
func (r *singleProducesOutputRule) DefaultSeverity() Severity { return SeverityError }

func (r *singleProducesOutputRule) Check(skill model.SkillBehavior) *LintResult {
	count := len(skill.Context.Produces)
	if count != 1 {
		return &LintResult{
			Rule:     "single-produces-output",
			Severity: SeverityError,
			Message:  fmt.Sprintf("Skill %q must produce exactly 1 output, got %d. SRP: one skill = one deliverable.", skill.Skill, count),
			Facet:    "context",
		}
	}
	return nil
}

func init() { Register(&singleProducesOutputRule{}) }

type producesMatchesDescriptionRule struct{}

func (r *producesMatchesDescriptionRule) Name() string            { return "produces-matches-description" }
func (r *producesMatchesDescriptionRule) DefaultSeverity() Severity { return SeverityError }

func (r *producesMatchesDescriptionRule) Check(skill model.SkillBehavior) *LintResult {
	if conj := containsDualAction(skill.Strategy.Approach); conj != "" {
		return &LintResult{
			Rule:     "produces-matches-description",
			Severity: SeverityError,
			Message:  fmt.Sprintf("Skill %q strategy.approach contains conjunction %q suggesting multiple responsibilities. Split into separate skills.", skill.Skill, conj),
			Facet:    "strategy",
		}
	}
	return nil
}

func init() { Register(&producesMatchesDescriptionRule{}) }

type skillNameMatchesOutputRule struct{}

func (r *skillNameMatchesOutputRule) Name() string            { return "skill-name-matches-output" }
func (r *skillNameMatchesOutputRule) DefaultSeverity() Severity { return SeverityError }

func (r *skillNameMatchesOutputRule) Check(skill model.SkillBehavior) *LintResult {
	patterns := []string{"-and-", "-then-", "-&-"}
	lower := strings.ToLower(skill.Skill)
	for _, pat := range patterns {
		if strings.Contains(lower, pat) {
			return &LintResult{
				Rule:     "skill-name-matches-output",
				Severity: SeverityError,
				Message:  fmt.Sprintf("Skill name %q contains conjunction pattern %q. A skill name should describe a single responsibility.", skill.Skill, pat),
				Facet:    "context",
			}
		}
	}
	return nil
}

func init() { Register(&skillNameMatchesOutputRule{}) }

// stepsConjunctionRule checks individual steps for conjunctions that suggest
// a single step is doing too much (e.g., "Read issue and search codebase").
type stepsConjunctionRule struct{}

func (r *stepsConjunctionRule) Name() string              { return "steps-conjunction-check" }
func (r *stepsConjunctionRule) DefaultSeverity() Severity { return SeverityError }

func (r *stepsConjunctionRule) Check(skill model.SkillBehavior) *LintResult {
	for _, step := range skill.Strategy.Steps {
		if conj := containsDualAction(step); conj != "" {
			return &LintResult{
				Rule:     "steps-conjunction-check",
				Severity: SeverityError,
				Message:  fmt.Sprintf("Skill %q step %q contains conjunction %q suggesting the step does too much. Break it down.", skill.Skill, step, conj),
				Facet:    "strategy",
			}
		}
	}
	return nil
}

func init() { Register(&stepsConjunctionRule{}) }

// maxStepsCountRule flags skills with too many steps — a sign of multiple responsibilities.
type maxStepsCountRule struct{}

func (r *maxStepsCountRule) Name() string              { return "max-steps-count" }
func (r *maxStepsCountRule) DefaultSeverity() Severity { return SeverityWarning }

func (r *maxStepsCountRule) Check(skill model.SkillBehavior) *LintResult {
	if len(skill.Strategy.Steps) > 5 {
		return &LintResult{
			Rule:     "max-steps-count",
			Severity: SeverityWarning,
			Message:  fmt.Sprintf("Skill %q has %d steps (max recommended: 5). Consider splitting into smaller skills.", skill.Skill, len(skill.Strategy.Steps)),
			Facet:    "strategy",
		}
	}
	return nil
}

func init() { Register(&maxStepsCountRule{}) }
