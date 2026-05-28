package dag_test

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/mirandaguillaume/reify/pkg/dag"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// runNode is a test helper: creates a node whose Run sets each produced type to the value from extra,
// or to "<id>_output" if not in extra.
func runNode(id string, consumes, produces []string, extra map[string]any) *dag.Node {
	n := &dag.Node{ID: id, Consumes: consumes, Produces: produces}
	n.Run = func(ctx context.Context, inputs map[string]any) (map[string]any, error) {
		out := make(map[string]any, len(produces))
		for _, p := range produces {
			if v, ok := extra[p]; ok {
				out[p] = v
			} else {
				out[p] = id + "_output"
			}
		}
		return out, nil
	}
	return n
}

func TestExecute_Pipeline(t *testing.T) {
	var order []string
	mkNode := func(id string, consumes, produces []string) *dag.Node {
		n := &dag.Node{ID: id, Consumes: consumes, Produces: produces}
		n.Run = func(ctx context.Context, inputs map[string]any) (map[string]any, error) {
			order = append(order, id)
			out := map[string]any{}
			for _, p := range produces {
				out[p] = id
			}
			return out, nil
		}
		return n
	}

	d, _ := dag.New(
		mkNode("a", nil, []string{"x"}),
		mkNode("b", []string{"x"}, []string{"y"}),
		mkNode("c", []string{"y"}, []string{"z"}),
	)
	results, err := d.Execute(context.Background())
	require.NoError(t, err)

	require.Equal(t, []string{"a", "b", "c"}, order)
	assert.Equal(t, "c", results["z"])
}

func TestExecute_Diamond_Parallel(t *testing.T) {
	d, _ := dag.New(
		runNode("a", nil, []string{"x"}, map[string]any{"x": "from-a"}),
		runNode("b", []string{"x"}, []string{"y"}, map[string]any{"y": "from-b"}),
		runNode("c", []string{"x"}, []string{"z"}, map[string]any{"z": "from-c"}),
		runNode("dd", []string{"y", "z"}, []string{"out"}, map[string]any{"out": "merged"}),
	)
	results, err := d.Execute(context.Background())
	require.NoError(t, err)
	assert.Equal(t, "merged", results["out"])
}

func TestExecute_WithInitialInputs(t *testing.T) {
	b := &dag.Node{
		ID:       "b",
		Consumes: []string{"req"},
		Produces: []string{"resp"},
		Run: func(ctx context.Context, inputs map[string]any) (map[string]any, error) {
			v, _ := inputs["req"].(string)
			return map[string]any{"resp": "got:" + v}, nil
		},
	}
	d, _ := dag.New(b)
	results, err := d.Execute(context.Background(),
		dag.WithInputs(map[string]any{"req": "hello"}),
	)
	require.NoError(t, err)
	assert.Equal(t, "got:hello", results["resp"])
}

func TestExecute_Router_S3(t *testing.T) {
	router := &dag.Node{
		ID:       "router",
		Kind:     dag.KindRouter,
		Consumes: []string{"score"},
		Produces: []string{"__route"},
		Run: func(ctx context.Context, inputs map[string]any) (map[string]any, error) {
			score, _ := inputs["score"].(int)
			if score > 50 {
				return map[string]any{"__route": "left"}, nil
			}
			return map[string]any{"__route": "right"}, nil
		},
	}
	left := runNode("left", nil, []string{"left_out"}, map[string]any{"left_out": "HIGH"})
	right := runNode("right", nil, []string{"right_out"}, map[string]any{"right_out": "LOW"})

	d, _ := dag.New(router, left, right)
	require.NoError(t, d.AddEdge("router", "left"))
	require.NoError(t, d.AddEdge("router", "right"))

	results, err := d.Execute(context.Background(),
		dag.WithInputs(map[string]any{"score": 90}),
	)
	require.NoError(t, err)
	assert.Equal(t, "HIGH", results["left_out"])
	assert.Nil(t, results["right_out"], "right branch should be skipped")
}

func TestExecute_FailFast(t *testing.T) {
	fail := &dag.Node{
		ID:       "fail",
		Produces: []string{"x"},
		Run: func(ctx context.Context, inputs map[string]any) (map[string]any, error) {
			return nil, errors.New("boom")
		},
	}
	next := runNode("next", []string{"x"}, []string{"y"}, nil)
	d, _ := dag.New(fail, next)

	_, err := d.Execute(context.Background())
	require.Error(t, err)
	assert.Contains(t, err.Error(), "boom")
}

func TestExecute_OnNodeComplete(t *testing.T) {
	var completed []string
	var mu sync.Mutex

	d, _ := dag.New(
		runNode("a", nil, []string{"x"}, nil),
		runNode("b", []string{"x"}, []string{"y"}, nil),
	)
	_, err := d.Execute(context.Background(),
		dag.WithOnNodeComplete(func(nodeID string) {
			mu.Lock()
			completed = append(completed, nodeID)
			mu.Unlock()
		}),
	)
	require.NoError(t, err)
	assert.ElementsMatch(t, []string{"a", "b"}, completed)
}

