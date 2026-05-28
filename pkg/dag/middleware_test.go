package dag

import (
	"context"
	"errors"
	"fmt"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ── Unit Tests: Middleware Types ──────────────────────────────────────

func TestMiddleware_SinglePreInspection(t *testing.T) {
	var handlerCalled bool
	handler := MiddlewareFunc(func(ctx context.Context, inputs map[string]any) (map[string]any, error) {
		handlerCalled = true
		return map[string]any{"result": "ok"}, nil
	})

	var sawInputs map[string]any
	mw := Middleware(func(ctx context.Context, inputs map[string]any, next MiddlewareFunc) (map[string]any, error) {
		sawInputs = inputs
		return next(ctx, inputs)
	})

	chain := buildChain(handler, []Middleware{mw})
	out, err := chain(context.Background(), map[string]any{"key": "val"})

	require.NoError(t, err)
	assert.True(t, handlerCalled)
	assert.Equal(t, "val", sawInputs["key"])
	assert.Equal(t, "ok", out["result"])
}

func TestMiddleware_SinglePostInspection(t *testing.T) {
	handler := MiddlewareFunc(func(ctx context.Context, inputs map[string]any) (map[string]any, error) {
		return map[string]any{"result": "hello"}, nil
	})

	var sawOutput map[string]any
	mw := Middleware(func(ctx context.Context, inputs map[string]any, next MiddlewareFunc) (map[string]any, error) {
		out, err := next(ctx, inputs)
		sawOutput = out
		return out, err
	})

	chain := buildChain(handler, []Middleware{mw})
	out, err := chain(context.Background(), map[string]any{})

	require.NoError(t, err)
	assert.Equal(t, "hello", out["result"])
	assert.Equal(t, "hello", sawOutput["result"])
}

func TestMiddleware_InputModification(t *testing.T) {
	handler := MiddlewareFunc(func(ctx context.Context, inputs map[string]any) (map[string]any, error) {
		return map[string]any{"saw_injected": inputs["injected"]}, nil
	})

	mw := Middleware(func(ctx context.Context, inputs map[string]any, next MiddlewareFunc) (map[string]any, error) {
		inputs["injected"] = "by-middleware"
		return next(ctx, inputs)
	})

	chain := buildChain(handler, []Middleware{mw})
	out, err := chain(context.Background(), map[string]any{})

	require.NoError(t, err)
	assert.Equal(t, "by-middleware", out["saw_injected"])
}

func TestMiddleware_OutputModification(t *testing.T) {
	handler := MiddlewareFunc(func(ctx context.Context, inputs map[string]any) (map[string]any, error) {
		return map[string]any{"original": true}, nil
	})

	mw := Middleware(func(ctx context.Context, inputs map[string]any, next MiddlewareFunc) (map[string]any, error) {
		out, err := next(ctx, inputs)
		if err != nil {
			return nil, err
		}
		out["added"] = true
		return out, nil
	})

	chain := buildChain(handler, []Middleware{mw})
	out, err := chain(context.Background(), map[string]any{})

	require.NoError(t, err)
	assert.True(t, out["original"].(bool))
	assert.True(t, out["added"].(bool))
}

func TestMiddleware_PreShortCircuit(t *testing.T) {
	var handlerCalls int
	handler := MiddlewareFunc(func(ctx context.Context, inputs map[string]any) (map[string]any, error) {
		handlerCalls++
		return map[string]any{}, nil
	})

	mw := Middleware(func(ctx context.Context, inputs map[string]any, next MiddlewareFunc) (map[string]any, error) {
		return nil, errors.New("blocked")
	})

	chain := buildChain(handler, []Middleware{mw})
	_, err := chain(context.Background(), map[string]any{})

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "blocked")
	assert.Equal(t, 0, handlerCalls, "handler should not be called when middleware short-circuits")
}

func TestMiddleware_PostRejection(t *testing.T) {
	handler := MiddlewareFunc(func(ctx context.Context, inputs map[string]any) (map[string]any, error) {
		return map[string]any{"result": "ok"}, nil
	})

	mw := Middleware(func(ctx context.Context, inputs map[string]any, next MiddlewareFunc) (map[string]any, error) {
		out, err := next(ctx, inputs)
		if err != nil {
			return nil, err
		}
		return nil, fmt.Errorf("rejected output: %v", out["result"])
	})

	chain := buildChain(handler, []Middleware{mw})
	_, err := chain(context.Background(), map[string]any{})

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "rejected output")
}

