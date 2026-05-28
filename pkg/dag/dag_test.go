package dag_test

import (
	"testing"

	"github.com/mirandaguillaume/reify/pkg/dag"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func node(id string, consumes, produces []string) *dag.Node {
	return &dag.Node{ID: id, Consumes: consumes, Produces: produces}
}

func TestNew_Pipeline(t *testing.T) {
	a := node("a", nil, []string{"x"})
	b := node("b", []string{"x"}, []string{"y"})
	c := node("c", []string{"y"}, []string{"z"})

	d, err := dag.New(a, b, c)
	require.NoError(t, err)

	assert.Contains(t, d.Downstream("a"), "b")
	assert.Contains(t, d.Downstream("b"), "c")
	assert.Empty(t, d.Upstream("a"))
}

func TestNew_Diamond(t *testing.T) {
	a := node("a", nil, []string{"x"})
	b := node("b", []string{"x"}, []string{"y"})
	c := node("c", []string{"x"}, []string{"z"})
	d := node("d", []string{"y", "z"}, []string{"out"})

	g, err := dag.New(a, b, c, d)
	require.NoError(t, err)

	assert.Contains(t, g.Downstream("a"), "b")
	assert.Contains(t, g.Downstream("a"), "c")
	assert.Contains(t, g.Downstream("b"), "d")
	assert.Contains(t, g.Downstream("c"), "d")
}

func TestNew_DuplicateID(t *testing.T) {
	_, err := dag.New(
		node("a", nil, []string{"x"}),
		node("a", []string{"x"}, []string{"y"}),
	)
	require.Error(t, err)
}

func TestAddRemoveEdge(t *testing.T) {
	a := node("a", nil, []string{"x"})
	b := node("b", nil, []string{"y"})
	g, err := dag.New(a, b)
	require.NoError(t, err)

	assert.Empty(t, g.Downstream("a"))

	require.NoError(t, g.AddEdge("a", "b"))
	assert.Contains(t, g.Downstream("a"), "b")

	require.NoError(t, g.RemoveEdge("a", "b"))
	assert.Empty(t, g.Downstream("a"))
}

func TestRemoveEdge_UnknownNode(t *testing.T) {
	g, _ := dag.New(node("a", nil, []string{"x"}))
	assert.Error(t, g.RemoveEdge("a", "nonexistent"))
	assert.Error(t, g.RemoveEdge("nonexistent", "a"))
}

func TestNew_DuplicateProducer(t *testing.T) {
	_, err := dag.New(
		node("a", nil, []string{"x"}),
		node("b", nil, []string{"x"}), // also produces x
	)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "x")
}

func TestAddEdge_PreventsCycle(t *testing.T) {
	a := node("a", nil, []string{"x"})
	b := node("b", []string{"x"}, []string{"y"})
	g, _ := dag.New(a, b) // a→b auto-wired
	err := g.AddEdge("b", "a") // would create cycle b→a
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "cycle")
}

// --- Topology layer tests (T1-T10) ---

func TestLayers_T1_Pipeline(t *testing.T) {
	d, _ := dag.New(
		node("a", nil, []string{"x"}),
		node("b", []string{"x"}, []string{"y"}),
		node("c", []string{"y"}, []string{"z"}),
	)
	layers := d.Layers()
	require.Len(t, layers, 3)
	assert.Contains(t, layers[0], "a")
	assert.Contains(t, layers[1], "b")
	assert.Contains(t, layers[2], "c")
}

func TestLayers_T2_FanOut(t *testing.T) {
	d, _ := dag.New(
		node("a", nil, []string{"x"}),
		node("b", []string{"x"}, []string{"p"}),
		node("c", []string{"x"}, []string{"q"}),
		node("dd", []string{"x"}, []string{"r"}),
	)
	layers := d.Layers()
	require.Len(t, layers, 2)
	assert.Len(t, layers[0], 1)
	assert.Len(t, layers[1], 3)
}

func TestLayers_T3_FanIn(t *testing.T) {
	d, _ := dag.New(
		node("a", nil, []string{"x"}),
		node("b", nil, []string{"y"}),
		node("c", nil, []string{"z"}),
		node("d", []string{"x", "y", "z"}, []string{"out"}),
	)
	layers := d.Layers()
	require.Len(t, layers, 2)
	assert.Len(t, layers[0], 3)
	assert.Contains(t, layers[1], "d")
}

func TestLayers_T4_Diamond(t *testing.T) {
	d, _ := dag.New(
		node("a", nil, []string{"x"}),
		node("b", []string{"x"}, []string{"y"}),
		node("c", []string{"x"}, []string{"z"}),
		node("d", []string{"y", "z"}, []string{"out"}),
	)
	layers := d.Layers()
	require.Len(t, layers, 3)
	assert.Contains(t, layers[0], "a")
	assert.ElementsMatch(t, []string{"b", "c"}, layers[1])
	assert.Contains(t, layers[2], "d")
}

func TestLayers_T6_WideParallel(t *testing.T) {
	d, _ := dag.New(
		node("a", nil, []string{"x"}),
		node("b", nil, []string{"y"}),
		node("c", nil, []string{"z"}),
	)
	layers := d.Layers()
	require.Len(t, layers, 1)
	assert.Len(t, layers[0], 3)
}

func TestLayers_T7_PipelineWithBypass(t *testing.T) {
	// a produces both "x" and "shortcut"
	// b consumes "x" → produces "y"
	// c consumes both "y" (from b) and "shortcut" (from a) → longest path a→b→c = 2 hops
	d, _ := dag.New(
		node("a", nil, []string{"x", "shortcut"}),
		node("b", []string{"x"}, []string{"y"}),
		node("c", []string{"y", "shortcut"}, []string{"z"}),
	)
	layers := d.Layers()
	require.Len(t, layers, 3)
	assert.Contains(t, layers[0], "a")
	assert.Contains(t, layers[1], "b")
	assert.Contains(t, layers[2], "c")
}

func TestLayers_T10_Hourglass(t *testing.T) {
	d, _ := dag.New(
		node("a", nil, []string{"x"}),
		node("b", nil, []string{"y"}),
		node("c", nil, []string{"z"}),
		node("d", []string{"x", "y", "z"}, []string{"m"}),
		node("e", []string{"m"}, []string{"p"}),
		node("f", []string{"m"}, []string{"q"}),
		node("g", []string{"m"}, []string{"r"}),
	)
	layers := d.Layers()
	require.Len(t, layers, 3)
	assert.Len(t, layers[0], 3)
	assert.Contains(t, layers[1], "d")
	assert.Len(t, layers[2], 3)
}

func TestLayers_Empty(t *testing.T) {
	d, _ := dag.New()
	assert.Empty(t, d.Layers())
}

func TestLayers_SingleNode(t *testing.T) {
	d, _ := dag.New(node("only", nil, []string{"out"}))
	layers := d.Layers()
	require.Len(t, layers, 1)
	assert.Equal(t, []string{"only"}, layers[0])
}
