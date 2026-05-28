package dag_test

import (
	"context"
	"sync/atomic"
	"testing"

	"github.com/mirandaguillaume/reify/pkg/dag"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// --- KindMap ---

func TestKindMap_ParallelPerItem(t *testing.T) {
	d, err := dag.New(
		&dag.Node{
			ID:       "source",
			Produces: []string{"items"},
			Run: func(ctx context.Context, in map[string]any) (map[string]any, error) {
				return map[string]any{"items": []any{1, 2, 3}}, nil
			},
		},
		&dag.Node{
			ID:       "doubler",
			Kind:     dag.KindMap,
			Consumes: []string{"items"},
			Produces: []string{"doubled"},
			Run: func(ctx context.Context, in map[string]any) (map[string]any, error) {
				v := in["items"].(int)
				return map[string]any{"doubled": v * 2}, nil
			},
		},
	)
	require.NoError(t, err)

	results, err := d.Execute(context.Background())
	require.NoError(t, err)
	assert.Equal(t, []any{2, 4, 6}, results["doubled"])
}

func TestKindMap_EmptyList(t *testing.T) {
	d, _ := dag.New(
		&dag.Node{
			ID: "m", Kind: dag.KindMap,
			Consumes: []string{"items"}, Produces: []string{"out"},
			Run: func(ctx context.Context, in map[string]any) (map[string]any, error) {
				return map[string]any{"out": "x"}, nil
			},
		},
	)
	results, err := d.Execute(context.Background(), dag.WithInputs(map[string]any{
		"items": []any{},
	}))
	require.NoError(t, err)
	assert.Equal(t, []any{}, results["out"])
}

func TestKindMap_NoListInput_Error(t *testing.T) {
	d, _ := dag.New(
		&dag.Node{
			ID: "m", Kind: dag.KindMap,
			Consumes: []string{"x"}, Produces: []string{"out"},
			Run: func(ctx context.Context, in map[string]any) (map[string]any, error) {
				return nil, nil
			},
		},
	)
	_, err := d.Execute(context.Background(), dag.WithInputs(map[string]any{"x": "not-a-list"}))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "no []any input")
}

// --- KindFilter ---

func TestKindFilter_KeepEven(t *testing.T) {
	d, _ := dag.New(
		&dag.Node{
			ID: "f", Kind: dag.KindFilter,
			Consumes: []string{"items"}, Produces: []string{"evens"},
			Run: func(ctx context.Context, in map[string]any) (map[string]any, error) {
				v := in["items"].(int)
				return map[string]any{"__keep": v%2 == 0}, nil
			},
		},
	)

	results, err := d.Execute(context.Background(), dag.WithInputs(map[string]any{
		"items": []any{1, 2, 3, 4, 5},
	}))
	require.NoError(t, err)
	assert.Equal(t, []any{2, 4}, results["evens"])
}

func TestKindFilter_KeepNone(t *testing.T) {
	d, _ := dag.New(
		&dag.Node{
			ID: "f", Kind: dag.KindFilter,
			Consumes: []string{"items"}, Produces: []string{"out"},
			Run: func(ctx context.Context, in map[string]any) (map[string]any, error) {
				return map[string]any{"__keep": false}, nil
			},
		},
	)

	results, err := d.Execute(context.Background(), dag.WithInputs(map[string]any{
		"items": []any{"a", "b"},
	}))
	require.NoError(t, err)
	assert.Nil(t, results["out"])
}

// --- KindFlatMap ---

func TestKindFlatMap_Expand(t *testing.T) {
	d, _ := dag.New(
		&dag.Node{
			ID: "fm", Kind: dag.KindFlatMap,
			Consumes: []string{"items"}, Produces: []string{"expanded"},
			Run: func(ctx context.Context, in map[string]any) (map[string]any, error) {
				v := in["items"].(int)
				return map[string]any{"expanded": []any{v, v * 10}}, nil
			},
		},
	)

	results, err := d.Execute(context.Background(), dag.WithInputs(map[string]any{
		"items": []any{1, 2},
	}))
	require.NoError(t, err)
	assert.Equal(t, []any{1, 10, 2, 20}, results["expanded"])
}

// --- KindReduce ---

func TestKindReduce_Sum(t *testing.T) {
	d, _ := dag.New(
		&dag.Node{
			ID: "r", Kind: dag.KindReduce,
			Consumes: []string{"items"}, Produces: []string{"total"},
			Run: func(ctx context.Context, in map[string]any) (map[string]any, error) {
				item := in["items"].(int)
				acc := 0
				if v, ok := in["__accumulator"]; ok && v != nil {
					acc = v.(int)
				}
				return map[string]any{"__accumulator": acc + item}, nil
			},
		},
	)

	results, err := d.Execute(context.Background(), dag.WithInputs(map[string]any{
		"items": []any{1, 2, 3, 4},
	}))
	require.NoError(t, err)
	assert.Equal(t, 10, results["total"])
}