func TestMiddleware_ChainOrder(t *testing.T) {
	var order []string

	makeMW := func(name string) Middleware {
		return func(ctx context.Context, inputs map[string]any, next MiddlewareFunc) (map[string]any, error) {
			order = append(order, name+"-pre")
			out, err := next(ctx, inputs)
			order = append(order, name+"-post")
			return out, err
		}
	}

	handler := MiddlewareFunc(func(ctx context.Context, inputs map[string]any) (map[string]any, error) {
		order = append(order, "handler")
		return map[string]any{}, nil
	})

	chain := buildChain(handler, []Middleware{makeMW("outer"), makeMW("middle"), makeMW("inner")})
	_, err := chain(context.Background(), map[string]any{})

	require.NoError(t, err)
	assert.Equal(t, []string{
		"outer-pre", "middle-pre", "inner-pre",
		"handler",
		"inner-post", "middle-post", "outer-post",
	}, order)
}

func TestChainMiddlewares_Empty(t *testing.T) {
	composed := ChainMiddlewares()
	var handlerCalled bool
	next := MiddlewareFunc(func(ctx context.Context, inputs map[string]any) (map[string]any, error) {
		handlerCalled = true
		return map[string]any{"pass": true}, nil
	})

	out, err := composed(context.Background(), map[string]any{}, next)
	require.NoError(t, err)
	assert.True(t, handlerCalled)
	assert.True(t, out["pass"].(bool))
}

func TestChainMiddlewares_Single(t *testing.T) {
	var called bool
	mw := Middleware(func(ctx context.Context, inputs map[string]any, next MiddlewareFunc) (map[string]any, error) {
		called = true
		return next(ctx, inputs)
	})

	composed := ChainMiddlewares(mw)
	handler := MiddlewareFunc(func(ctx context.Context, inputs map[string]any) (map[string]any, error) {
		return map[string]any{}, nil
	})

	_, err := composed(context.Background(), map[string]any{}, handler)
	require.NoError(t, err)
	assert.True(t, called)
}

func TestChainMiddlewares_Multiple(t *testing.T) {
	var order []string

	makeMW := func(name string) Middleware {
		return func(ctx context.Context, inputs map[string]any, next MiddlewareFunc) (map[string]any, error) {
			order = append(order, name+"-pre")
			out, err := next(ctx, inputs)
			order = append(order, name+"-post")
			return out, err
		}
	}

	composed := ChainMiddlewares(makeMW("A"), makeMW("B"), makeMW("C"))
	handler := MiddlewareFunc(func(ctx context.Context, inputs map[string]any) (map[string]any, error) {
		order = append(order, "handler")
		return map[string]any{}, nil
	})

	_, err := composed(context.Background(), map[string]any{}, handler)
	require.NoError(t, err)
	assert.Equal(t, []string{"A-pre", "B-pre", "C-pre", "handler", "C-post", "B-post", "A-post"}, order)
}

// ── Integration Tests: DAG Execution with Middlewares ─────────────────

func TestDAG_NoMiddlewares_Unchanged(t *testing.T) {
	a := &Node{ID: "a", Produces: []string{"x"},
		Run: func(ctx context.Context, in map[string]any) (map[string]any, error) {
			return map[string]any{"x": "from-a"}, nil
		}}
	b := &Node{ID: "b", Consumes: []string{"x"}, Produces: []string{"y"},
		Run: func(ctx context.Context, in map[string]any) (map[string]any, error) {
			return map[string]any{"y": in["x"].(string) + "+b"}, nil
		}}

	d, err := New(a, b)
	require.NoError(t, err)
	out, err := d.Execute(context.Background())
	require.NoError(t, err)
	assert.Equal(t, "from-a+b", out["y"])
}

func TestDAG_WithMiddleware_PreInspection(t *testing.T) {
	var sawInputs map[string]any
	mw := Middleware(func(ctx context.Context, inputs map[string]any, next MiddlewareFunc) (map[string]any, error) {
		sawInputs = inputs
		return next(ctx, inputs)
	})

	a := &Node{ID: "a", Produces: []string{"x"},
		Run: func(ctx context.Context, in map[string]any) (map[string]any, error) {
			return map[string]any{"x": "val"}, nil
		},
		Middlewares: []Middleware{mw},
	}

	d, err := New(a)
	require.NoError(t, err)
	out, err := d.Execute(context.Background())
	require.NoError(t, err)
	assert.Equal(t, "val", out["x"])
	assert.NotNil(t, sawInputs)
}

