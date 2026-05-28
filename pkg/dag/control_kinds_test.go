package dag_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/mirandaguillaume/reify/pkg/dag"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// --- KindGate ---

func TestKindGate_Pass(t *testing.T) {
	d, _ := dag.New(
		&dag.Node{
			ID: "gate", Kind: dag.KindGate,
			Consumes: []string{"x"}, Produces: []string{"y"},
			Run: func(ctx context.Context, in map[string]any) (map[string]any, error) {
				return map[string]any{"__halt": false, "y": "passed"}, nil
			},
		},
	)

	results, err := d.Execute(context.Background(), dag.WithInputs(map[string]any{"x": 1}))
	require.NoError(t, err)
	assert.Equal(t, "passed", results["y"])
}

func TestKindGate_Halt(t *testing.T) {
	d, _ := dag.New(
		&dag.Node{
			ID: "gate", Kind: dag.KindGate,
			Consumes: []string{"x"}, Produces: []string{"gate_out"},
			Run: func(ctx context.Context, in map[string]any) (map[string]any, error) {
				return map[string]any{"__halt": true, "gate_out": "halted"}, nil
			},
		},
		&dag.Node{
			ID:       "after-gate",
			Consumes: []string{"gate_out"}, Produces: []string{"y"},
			Run: func(ctx context.Context, in map[string]any) (map[string]any, error) {
				t.Fatal("should not reach here")
				return nil, nil
			},
		},
	)

	_, err := d.Execute(context.Background(), dag.WithInputs(map[string]any{"x": 1}))
	require.Error(t, err)
	var haltErr *dag.HaltError
	assert.True(t, errors.As(err, &haltErr))
	assert.Equal(t, "gate", haltErr.NodeID)
}

func TestKindGate_InPipeline(t *testing.T) {
	d, _ := dag.New(
		&dag.Node{
			ID:       "source",
			Produces: []string{"val"},
			Run: func(ctx context.Context, in map[string]any) (map[string]any, error) {
				return map[string]any{"val": 42}, nil
			},
		},
		&dag.Node{
			ID: "check", Kind: dag.KindGate,
			Consumes: []string{"val"}, Produces: []string{"checked"},
			Run: func(ctx context.Context, in map[string]any) (map[string]any, error) {
				v := in["val"].(int)
				if v > 100 {
					return map[string]any{"__halt": true}, nil
				}
				return map[string]any{"checked": v}, nil
			},
		},
		&dag.Node{
			ID:       "sink",
			Consumes: []string{"checked"}, Produces: []string{"result"},
			Run: func(ctx context.Context, in map[string]any) (map[string]any, error) {
				return map[string]any{"result": "ok"}, nil
			},
		},
	)

	results, err := d.Execute(context.Background())
	require.NoError(t, err)
	assert.Equal(t, "ok", results["result"])
}

// --- KindFallback ---

func TestKindFallback_PrimarySucceeds(t *testing.T) {
	d, _ := dag.New(
		&dag.Node{
			ID: "fb", Kind: dag.KindFallback,
			Produces: []string{"out"},
			Config: &dag.NodeConfig{Fallbacks: []dag.RunFunc{
				func(ctx context.Context, in map[string]any) (map[string]any, error) {
					return map[string]any{"out": "backup"}, nil
				},
			}},
			Run: func(ctx context.Context, in map[string]any) (map[string]any, error) {
				return map[string]any{"out": "primary"}, nil
			},
		},
	)

	results, err := d.Execute(context.Background())
	require.NoError(t, err)
	assert.Equal(t, "primary", results["out"])
}

