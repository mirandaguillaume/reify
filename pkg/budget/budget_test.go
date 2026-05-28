package budget

import (
	"context"
	"errors"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/mirandaguillaume/reify/pkg/dag"
	"github.com/mirandaguillaume/reify/pkg/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"
)

// ── ParseBudgetPolicy Tests ──────────────────────────────────────────

func parseRules(yamlStr string) []model.GuardrailRule {
	var rules []model.GuardrailRule
	if err := yaml.Unmarshal([]byte(yamlStr), &rules); err != nil {
		panic(err)
	}
	return rules
}

func TestParseBudgetPolicy_MaxTokensOnly(t *testing.T) {
	p := ParseBudgetPolicy(parseRules(`- max_tokens: 4000`))
	require.NotNil(t, p)
	assert.Equal(t, 4000, p.MaxTokens)
	assert.Equal(t, time.Duration(0), p.Timeout)
}

func TestParseBudgetPolicy_TimeoutOnly(t *testing.T) {
	p := ParseBudgetPolicy(parseRules(`- timeout: "30s"`))
	require.NotNil(t, p)
	assert.Equal(t, 30*time.Second, p.Timeout)
	assert.Equal(t, 0, p.MaxTokens)
}

func TestParseBudgetPolicy_Both(t *testing.T) {
	p := ParseBudgetPolicy(parseRules("- max_tokens: 4000\n  timeout: \"30s\""))
	require.NotNil(t, p)
	assert.Equal(t, 4000, p.MaxTokens)
	assert.Equal(t, 30*time.Second, p.Timeout)
}

func TestParseBudgetPolicy_SeparateRules(t *testing.T) {
	p := ParseBudgetPolicy(parseRules("- max_tokens: 2000\n- timeout: \"10s\""))
	require.NotNil(t, p)
	assert.Equal(t, 2000, p.MaxTokens)
	assert.Equal(t, 10*time.Second, p.Timeout)
}

func TestParseBudgetPolicy_NoBudgetRules(t *testing.T) {
	p := ParseBudgetPolicy(parseRules(`- "no PII in output"`))
	assert.Nil(t, p)
}

func TestParseBudgetPolicy_EmptyGuardrails(t *testing.T) {
	assert.Nil(t, ParseBudgetPolicy([]model.GuardrailRule{}))
}

func TestParseBudgetPolicy_NilGuardrails(t *testing.T) {
	assert.Nil(t, ParseBudgetPolicy(nil))
}

func TestParseBudgetPolicy_InvalidTimeout(t *testing.T) {
	p := ParseBudgetPolicy(parseRules(`- timeout: "not-a-duration"`))
	assert.Nil(t, p, "invalid duration should be ignored")
}

func TestParseBudgetPolicy_MixedWithStringRules(t *testing.T) {
	p := ParseBudgetPolicy(parseRules("- \"no PII\"\n- max_tokens: 8000\n- \"be concise\""))
	require.NotNil(t, p)
	assert.Equal(t, 8000, p.MaxTokens)
}

// ── EstimateTokens / TruncateToTokens ────────────────────────────────

func TestEstimateTokens_Basic(t *testing.T) {
	assert.Equal(t, 2, EstimateTokens("hello world"))
}

func TestEstimateTokens_Empty(t *testing.T) {
	assert.Equal(t, 0, EstimateTokens(""))
}

func TestTruncateToTokens_NoTruncation(t *testing.T) {
	s, truncated := TruncateToTokens("short", 100)
	assert.Equal(t, "short", s)
	assert.False(t, truncated)
}

func TestTruncateToTokens_Truncates(t *testing.T) {
	long := strings.Repeat("x", 8000) // 2000 tokens
	s, truncated := TruncateToTokens(long, 100)
	assert.True(t, truncated)
	assert.Equal(t, 400, len(s)) // 100 tokens * 4 chars
}

// ── BudgetMiddleware Tests ───────────────────────────────────────────

