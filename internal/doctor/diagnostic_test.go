package doctor

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/mirandaguillaume/reify/pkg/dag"
)

// ─── NodeError ────────────────────────────────────────────────────────────────

func TestNodeError_Error_SingleFailure(t *testing.T) {
	e := &NodeError{
		NodeID:    "analyzer",
		InputKeys: []string{"detected_format", "provider", "registry"},
		Err:       errors.New("LLM timeout"),
		Attempts:  1,
		AllErrors: []error{errors.New("LLM timeout")},
	}
	msg := e.Error()
	assert.Contains(t, msg, "analyzer")
	assert.Contains(t, msg, "LLM timeout")
	assert.Contains(t, msg, "detected_format")
	assert.Contains(t, msg, "provider")
	// Single attempt → no retry suffix
	assert.NotContains(t, msg, "Retry")
}

func TestNodeError_Error_WithRetry(t *testing.T) {
	e := &NodeError{
		NodeID:    "analyzer",
		InputKeys: []string{"detected_format"},
		Err:       errors.New("final error"),
		Attempts:  3,
		AllErrors: []error{
			errors.New("attempt 1"),
			errors.New("attempt 2"),
			errors.New("final error"),
		},
	}
	msg := e.Error()
	assert.Contains(t, msg, "3 attempts")
	// All failure reasons must appear (AC #3)
	assert.Contains(t, msg, "attempt 1")
	assert.Contains(t, msg, "attempt 2")
	assert.Contains(t, msg, "final error")
}

func TestNodeError_Unwrap(t *testing.T) {
	inner := errors.New("inner error")
	e := &NodeError{NodeID: "x", Err: inner, Attempts: 1}
	assert.Equal(t, inner, errors.Unwrap(e))
}

// ─── wrapWithPanic ────────────────────────────────────────────────────────────

func TestWrapWithPanic_NormalExecution(t *testing.T) {
	n := &dag.Node{
		ID:   "test",
		Kind: dag.KindTask,
		Run: func(_ context.Context, inputs map[string]any) (map[string]any, error) {
			return map[string]any{"out": "value"}, nil
		},
	}
	wrapped := wrapWithPanic(n)
	out, err := wrapped.Run(context.Background(), map[string]any{"in": "x"})
	require.NoError(t, err)
	assert.Equal(t, "value", out["out"])
}

func TestWrapWithPanic_RecoversPanic(t *testing.T) {
	n := &dag.Node{
		ID:   "panicking",
		Kind: dag.KindTask,
		Run: func(_ context.Context, _ map[string]any) (map[string]any, error) {
			panic("something went wrong")
		},
	}
	wrapped := wrapWithPanic(n)
	_, err := wrapped.Run(context.Background(), map[string]any{"key": "val"})
	require.Error(t, err)

	var nodeErr *NodeError
	require.ErrorAs(t, err, &nodeErr)
	assert.Equal(t, "panicking", nodeErr.NodeID)
	assert.Contains(t, nodeErr.Error(), "panic")
	assert.Contains(t, nodeErr.Error(), "something went wrong")
}

func TestWrapWithPanic_NilRunUnchanged(t *testing.T) {
	n := &dag.Node{ID: "nil-run", Kind: dag.KindTask, Run: nil}
	wrapped := wrapWithPanic(n)
	assert.Nil(t, wrapped.Run)
}

// ─── wrapWithRetry ────────────────────────────────────────────────────────────

func TestWrapWithRetry_SucceedsOnFirstAttempt(t *testing.T) {
	calls := 0
	n := &dag.Node{
		ID:   "succeeder",
		Kind: dag.KindTask,
		Run: func(_ context.Context, _ map[string]any) (map[string]any, error) {
			calls++
			return map[string]any{"result": "ok"}, nil
		},
	}
	wrapped := wrapWithRetry(n, 3)
	out, err := wrapped.Run(context.Background(), nil)
	require.NoError(t, err)
	assert.Equal(t, 1, calls)
	assert.Equal(t, "ok", out["result"])
}