func TestDAG_WithMiddleware_PreShortCircuit_ErrorPropagates(t *testing.T) {
	mw := Middleware(func(ctx context.Context, inputs map[string]any, next MiddlewareFunc) (map[string]any, error) {
		return nil, errors.New("middleware-blocked")
	})

	a := &Node{ID: "a", Produces: []string{"x"},
		Run: func(ctx context.Context, in map[string]any) (map[string]any, error) {
			return map[string]any{"x": "val"}, nil
		},
		Middlewares: []Middleware{mw},
	}

	d, err := New(a)
	require.NoError(t, err)
	_, err = d.Execute(context.Background())
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "middleware-blocked")
}

func TestDAG_WithMiddleware_PostRejection_TriggersRetry(t *testing.T) {
	var handlerCalls int32
	var mwCalls int32

	// Post-middleware rejects on first invocation, accepts on second.
	// Uses its own call counter (not the handler's) to avoid cross-concern coupling.
	mw := Middleware(func(ctx context.Context, inputs map[string]any, next MiddlewareFunc) (map[string]any, error) {
		out, err := next(ctx, inputs)
		if err != nil {
			return nil, err
		}
		n := atomic.AddInt32(&mwCalls, 1)
		if n == 1 {
			return nil, errors.New("rejected-first")
		}
		return out, nil
	})

	a := &Node{ID: "a", Produces: []string{"x"}, MaxRetries: 1,
		Run: func(ctx context.Context, in map[string]any) (map[string]any, error) {
			atomic.AddInt32(&handlerCalls, 1)
			return map[string]any{"x": "ok"}, nil
		},
		Middlewares: []Middleware{mw},
	}

	d, err := New(a)
	require.NoError(t, err)
	out, err := d.Execute(context.Background())
	require.NoError(t, err)
	assert.Equal(t, "ok", out["x"])
	assert.Equal(t, int32(2), atomic.LoadInt32(&handlerCalls), "handler should be called twice (1 + 1 retry)")
	assert.Equal(t, int32(2), atomic.LoadInt32(&mwCalls), "middleware should be called twice")
}

func TestDAG_WithMiddleware_RetryExhausted(t *testing.T) {
	var calls int32

	mw := Middleware(func(ctx context.Context, inputs map[string]any, next MiddlewareFunc) (map[string]any, error) {
		out, err := next(ctx, inputs)
		if err != nil {
			return nil, err
		}
		_ = out
		return nil, errors.New("always-rejected")
	})

	a := &Node{ID: "a", Produces: []string{"x"}, MaxRetries: 1,
		Run: func(ctx context.Context, in map[string]any) (map[string]any, error) {
			atomic.AddInt32(&calls, 1)
			return map[string]any{"x": "ok"}, nil
		},
		Middlewares: []Middleware{mw},
	}

	d, err := New(a)
	require.NoError(t, err)
	_, err = d.Execute(context.Background())
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "always-rejected")
	assert.Equal(t, int32(2), atomic.LoadInt32(&calls), "should attempt 2 times (1 + 1 retry)")
}

func TestDAG_WithMiddleware_MultipleNodes(t *testing.T) {
	var middleSawInput string

	mw := Middleware(func(ctx context.Context, inputs map[string]any, next MiddlewareFunc) (map[string]any, error) {
		if v, ok := inputs["x"]; ok {
			middleSawInput = v.(string)
		}
		return next(ctx, inputs)
	})

	a := &Node{ID: "a", Produces: []string{"x"},
		Run: func(ctx context.Context, in map[string]any) (map[string]any, error) {
			return map[string]any{"x": "from-a"}, nil
		}}
	b := &Node{ID: "b", Consumes: []string{"x"}, Produces: []string{"y"},
		Run: func(ctx context.Context, in map[string]any) (map[string]any, error) {
			return map[string]any{"y": in["x"].(string) + "+b"}, nil
		},
		Middlewares: []Middleware{mw},
	}
	c := &Node{ID: "c", Consumes: []string{"y"}, Produces: []string{"z"},
		Run: func(ctx context.Context, in map[string]any) (map[string]any, error) {
			return map[string]any{"z": in["y"].(string) + "+c"}, nil
		}}

	d, err := New(a, b, c)
	require.NoError(t, err)
	out, err := d.Execute(context.Background())
	require.NoError(t, err)
	assert.Equal(t, "from-a+b+c", out["z"])
	assert.Equal(t, "from-a", middleSawInput, "middleware on middle node should see input from first node")
}

