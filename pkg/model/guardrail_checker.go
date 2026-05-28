package model

// GuardrailChecker abstracts guardrail capability detection.
type GuardrailChecker interface {
	HasCapability(skill SkillBehavior, capability string) bool
}