func TestBudgetMiddleware_NilPolicy(t *testing.T) {
	var handlerCalled bool
	mw := BudgetMiddleware(nil)
	out, err := mw(context.Background(), map[string]any{"data": "hi"}, func(ctx context.Context, in map[string]any) (map[string]any, error) {
		handlerCalled = true
		_, hasMaxTokens := in[InputKeyMaxTokens]
		return map[string]any{"result": "ok", "has_max_tokens": hasMaxTokens}, nil
	})
	require.NoError(t, err)
	assert.True(t, handlerCalled)
	assert.False(t, out["has_max_tokens"].(bool), "nil policy should not inject __max_tokens")
}

func TestBudgetMiddleware_MaxTokens_InjectsInputKey(t *testing.T) {
	mw := BudgetMiddleware(&BudgetPolicy{MaxTokens: 4000})
	out, err := mw(context.Background(), map[string]any{}, func(ctx context.Context, in map[string]any) (map[string]any, error) {
		return map[string]any{"saw_max_tokens": in[InputKeyMaxTokens]}, nil
	})
	require.NoError(t, err)
	assert.Equal(t, 4000, out["saw_max_tokens"])
}

func TestBudgetMiddleware_MaxTokens_UnderLimit(t *testing.T) {
	mw := BudgetMiddleware(&BudgetPolicy{MaxTokens: 1000})
	out, err := mw(context.Background(), map[string]any{}, func(ctx context.Context, in map[string]any) (map[string]any, error) {
		return map[string]any{"text": strings.Repeat("x", 100)}, nil // 25 tokens
	})
	require.NoError(t, err)
	_, hasWarning := out[OutputKeyBudgetWarning]
	assert.False(t, hasWarning)
}

func TestBudgetMiddleware_MaxTokens_OverLimit(t *testing.T) {
	mw := BudgetMiddleware(&BudgetPolicy{MaxTokens: 1000})
	out, err := mw(context.Background(), map[string]any{}, func(ctx context.Context, in map[string]any) (map[string]any, error) {
		return map[string]any{"text": strings.Repeat("x", 20000)}, nil // 5000 tokens
	})
	require.NoError(t, err)
	assert.Equal(t, 4000, len(out["text"].(string)), "should be truncated to 1000*4 chars")
	warning, ok := out[OutputKeyBudgetWarning]
	assert.True(t, ok)
	assert.Contains(t, warning.(string), "5000")
	assert.Contains(t, warning.(string), "1000")
}

func TestBudgetMiddleware_MaxTokens_NonStringIgnored(t *testing.T) {
	mw := BudgetMiddleware(&BudgetPolicy{MaxTokens: 10})
	out, err := mw(context.Background(), map[string]any{}, func(ctx context.Context, in map[string]any) (map[string]any, error) {
		return map[string]any{"count": 42, "flag": true}, nil
	})
	require.NoError(t, err)
	_, hasWarning := out[OutputKeyBudgetWarning]
	assert.False(t, hasWarning)
}

func TestBudgetMiddleware_Timeout_WithinLimit(t *testing.T) {
	mw := BudgetMiddleware(&BudgetPolicy{Timeout: 1 * time.Second})
	out, err := mw(context.Background(), map[string]any{}, func(ctx context.Context, in map[string]any) (map[string]any, error) {
		return map[string]any{"ok": true}, nil
	})
	require.NoError(t, err)
	assert.True(t, out["ok"].(bool))
}

func TestBudgetMiddleware_Timeout_Exceeded(t *testing.T) {
	mw := BudgetMiddleware(&BudgetPolicy{Timeout: 50 * time.Millisecond})
	_, err := mw(context.Background(), map[string]any{}, func(ctx context.Context, in map[string]any) (map[string]any, error) {
		time.Sleep(200 * time.Millisecond)
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}
		return map[string]any{}, nil
	})
	assert.Error(t, err)
	assert.True(t, errors.Is(err, context.DeadlineExceeded))
}

