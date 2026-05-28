package linter

import "github.com/mirandaguillaume/reify/pkg/model"

// Severity represents the severity level of a lint result.
type Severity string

const (
	SeverityError   Severity = "error"
	SeverityWarning Severity = "warning"
	SeverityInfo    Severity = "info"
)

// LintResult represents a single lint finding.
type LintResult struct {
	Rule     string
	Severity Severity
	Message  string
	Facet    string
}

// LintSkill runs all registered lint rules against a skill and returns findings.
func LintSkill(skill model.SkillBehavior) []LintResult {
	var results []LintResult
	for _, rule := range rules {
		if r := rule.Check(skill); r != nil {
			results = append(results, *r)
		}
	}
	return results
}