func TestExecute_NilRun_Skipped(t *testing.T) {
	// Nodes with nil Run should be skipped gracefully (used by formal.Graph analysis nodes)
	n := &dag.Node{ID: "n", Produces: []string{"x"}} // Run is nil
	d, _ := dag.New(n)
	results, err := d.Execute(context.Background())
	require.NoError(t, err)
	_ = results // nil Run produces no output
}

func TestExecute_ContextCancelled(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // pre-cancel

	slow := &dag.Node{
		ID:       "slow",
		Produces: []string{"x"},
		Run: func(ctx context.Context, inputs map[string]any) (map[string]any, error) {
			select {
			case <-time.After(1 * time.Second):
				return map[string]any{"x": "done"}, nil
			case <-ctx.Done():
				return nil, ctx.Err()
			}
		},
	}
	d, _ := dag.New(slow)
	_, err := d.Execute(ctx)
	assert.Error(t, err)
}

func TestExecute_Retry_S4_SucceedsOnThirdAttempt(t *testing.T) {
	attempts := 0
	flaky := &dag.Node{
		ID:         "flaky",
		Produces:   []string{"x"},
		MaxRetries: 2,
		Run: func(ctx context.Context, inputs map[string]any) (map[string]any, error) {
			attempts++
			if attempts < 3 {
				return nil, errors.New("transient")
			}
			return map[string]any{"x": "ok"}, nil
		},
	}
	d, _ := dag.New(flaky)
	results, err := d.Execute(context.Background())
	require.NoError(t, err)
	assert.Equal(t, "ok", results["x"])
	assert.Equal(t, 3, attempts, "should have tried 3 times (1 initial + 2 retries)")
}

func TestExecute_Retry_S4_ExhaustedReturnsError(t *testing.T) {
	calls := 0
	n := &dag.Node{
		ID:         "n",
		Produces:   []string{"x"},
		MaxRetries: 1,
		Run: func(ctx context.Context, inputs map[string]any) (map[string]any, error) {
			calls++
			return nil, errors.New("always fails")
		},
	}
	d, _ := dag.New(n)
	_, err := d.Execute(context.Background())
	require.Error(t, err)
	assert.Contains(t, err.Error(), "always fails")
	assert.Equal(t, 2, calls, "should have tried 2 times (1 initial + 1 retry)")
}

func TestExecute_Timeout_S5_NodeCancelled(t *testing.T) {
	slow := &dag.Node{
		ID:       "slow",
		Produces: []string{"x"},
		Timeout:  10 * time.Millisecond,
		Run: func(ctx context.Context, inputs map[string]any) (map[string]any, error) {
			select {
			case <-time.After(1 * time.Second):
				return map[string]any{"x": "done"}, nil
			case <-ctx.Done():
				return nil, ctx.Err()
			}
		},
	}
	d, _ := dag.New(slow)
	_, err := d.Execute(context.Background())
	require.Error(t, err)
	assert.Contains(t, err.Error(), "deadline exceeded")
}

func TestExecute_BoundedCycle_IteratesCorrectly(t *testing.T) {
	var mu sync.Mutex
	callCount := 0

	d, err := dag.New(
		&dag.Node{
			ID:       "produce",
			Produces: []string{"val"},
			Run: func(ctx context.Context, inputs map[string]any) (map[string]any, error) {
				mu.Lock()
				callCount++
				c := callCount
				mu.Unlock()
				return map[string]any{"val": c}, nil
			},
		},
		&dag.Node{
			ID:       "consume",
			Consumes: []string{"val"},
			Produces: []string{"result"},
			Run: func(ctx context.Context, inputs map[string]any) (map[string]any, error) {
				v := inputs["val"].(int)
				return map[string]any{"result": v * 10}, nil
			},
		},
	)
	require.NoError(t, err)

	// Back-edge: consume → produce, max 2 iterations
	require.NoError(t, d.AddBackEdge("consume", "produce", 2))

	results, err := d.Execute(context.Background())
	require.NoError(t, err)

	// produce runs 3 times (1 initial + 2 iterations), consume runs 3 times
	assert.Equal(t, 3, callCount)
	// Final result is from the last iteration
	assert.Equal(t, 30, results["result"])
}

func TestExecute_BoundedCycle_OnNodeComplete(t *testing.T) {
	var completions []string
	var mu sync.Mutex

	d, err := dag.New(
		&dag.Node{
			ID:       "a",
			Produces: []string{"x"},
			Run: func(ctx context.Context, inputs map[string]any) (map[string]any, error) {
				return map[string]any{"x": "data"}, nil
			},
		},
		&dag.Node{
			ID:       "b",
			Consumes: []string{"x"},
			Produces: []string{"y"},
			Run: func(ctx context.Context, inputs map[string]any) (map[string]any, error) {
				return map[string]any{"y": "done"}, nil
			},
		},
	)
	require.NoError(t, err)
	require.NoError(t, d.AddBackEdge("b", "a", 1))

	_, err = d.Execute(context.Background(), dag.WithOnNodeComplete(func(nodeID string) {
		mu.Lock()
		completions = append(completions, nodeID)
		mu.Unlock()
	}))
	require.NoError(t, err)

	// 2 passes × 2 nodes = 4 completions
	assert.Len(t, completions, 4)
}

