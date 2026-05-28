package budget

import (
	"context"
	"fmt"

	"github.com/mirandaguillaume/reify/pkg/dag"
)

const (
	// InputKeyMaxTokens is injected into inputs to signal the handler
	// to limit LLM output tokens.
	InputKeyMaxTokens = "__max_tokens"

	// OutputKeyBudgetWarning is added to outputs when token truncation occurs.
	OutputKeyBudgetWarning = "__budget_warning"
)

// BudgetMiddleware returns a dag.Middleware that enforces token cap and timeout
// constraints on LLM step execution.
//
// If policy is nil, returns a no-op middleware (zero overhead).
func BudgetMiddleware(policy *BudgetPolicy) dag.Middleware {
	if policy == nil {
		return func(ctx context.Context, inputs map[string]any, next dag.MiddlewareFunc) (map[string]any, error) {
			return next(ctx, inputs)
		}
	}

	return func(ctx context.Context, inputs map[string]any, next dag.MiddlewareFunc) (map[string]any, error) {
		// Pre: inject max_tokens hint for the handler.
		// Copy inputs to avoid mutating the caller's map across retry attempts —
		// the DAG reuses the same map reference on every runWithRetry iteration.
		if policy.MaxTokens > 0 {
			copied := make(map[string]any, len(inputs)+1)
			for k, v := range inputs {
				copied[k] = v
			}
			copied[InputKeyMaxTokens] = policy.MaxTokens
			inputs = copied
		}

		// Pre: apply guardrail-level timeout
		if policy.Timeout > 0 {
			var cancel context.CancelFunc
			ctx, cancel = context.WithTimeout(ctx, policy.Timeout)
			defer cancel()
		}

		// Execute handler
		outputs, err := next(ctx, inputs)
		if err != nil {
			return nil, err
		}

		// Post: check token counts on string outputs
		if policy.MaxTokens > 0 {
			for k, v := range outputs {
				s, ok := v.(string)
				if !ok {
					continue
				}
				estimated := EstimateTokens(s)
				if estimated > policy.MaxTokens {
					truncated, _ := TruncateToTokens(s, policy.MaxTokens)
					outputs[k] = truncated
					outputs[OutputKeyBudgetWarning] = fmt.Sprintf(
						"output truncated: estimated %d tokens exceeds cap of %d",
						estimated, policy.MaxTokens)
				}
			}
		}

		return outputs, nil
	}
}