func TestWrapWithRetry_SucceedsOnRetry(t *testing.T) {
	calls := 0
	n := &dag.Node{
		ID:   "flaky",
		Kind: dag.KindTask,
		Run: func(_ context.Context, _ map[string]any) (map[string]any, error) {
			calls++
			if calls < 2 {
				return nil, fmt.Errorf("transient error attempt %d", calls)
			}
			return map[string]any{"result": "ok"}, nil
		},
	}
	wrapped := wrapWithRetry(n, 3)
	out, err := wrapped.Run(context.Background(), nil)
	require.NoError(t, err)
	assert.Equal(t, 2, calls)
	assert.Equal(t, "ok", out["result"])
}

func TestWrapWithRetry_ExhaustsAllAttempts(t *testing.T) {
	calls := 0
	n := &dag.Node{
		ID:   "always-fails",
		Kind: dag.KindTask,
		Run: func(_ context.Context, _ map[string]any) (map[string]any, error) {
			calls++
			return nil, fmt.Errorf("error attempt %d", calls)
		},
	}
	wrapped := wrapWithRetry(n, 3)
	_, err := wrapped.Run(context.Background(), map[string]any{"input": "x"})
	require.Error(t, err)
	assert.Equal(t, 3, calls)

	var nodeErr *NodeError
	require.ErrorAs(t, err, &nodeErr)
	assert.Equal(t, "always-fails", nodeErr.NodeID)
	assert.Equal(t, 3, nodeErr.Attempts)
	assert.Len(t, nodeErr.AllErrors, 3)
	assert.Contains(t, nodeErr.Error(), "3 attempts")
	// AC #3: all failure reasons must appear
	assert.Contains(t, nodeErr.Error(), "[1]")
	assert.Contains(t, nodeErr.Error(), "[3]")
}

func TestWrapWithRetry_PreventsDoubleRetry(t *testing.T) {
	n := &dag.Node{
		ID:         "retry-node",
		Kind:       dag.KindTask,
		MaxRetries: 5, // will be zeroed by wrapWithRetry
		Run: func(_ context.Context, _ map[string]any) (map[string]any, error) {
			return nil, errors.New("fail")
		},
	}
	wrapped := wrapWithRetry(n, 3)
	assert.Equal(t, 0, wrapped.MaxRetries, "MaxRetries must be zeroed to prevent pkg/dag double-retry")
}

func TestWrapWithRetry_ContextCancelledBeforeFirstAttempt(t *testing.T) {
	n := &dag.Node{
		ID:   "never-runs",
		Kind: dag.KindTask,
		Run: func(_ context.Context, _ map[string]any) (map[string]any, error) {
			t.Fatal("Run should never be called with cancelled context")
			return nil, nil
		},
	}
	wrapped := wrapWithRetry(n, 3)

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // cancel before calling

	_, err := wrapped.Run(ctx, map[string]any{"k": "v"})
	require.Error(t, err, "must return error, not panic")

	var nodeErr *NodeError
	require.ErrorAs(t, err, &nodeErr)
	assert.Equal(t, 1, nodeErr.Attempts)
	assert.Len(t, nodeErr.AllErrors, 1)
}

func TestWrapWithRetry_SkipsFurtherRetriesOnPanic(t *testing.T) {
	calls := 0
	n := &dag.Node{
		ID:   "panicker",
		Kind: dag.KindTask,
		Run:  nil, // set below after panic wrapping
	}
	// Simulate the composition: wrapWithPanic first, then wrapWithRetry
	n.Run = func(_ context.Context, _ map[string]any) (map[string]any, error) {
		calls++
		panic("nil pointer")
	}
	n = wrapWithPanic(n)
	n = wrapWithRetry(n, 3)

	_, err := n.Run(context.Background(), nil)
	require.Error(t, err)
	assert.Equal(t, 1, calls, "deterministic panic should not be retried")
}