func TestBudgetMiddleware_TimeoutOnly_NoTokenCheck(t *testing.T) {
	mw := BudgetMiddleware(&BudgetPolicy{Timeout: 1 * time.Second})
	out, err := mw(context.Background(), map[string]any{}, func(ctx context.Context, in map[string]any) (map[string]any, error) {
		return map[string]any{"text": strings.Repeat("x", 100000)}, nil
	})
	require.NoError(t, err)
	_, hasWarning := out[OutputKeyBudgetWarning]
	assert.False(t, hasWarning, "no MaxTokens = no truncation")
}

func TestBudgetMiddleware_HandlerError_NoPostCheck(t *testing.T) {
	mw := BudgetMiddleware(&BudgetPolicy{MaxTokens: 10})
	_, err := mw(context.Background(), map[string]any{}, func(ctx context.Context, in map[string]any) (map[string]any, error) {
		return nil, errors.New("handler failed")
	})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "handler failed")
}

// ── DAG Integration ──────────────────────────────────��───────────────

func TestBudgetMiddleware_DAG_TimeoutTriggersRetry(t *testing.T) {
	var calls int32
	a := &dag.Node{
		ID: "a", Produces: []string{"x"}, MaxRetries: 1,
		Run: func(ctx context.Context, in map[string]any) (map[string]any, error) {
			n := atomic.AddInt32(&calls, 1)
			if n == 1 {
				time.Sleep(200 * time.Millisecond)
				if ctx.Err() != nil {
					return nil, ctx.Err()
				}
			}
			return map[string]any{"x": "ok"}, nil
		},
		Middlewares: []dag.Middleware{BudgetMiddleware(&BudgetPolicy{Timeout: 50 * time.Millisecond})},
	}

	d, err := dag.New(a)
	require.NoError(t, err)
	out, err := d.Execute(context.Background())
	require.NoError(t, err)
	assert.Equal(t, "ok", out["x"])
	assert.Equal(t, int32(2), atomic.LoadInt32(&calls))
}

func TestBudgetMiddleware_DAG_NoBudgetPolicyUnchanged(t *testing.T) {
	a := &dag.Node{
		ID: "a", Produces: []string{"x"},
		Run: func(ctx context.Context, in map[string]any) (map[string]any, error) {
			return map[string]any{"x": "hello"}, nil
		},
		Middlewares: []dag.Middleware{BudgetMiddleware(nil)},
	}

	d, err := dag.New(a)
	require.NoError(t, err)
	out, err := d.Execute(context.Background())
	require.NoError(t, err)
	assert.Equal(t, "hello", out["x"])
}

func TestBudgetMiddleware_DAG_ConcurrentNodes(t *testing.T) {
	var c1, c2 int32
	a := &dag.Node{
		ID: "a", Produces: []string{"x"},
		Run: func(ctx context.Context, in map[string]any) (map[string]any, error) {
			atomic.AddInt32(&c1, 1)
			return map[string]any{"x": 1}, nil
		},
		Middlewares: []dag.Middleware{BudgetMiddleware(&BudgetPolicy{MaxTokens: 100})},
	}
	b := &dag.Node{
		ID: "b", Produces: []string{"y"},
		Run: func(ctx context.Context, in map[string]any) (map[string]any, error) {
			atomic.AddInt32(&c2, 1)
			return map[string]any{"y": 2}, nil
		},
		Middlewares: []dag.Middleware{BudgetMiddleware(&BudgetPolicy{Timeout: 5 * time.Second})},
	}

	d, err := dag.New(a, b)
	require.NoError(t, err)
	_, err = d.Execute(context.Background())
	require.NoError(t, err)
	assert.Equal(t, int32(1), atomic.LoadInt32(&c1))
	assert.Equal(t, int32(1), atomic.LoadInt32(&c2))
}
