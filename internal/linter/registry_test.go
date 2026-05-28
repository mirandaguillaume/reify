package linter

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/mirandaguillaume/reify/pkg/model"
)

type dummyRule struct {
	name     string
	severity Severity
}

func (r *dummyRule) Name() string            { return r.name }
func (r *dummyRule) DefaultSeverity() Severity { return r.severity }
func (r *dummyRule) Check(_ model.SkillBehavior) *LintResult {
	return &LintResult{Rule: r.name, Severity: r.severity, Message: "dummy", Facet: "test"}
}

func TestRegister_AddsRule(t *testing.T) {
	original := rules
	defer func() { rules = original }()

	rules = nil
	Register(&dummyRule{name: "test-rule", severity: SeverityInfo})

	assert.Len(t, Rules(), 1)
	assert.Equal(t, "test-rule", Rules()[0].Name())
}

func TestResetRules_ClearsRegistry(t *testing.T) {
	original := rules
	defer func() { rules = original }()

	Register(&dummyRule{name: "temp", severity: SeverityInfo})
	ResetRules()

	assert.Empty(t, Rules())
}

func TestRules_ReturnsAllRegistered(t *testing.T) {
	original := rules
	defer func() { rules = original }()

	rules = nil
	Register(&dummyRule{name: "a", severity: SeverityInfo})
	Register(&dummyRule{name: "b", severity: SeverityWarning})
	Register(&dummyRule{name: "c", severity: SeverityError})

	assert.Len(t, Rules(), 3)
	assert.Equal(t, "a", Rules()[0].Name())
	assert.Equal(t, "b", Rules()[1].Name())
	assert.Equal(t, "c", Rules()[2].Name())
}