func TestKindFallback_PrimaryFailsBackupSucceeds(t *testing.T) {
	d, _ := dag.New(
		&dag.Node{
			ID: "fb", Kind: dag.KindFallback,
			Produces: []string{"out"},
			Config: &dag.NodeConfig{Fallbacks: []dag.RunFunc{
				func(ctx context.Context, in map[string]any) (map[string]any, error) {
					return map[string]any{"out": "backup-result"}, nil
				},
			}},
			Run: func(ctx context.Context, in map[string]any) (map[string]any, error) {
				return nil, errors.New("primary failed")
			},
		},
	)

	results, err := d.Execute(context.Background())
	require.NoError(t, err)
	assert.Equal(t, "backup-result", results["out"])
}

func TestKindFallback_AllFail(t *testing.T) {
	d, _ := dag.New(
		&dag.Node{
			ID: "fb", Kind: dag.KindFallback,
			Produces: []string{"out"},
			Config: &dag.NodeConfig{Fallbacks: []dag.RunFunc{
				func(ctx context.Context, in map[string]any) (map[string]any, error) {
					return nil, errors.New("backup also failed")
				},
			}},
			Run: func(ctx context.Context, in map[string]any) (map[string]any, error) {
				return nil, errors.New("primary failed")
			},
		},
	)

	_, err := d.Execute(context.Background())
	require.Error(t, err)
	assert.Contains(t, err.Error(), "all fallbacks exhausted")
}

// --- KindRace ---

func TestKindRace_FastestWins(t *testing.T) {
	d, _ := dag.New(
		&dag.Node{
			ID: "racer", Kind: dag.KindRace,
			Produces: []string{"out"},
			Config: &dag.NodeConfig{Competitors: []dag.RunFunc{
				func(ctx context.Context, in map[string]any) (map[string]any, error) {
					return map[string]any{"out": "fast-wins"}, nil
				},
			}},
			Run: func(ctx context.Context, in map[string]any) (map[string]any, error) {
				select {
				case <-time.After(1 * time.Second):
					return map[string]any{"out": "slow"}, nil
				case <-ctx.Done():
					return nil, ctx.Err()
				}
			},
		},
	)

	results, err := d.Execute(context.Background())
	require.NoError(t, err)
	assert.Equal(t, "fast-wins", results["out"])
}

func TestKindRace_PrimaryWins(t *testing.T) {
	d, _ := dag.New(
		&dag.Node{
			ID: "racer", Kind: dag.KindRace,
			Produces: []string{"out"},
			Config: &dag.NodeConfig{Competitors: []dag.RunFunc{
				func(ctx context.Context, in map[string]any) (map[string]any, error) {
					select {
					case <-time.After(1 * time.Second):
						return map[string]any{"out": "slow"}, nil
					case <-ctx.Done():
						return nil, ctx.Err()
					}
				},
			}},
			Run: func(ctx context.Context, in map[string]any) (map[string]any, error) {
				return map[string]any{"out": "primary-wins"}, nil
			},
		},
	)

	results, err := d.Execute(context.Background())
	require.NoError(t, err)
	assert.Equal(t, "primary-wins", results["out"])
}

func TestKindRace_AllFail(t *testing.T) {
	d, _ := dag.New(
		&dag.Node{
			ID: "racer", Kind: dag.KindRace,
			Produces: []string{"out"},
			Config: &dag.NodeConfig{Competitors: []dag.RunFunc{
				func(ctx context.Context, in map[string]any) (map[string]any, error) {
					return nil, errors.New("competitor fail")
				},
			}},
			Run: func(ctx context.Context, in map[string]any) (map[string]any, error) {
				return nil, errors.New("primary fail")
			},
		},
	)

	_, err := d.Execute(context.Background())
	require.Error(t, err)
	assert.Contains(t, err.Error(), "all competitors failed")
}

func TestKindRace_Timeout(t *testing.T) {
	d, _ := dag.New(
		&dag.Node{
			ID: "racer", Kind: dag.KindRace,
			Produces: []string{"out"},
			Config: &dag.NodeConfig{
				RaceTimeout: 10 * time.Millisecond,
			},
			Run: func(ctx context.Context, in map[string]any) (map[string]any, error) {
				select {
				case <-time.After(5 * time.Second):
					return map[string]any{"out": "done"}, nil
				case <-ctx.Done():
					return nil, ctx.Err()
				}
			},
		},
	)

	_, err := d.Execute(context.Background())
	require.Error(t, err)
}

