package linter

import "github.com/mirandaguillaume/reify/pkg/model"

// Rule is the interface for lint rules. Rules self-register via init().
type Rule interface {
	Name() string
	DefaultSeverity() Severity
	Check(skill model.SkillBehavior) *LintResult
}

var rules []Rule

// Register adds a rule to the global registry.
func Register(r Rule) {
	rules = append(rules, r)
}

// Rules returns all registered rules.
func Rules() []Rule {
	return rules
}

// ResetRules clears the registry. Used only in tests.
func ResetRules() {
	rules = nil
}