func TestDAG_WithMiddleware_PerNodeConfig(t *testing.T) {
	var node1MW, node2MW, node3MW bool

	a := &Node{ID: "a", Produces: []string{"x"},
		Run: func(ctx context.Context, in map[string]any) (map[string]any, error) {
			return map[string]any{"x": 1}, nil
		},
		Middlewares: []Middleware{func(ctx context.Context, in map[string]any, next MiddlewareFunc) (map[string]any, error) {
			node1MW = true
			return next(ctx, in)
		}},
	}
	b := &Node{ID: "b", Consumes: []string{"x"}, Produces: []string{"y"},
		Run: func(ctx context.Context, in map[string]any) (map[string]any, error) {
			return map[string]any{"y": 2}, nil
		},
		Middlewares: []Middleware{func(ctx context.Context, in map[string]any, next MiddlewareFunc) (map[string]any, error) {
			node2MW = true
			return next(ctx, in)
		}},
	}
	c := &Node{ID: "c", Consumes: []string{"y"}, Produces: []string{"z"},
		Run: func(ctx context.Context, in map[string]any) (map[string]any, error) {
			return map[string]any{"z": 3}, nil
		},
		Middlewares: []Middleware{func(ctx context.Context, in map[string]any, next MiddlewareFunc) (map[string]any, error) {
			node3MW = true
			return next(ctx, in)
		}},
	}

	d, err := New(a, b, c)
	require.NoError(t, err)
	_, err = d.Execute(context.Background())
	require.NoError(t, err)
	assert.True(t, node1MW, "node a middleware should run")
	assert.True(t, node2MW, "node b middleware should run")
	assert.True(t, node3MW, "node c middleware should run")
}

func TestDAG_WithMiddleware_Timeout(t *testing.T) {
	mw := Middleware(func(ctx context.Context, inputs map[string]any, next MiddlewareFunc) (map[string]any, error) {
		// Verify context has a deadline (from Node.Timeout)
		_, hasDeadline := ctx.Deadline()
		inputs["has_deadline"] = hasDeadline
		return next(ctx, inputs)
	})

	a := &Node{ID: "a", Produces: []string{"x"}, Timeout: 5 * time.Second,
		Run: func(ctx context.Context, in map[string]any) (map[string]any, error) {
			return map[string]any{"x": in["has_deadline"]}, nil
		},
		Middlewares: []Middleware{mw},
	}

	d, err := New(a)
	require.NoError(t, err)
	out, err := d.Execute(context.Background())
	require.NoError(t, err)
	assert.True(t, out["x"].(bool), "middleware should see timeout context")
}

func TestDAG_WithMiddleware_ConcurrentNodes(t *testing.T) {
	var count1, count2 int32

	mw1 := Middleware(func(ctx context.Context, inputs map[string]any, next MiddlewareFunc) (map[string]any, error) {
		atomic.AddInt32(&count1, 1)
		return next(ctx, inputs)
	})
	mw2 := Middleware(func(ctx context.Context, inputs map[string]any, next MiddlewareFunc) (map[string]any, error) {
		atomic.AddInt32(&count2, 1)
		return next(ctx, inputs)
	})

	// Two independent nodes (same layer, run in parallel)
	a := &Node{ID: "a", Produces: []string{"x"},
		Run: func(ctx context.Context, in map[string]any) (map[string]any, error) {
			return map[string]any{"x": 1}, nil
		},
		Middlewares: []Middleware{mw1},
	}
	b := &Node{ID: "b", Produces: []string{"y"},
		Run: func(ctx context.Context, in map[string]any) (map[string]any, error) {
			return map[string]any{"y": 2}, nil
		},
		Middlewares: []Middleware{mw2},
	}

	d, err := New(a, b)
	require.NoError(t, err)
	_, err = d.Execute(context.Background())
	require.NoError(t, err)
	assert.Equal(t, int32(1), atomic.LoadInt32(&count1))
	assert.Equal(t, int32(1), atomic.LoadInt32(&count2))
}