func TestExecute_EventDriven_NoCycle_SameResultAsLayerBased(t *testing.T) {
	// Verify that a simple pipeline works the same with event-driven (triggered by a back-edge)
	d, err := dag.New(
		&dag.Node{
			ID:       "a",
			Produces: []string{"x"},
			Run: func(ctx context.Context, inputs map[string]any) (map[string]any, error) {
				return map[string]any{"x": 1}, nil
			},
		},
		&dag.Node{
			ID:       "b",
			Consumes: []string{"x"},
			Produces: []string{"y"},
			Run: func(ctx context.Context, inputs map[string]any) (map[string]any, error) {
				return map[string]any{"y": inputs["x"].(int) + 1}, nil
			},
		},
		&dag.Node{
			ID:       "c",
			Consumes: []string{"y"},
			Produces: []string{"z"},
			Run: func(ctx context.Context, inputs map[string]any) (map[string]any, error) {
				return map[string]any{"z": inputs["y"].(int) + 1}, nil
			},
		},
	)
	require.NoError(t, err)
	// Add back-edge with MaxIterations=1 so event-driven is used
	require.NoError(t, d.AddBackEdge("c", "a", 1))

	results, err := d.Execute(context.Background())
	require.NoError(t, err)
	// After 2 passes: z = 3 (first pass), then z = 3 again (inputs refresh from store)
	assert.Equal(t, 3, results["z"])
}

func TestExecute_BoundedCycle_ErrorStopsExecution(t *testing.T) {
	callCount := 0
	d, err := dag.New(
		&dag.Node{
			ID:       "a",
			Produces: []string{"x"},
			Run: func(ctx context.Context, inputs map[string]any) (map[string]any, error) {
				callCount++
				if callCount > 1 {
					return nil, errors.New("fail on second pass")
				}
				return map[string]any{"x": 1}, nil
			},
		},
		&dag.Node{
			ID:       "b",
			Consumes: []string{"x"},
			Produces: []string{"y"},
			Run: func(ctx context.Context, inputs map[string]any) (map[string]any, error) {
				return map[string]any{"y": 2}, nil
			},
		},
	)
	require.NoError(t, err)
	require.NoError(t, d.AddBackEdge("b", "a", 3))

	_, err = d.Execute(context.Background())
	require.Error(t, err)
	assert.Contains(t, err.Error(), "fail on second pass")
}

func TestExecute_TrueStreaming(t *testing.T) {
	// Verify that a node whose only dependency is satisfied starts immediately,
	// without waiting for a concurrent slow sibling node to finish.
	// The old layer-based path would have blocked C until both A and B (same layer) completed.
	var mu sync.Mutex
	var cStarted, bFinished time.Time

	a := &dag.Node{
		ID:       "a",
		Produces: []string{"x"},
		Run: func(ctx context.Context, _ map[string]any) (map[string]any, error) {
			return map[string]any{"x": 1}, nil
		},
	}
	b := &dag.Node{
		ID:       "b",
		Produces: []string{"y"},
		Run: func(ctx context.Context, _ map[string]any) (map[string]any, error) {
			time.Sleep(100 * time.Millisecond)
			mu.Lock()
			bFinished = time.Now()
			mu.Unlock()
			return map[string]any{"y": 2}, nil
		},
	}
	c := &dag.Node{
		ID:       "c",
		Consumes: []string{"x"},
		Produces: []string{"z"},
		Run: func(ctx context.Context, _ map[string]any) (map[string]any, error) {
			mu.Lock()
			cStarted = time.Now()
			mu.Unlock()
			return map[string]any{"z": 3}, nil
		},
	}

	d, err := dag.New(a, b, c)
	require.NoError(t, err)
	_, err = d.Execute(context.Background())
	require.NoError(t, err)

	mu.Lock()
	cs, bf := cStarted, bFinished
	mu.Unlock()

	assert.True(t, cs.Before(bf),
		"true streaming: C (ready after A) should start before slow B finishes (C=%v B=%v)", cs, bf)
}

func TestExecute_EventDriven_KindMap(t *testing.T) {
	// Regression: KindMap was silently broken when executeEventDriven called
	// runWithRetry instead of runNode, bypassing the per-item dispatch in runMap.
	source := &dag.Node{
		ID:       "source",
		Produces: []string{"items"},
		Run: func(ctx context.Context, _ map[string]any) (map[string]any, error) {
			return map[string]any{"items": []any{1, 2, 3}}, nil
		},
	}
	mapper := &dag.Node{
		ID:       "mapper",
		Kind:     dag.KindMap,
		Consumes: []string{"items"},
		Produces: []string{"results"},
		Run: func(ctx context.Context, inputs map[string]any) (map[string]any, error) {
			v, _ := inputs["items"].(int)
			return map[string]any{"results": v * 2}, nil
		},
	}
	d, err := dag.New(source, mapper)
	require.NoError(t, err)

	results, err := d.Execute(context.Background())
	require.NoError(t, err)
	got, _ := results["results"].([]any)
	assert.Equal(t, []any{2, 4, 6}, got)
}
