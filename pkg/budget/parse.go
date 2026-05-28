package budget

import (
	"time"

	"github.com/mirandaguillaume/reify/pkg/model"
)

// ParseBudgetPolicy extracts budget-related guardrail rules from a guardrails list.
// Recognizes map rules with keys "max_tokens" (int) and "timeout" (duration string).
// Returns nil if no budget rules found.
func ParseBudgetPolicy(guardrails []model.GuardrailRule) *BudgetPolicy {
	var p BudgetPolicy
	found := false

	for _, g := range guardrails {
		m, ok := g.MapValue()
		if !ok {
			continue
		}

		if v, has := m["max_tokens"]; has {
			switch n := v.(type) {
			case float64:
				p.MaxTokens = int(n)
				found = true
			case int:
				p.MaxTokens = n
				found = true
			}
		}

		if v, has := m["timeout"]; has {
			if s, ok := v.(string); ok {
				if d, err := time.ParseDuration(s); err == nil {
					p.Timeout = d
					found = true
				}
			}
		}
	}

	if !found {
		return nil
	}
	return &p
}