func TestWrapWithRetry_MaxAttemptsOne_NilRunReturnsUnchanged(t *testing.T) {
	n := &dag.Node{ID: "x", Run: nil}
	wrapped := wrapWithRetry(n, 3)
	assert.Nil(t, wrapped.Run)
}

// ─── wrapWithDebug ────────────────────────────────────────────────────────────

func TestWrapWithDebug_LogsStartAndComplete(t *testing.T) {
	var buf bytes.Buffer
	n := &dag.Node{
		ID:       "test-node",
		Kind:     dag.KindTask,
		Consumes: []string{"alpha", "beta"},
		Run: func(_ context.Context, _ map[string]any) (map[string]any, error) {
			return map[string]any{"result": "done"}, nil
		},
	}
	wrapped := wrapWithDebug(n, &buf)
	_, err := wrapped.Run(context.Background(), map[string]any{"alpha": 1, "beta": 2})
	require.NoError(t, err)

	output := buf.String()
	assert.Contains(t, output, "[DEBUG]")
	assert.Contains(t, output, "test-node")
	assert.Contains(t, output, "starting")
	assert.Contains(t, output, "completed")
	assert.Contains(t, output, "result") // output key appears in completion log
}

func TestWrapWithDebug_LogsOnFailure(t *testing.T) {
	var buf bytes.Buffer
	n := &dag.Node{
		ID:   "failing-node",
		Kind: dag.KindTask,
		Run: func(_ context.Context, _ map[string]any) (map[string]any, error) {
			return nil, errors.New("node error")
		},
	}
	wrapped := wrapWithDebug(n, &buf)
	_, err := wrapped.Run(context.Background(), nil)
	require.Error(t, err)

	output := buf.String()
	assert.Contains(t, output, "[DEBUG]")
	assert.Contains(t, output, "failed")
	assert.Contains(t, output, "node error")
}

func TestWrapWithDebug_SilentWhenNilWriter(t *testing.T) {
	n := &dag.Node{
		ID:   "silent",
		Kind: dag.KindTask,
		Run: func(_ context.Context, _ map[string]any) (map[string]any, error) {
			return map[string]any{"x": 1}, nil
		},
	}
	wrapped := wrapWithDebug(n, nil)
	// When w is nil, the Run function should be identical (not wrapped)
	assert.NotNil(t, wrapped.Run)
	out, err := wrapped.Run(context.Background(), nil)
	require.NoError(t, err)
	assert.Equal(t, 1, out["x"])
}

func TestWrapWithDebug_LLMNodeLogsExtraLine(t *testing.T) {
	var buf bytes.Buffer
	n := &dag.Node{
		ID:       "analyzer",
		Kind:     dag.KindTask,
		Consumes: []string{"provider", "detected_format"},
		Run: func(_ context.Context, _ map[string]any) (map[string]any, error) {
			return map[string]any{"findings": []string{}}, nil
		},
	}
	wrapped := wrapWithDebug(n, &buf)
	_, err := wrapped.Run(context.Background(), map[string]any{"provider": nil, "detected_format": nil})
	require.NoError(t, err)

	output := buf.String()
	lines := strings.Split(strings.TrimSpace(output), "\n")
	assert.GreaterOrEqual(t, len(lines), 3, "LLM node should emit at least 3 debug lines (start, LLM note, complete)")
}

// ─── Task 2: nil Run node skip ────────────────────────────────────────────────

func TestNilRunNodeIsSkipped(t *testing.T) {
	nilNode := &dag.Node{
		ID:       "nil-run",
		Kind:     dag.KindTask,
		Consumes: []string{},
		Produces: []string{"nil_output"},
		Run:      nil,
	}
	d, err := dag.New(nilNode)
	require.NoError(t, err)

	outputs, err := d.Execute(context.Background())
	require.NoError(t, err)

	_, present := outputs["nil_output"]
	assert.False(t, present, "nil-run node must not produce output")
}
