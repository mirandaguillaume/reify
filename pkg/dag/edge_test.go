package dag

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestEdge_IsBackEdge(t *testing.T) {
	forward := &Edge{From: "a", To: "b", MaxIterations: 0}
	back := &Edge{From: "b", To: "a", MaxIterations: 3}
	assert.False(t, forward.IsBackEdge())
	assert.True(t, back.IsBackEdge())
}

func TestAddBackEdge(t *testing.T) {
	d, err := New(
		&Node{ID: "a", Produces: []string{"x"}},
		&Node{ID: "b", Consumes: []string{"x"}, Produces: []string{"y"}},
	)
	require.NoError(t, err)

	err = d.AddBackEdge("b", "a", 5)
	require.NoError(t, err)

	// Back-edge is visible in Downstream/Upstream
	assert.Contains(t, d.Downstream("b"), "a")
	assert.Contains(t, d.Upstream("a"), "b")

	// HasBackEdges reports true
	assert.True(t, d.HasBackEdges())

	// Forward-edge subgraph has no cycle
	assert.False(t, d.HasCycle())
}

func TestAddBackEdge_RequiresPositiveMaxIter(t *testing.T) {
	d, _ := New(
		&Node{ID: "a"},
		&Node{ID: "b"},
	)
	err := d.AddBackEdge("a", "b", 0)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "MaxIterations > 0")
}

func TestAddBackEdge_DuplicateEdge(t *testing.T) {
	d, _ := New(
		&Node{ID: "a", Produces: []string{"x"}},
		&Node{ID: "b", Consumes: []string{"x"}},
	)
	// a→b already exists as forward edge
	err := d.AddBackEdge("a", "b", 3)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "already exists")
}

func TestAddBackEdge_UnknownNode(t *testing.T) {
	d, _ := New(&Node{ID: "a"})
	assert.Error(t, d.AddBackEdge("a", "z", 1))
	assert.Error(t, d.AddBackEdge("z", "a", 1))
}

func TestForwardEdges(t *testing.T) {
	d, _ := New(
		&Node{ID: "a", Produces: []string{"x"}},
		&Node{ID: "b", Consumes: []string{"x"}, Produces: []string{"y"}},
	)
	require.NoError(t, d.AddBackEdge("b", "a", 3))

	// a has 1 forward edge (a→b), b has 1 back edge (b→a)
	fwd := d.ForwardEdges("a")
	assert.Len(t, fwd, 1)
	assert.Equal(t, "b", fwd[0].To)

	// b's forward edges: none (only back-edge to a)
	assert.Empty(t, d.ForwardEdges("b"))
}

func TestEdges_ReturnsAll(t *testing.T) {
	d, _ := New(
		&Node{ID: "a", Produces: []string{"x"}},
		&Node{ID: "b", Consumes: []string{"x"}, Produces: []string{"y"}},
		&Node{ID: "c", Consumes: []string{"y"}},
	)
	require.NoError(t, d.AddBackEdge("c", "a", 2))

	edges := d.Edges()
	// a→b (auto-wired), b→c (auto-wired), c→a (back-edge) = 3
	assert.Len(t, edges, 3)
}

func TestLayers_IgnoresBackEdges(t *testing.T) {
	d, _ := New(
		&Node{ID: "a", Produces: []string{"x"}},
		&Node{ID: "b", Consumes: []string{"x"}, Produces: []string{"y"}},
		&Node{ID: "c", Consumes: []string{"y"}},
	)
	require.NoError(t, d.AddBackEdge("c", "a", 2))

	layers := d.Layers()
	// Same layering as without back-edge: [a] [b] [c]
	require.Len(t, layers, 3)
	assert.Equal(t, []string{"a"}, layers[0])
	assert.Equal(t, []string{"b"}, layers[1])
	assert.Equal(t, []string{"c"}, layers[2])
}
