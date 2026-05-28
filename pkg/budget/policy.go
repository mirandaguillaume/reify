package budget

import "time"

// BudgetPolicy defines per-step budget constraints for LLM calls.
type BudgetPolicy struct {
	MaxTokens int           // max output tokens; 0 = no limit
	Timeout   time.Duration // wall-clock timeout per step; 0 = no timeout
}