func TestKindReduce_EmptyList(t *testing.T) {
	d, _ := dag.New(
		&dag.Node{
			ID: "r", Kind: dag.KindReduce,
			Consumes: []string{"items"}, Produces: []string{"out"},
			Run: func(ctx context.Context, in map[string]any) (map[string]any, error) {
				return map[string]any{"__accumulator": 0}, nil
			},
		},
	)

	results, err := d.Execute(context.Background(), dag.WithInputs(map[string]any{
		"items": []any{},
	}))
	require.NoError(t, err)
	assert.Nil(t, results["out"])
}

// --- KindBatch ---

func TestKindBatch_ChunkProcessing(t *testing.T) {
	d, _ := dag.New(
		&dag.Node{
			ID: "b", Kind: dag.KindBatch,
			Consumes: []string{"items"}, Produces: []string{"processed"},
			Config:   &dag.NodeConfig{BatchSize: 2},
			Run: func(ctx context.Context, in map[string]any) (map[string]any, error) {
				chunk := in["items"].([]any)
				var results []any
				for _, v := range chunk {
					results = append(results, v.(int)*10)
				}
				return map[string]any{"processed": results}, nil
			},
		},
	)

	results, err := d.Execute(context.Background(), dag.WithInputs(map[string]any{
		"items": []any{1, 2, 3, 4, 5},
	}))
	require.NoError(t, err)
	assert.Equal(t, []any{10, 20, 30, 40, 50}, results["processed"])
}

func TestKindBatch_DefaultBatchSize(t *testing.T) {
	var callCount int32
	d, _ := dag.New(
		&dag.Node{
			ID: "b", Kind: dag.KindBatch,
			Consumes: []string{"items"}, Produces: []string{"out"},
			Run: func(ctx context.Context, in map[string]any) (map[string]any, error) {
				atomic.AddInt32(&callCount, 1)
				chunk := in["items"].([]any)
				return map[string]any{"out": chunk}, nil
			},
		},
	)

	_, err := d.Execute(context.Background(), dag.WithInputs(map[string]any{
		"items": []any{1, 2, 3},
	}))
	require.NoError(t, err)
	// Default batch size is 1, so 3 calls
	assert.Equal(t, int32(3), atomic.LoadInt32(&callCount))
}

// --- KindZip ---

func TestKindZip_PairsElements(t *testing.T) {
	d, _ := dag.New(
		&dag.Node{
			ID:       "src-a",
			Produces: []string{"as"},
			Run: func(ctx context.Context, in map[string]any) (map[string]any, error) {
				return map[string]any{"as": []any{"x", "y", "z"}}, nil
			},
		},
		&dag.Node{
			ID:       "src-b",
			Produces: []string{"bs"},
			Run: func(ctx context.Context, in map[string]any) (map[string]any, error) {
				return map[string]any{"bs": []any{1, 2, 3}}, nil
			},
		},
		&dag.Node{
			ID: "z", Kind: dag.KindZip,
			Consumes: []string{"as", "bs"}, Produces: []string{"pairs"},
		},
	)

	results, err := d.Execute(context.Background())
	require.NoError(t, err)
	pairs := results["pairs"].([]any)
	require.Len(t, pairs, 3)
	assert.Equal(t, []any{"x", 1}, pairs[0])
	assert.Equal(t, []any{"y", 2}, pairs[1])
	assert.Equal(t, []any{"z", 3}, pairs[2])
}

func TestKindZip_UnequalLengths_TruncatesToShorter(t *testing.T) {
	d, _ := dag.New(
		&dag.Node{
			ID: "z", Kind: dag.KindZip,
			Consumes: []string{"as", "bs"}, Produces: []string{"pairs"},
		},
	)

	results, err := d.Execute(context.Background(), dag.WithInputs(map[string]any{
		"as": []any{1, 2, 3},
		"bs": []any{"a", "b"},
	}))
	require.NoError(t, err)
	pairs := results["pairs"].([]any)
	assert.Len(t, pairs, 2)
}

// --- KindMap in pipeline ---

func TestKindMap_InPipeline(t *testing.T) {
	d, err := dag.New(
		&dag.Node{
			ID:       "produce",
			Produces: []string{"items"},
			Run: func(ctx context.Context, in map[string]any) (map[string]any, error) {
				return map[string]any{"items": []any{1, 2, 3}}, nil
			},
		},
		&dag.Node{
			ID: "mapper", Kind: dag.KindMap,
			Consumes: []string{"items"}, Produces: []string{"mapped"},
			Run: func(ctx context.Context, in map[string]any) (map[string]any, error) {
				v := in["items"].(int)
				return map[string]any{"mapped": v + 100}, nil
			},
		},
		&dag.Node{
			ID: "reducer", Kind: dag.KindReduce,
			Consumes: []string{"mapped"}, Produces: []string{"total"},
			Run: func(ctx context.Context, in map[string]any) (map[string]any, error) {
				item := in["mapped"].(int)
				acc := 0
				if v, ok := in["__accumulator"]; ok && v != nil {
					acc = v.(int)
				}
				return map[string]any{"__accumulator": acc + item}, nil
			},
		},
	)
	require.NoError(t, err)

	results, err := d.Execute(context.Background())
	require.NoError(t, err)
	// (1+100) + (2+100) + (3+100) = 306
	assert.Equal(t, 306, results["total"])
}