// --- KindCache ---

func TestKindCache_Hit(t *testing.T) {
	dag.ClearCache()
	callCount := 0

	d, _ := dag.New(
		&dag.Node{
			ID: "cached", Kind: dag.KindCache,
			Consumes: []string{"key"}, Produces: []string{"val"},
			Config: &dag.NodeConfig{
				CacheKeyFunc: func(inputs map[string]any) string {
					return inputs["key"].(string)
				},
			},
			Run: func(ctx context.Context, in map[string]any) (map[string]any, error) {
				callCount++
				return map[string]any{"val": "computed"}, nil
			},
		},
	)

	// First call: cache miss
	results, err := d.Execute(context.Background(), dag.WithInputs(map[string]any{"key": "k1"}))
	require.NoError(t, err)
	assert.Equal(t, "computed", results["val"])
	assert.Equal(t, 1, callCount)

	// Second call: cache hit
	results, err = d.Execute(context.Background(), dag.WithInputs(map[string]any{"key": "k1"}))
	require.NoError(t, err)
	assert.Equal(t, "computed", results["val"])
	assert.Equal(t, 1, callCount) // Run NOT called again
}

func TestKindCache_DifferentKeys(t *testing.T) {
	dag.ClearCache()
	callCount := 0

	d, _ := dag.New(
		&dag.Node{
			ID: "cached", Kind: dag.KindCache,
			Consumes: []string{"key"}, Produces: []string{"val"},
			Config: &dag.NodeConfig{
				CacheKeyFunc: func(inputs map[string]any) string {
					return inputs["key"].(string)
				},
			},
			Run: func(ctx context.Context, in map[string]any) (map[string]any, error) {
				callCount++
				return map[string]any{"val": in["key"]}, nil
			},
		},
	)

	d.Execute(context.Background(), dag.WithInputs(map[string]any{"key": "k1"}))
	d.Execute(context.Background(), dag.WithInputs(map[string]any{"key": "k2"}))
	assert.Equal(t, 2, callCount) // Two different keys = two misses
}

func TestKindCache_DefaultKeyFunc(t *testing.T) {
	dag.ClearCache()
	callCount := 0

	d, _ := dag.New(
		&dag.Node{
			ID: "cached", Kind: dag.KindCache,
			Consumes: []string{"x"}, Produces: []string{"y"},
			Run: func(ctx context.Context, in map[string]any) (map[string]any, error) {
				callCount++
				return map[string]any{"y": "result"}, nil
			},
		},
	)

	d.Execute(context.Background(), dag.WithInputs(map[string]any{"x": 1}))
	d.Execute(context.Background(), dag.WithInputs(map[string]any{"x": 1}))
	assert.Equal(t, 1, callCount) // Default key func uses Sprint, same inputs = cache hit
}

func TestClearCache(t *testing.T) {
	dag.ClearCache()
	callCount := 0

	d, _ := dag.New(
		&dag.Node{
			ID: "cached", Kind: dag.KindCache,
			Consumes: []string{"x"}, Produces: []string{"y"},
			Config: &dag.NodeConfig{
				CacheKeyFunc: func(inputs map[string]any) string { return "fixed" },
			},
			Run: func(ctx context.Context, in map[string]any) (map[string]any, error) {
				callCount++
				return map[string]any{"y": "v"}, nil
			},
		},
	)

	d.Execute(context.Background(), dag.WithInputs(map[string]any{"x": 1}))
	assert.Equal(t, 1, callCount)

	dag.ClearCache()

	d.Execute(context.Background(), dag.WithInputs(map[string]any{"x": 1}))
	assert.Equal(t, 2, callCount) // After clear, cache miss
}
