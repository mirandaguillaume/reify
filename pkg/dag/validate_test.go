package dag_test

import (
	"testing"

	"github.com/mirandaguillaume/reify/pkg/dag"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// --- HasCycle tests ---

func TestHasCycle_NoCycle_Pipeline(t *testing.T) {
	d, _ := dag.New(
		node("a", nil, []string{"x"}),
		node("b", []string{"x"}, []string{"y"}),
		node("c", []string{"y"}, []string{"z"}),
	)
	assert.False(t, d.HasCycle())
}

func TestHasCycle_NoCycle_Diamond(t *testing.T) {
	d, _ := dag.New(
		node("a", nil, []string{"x"}),
		node("b", []string{"x"}, []string{"y"}),
		node("c", []string{"x"}, []string{"z"}),
		node("d", []string{"y", "z"}, []string{"out"}),
	)
	assert.False(t, d.HasCycle())
}

func TestHasCycle_SingleNode(t *testing.T) {
	d, _ := dag.New(node("a", nil, []string{"x"}))
	assert.False(t, d.HasCycle())
}

func TestHasCycle_EmptyGraph(t *testing.T) {
	d, _ := dag.New()
	assert.False(t, d.HasCycle())
}

// --- ValidateTopology tests ---

func TestValidateTopology_Pipeline(t *testing.T) {
	d, _ := dag.New(
		node("a", nil, []string{"x"}),
		node("b", []string{"x"}, []string{"y"}),
		node("c", []string{"y"}, []string{"z"}),
	)
	assert.NoError(t, d.ValidateTopology(dag.TopoPipeline))
}

func TestValidateTopology_Pipeline_Mismatch_FanOut(t *testing.T) {
	// A→[B,C] is NOT a pipeline
	d, _ := dag.New(
		node("a", nil, []string{"x"}),
		node("b", []string{"x"}, []string{"y"}),
		node("c", []string{"x"}, []string{"z"}),
	)
	err := d.ValidateTopology(dag.TopoPipeline)
	assert.Error(t, err)
}

func TestValidateTopology_FanOut(t *testing.T) {
	d, _ := dag.New(
		node("a", nil, []string{"x"}),
		node("b", []string{"x"}, []string{"y"}),
		node("c", []string{"x"}, []string{"z"}),
	)
	assert.NoError(t, d.ValidateTopology(dag.TopoFanOut))
}

func TestValidateTopology_FanIn(t *testing.T) {
	d, _ := dag.New(
		node("a", nil, []string{"x"}),
		node("b", nil, []string{"y"}),
		node("c", []string{"x", "y"}, []string{"z"}),
	)
	assert.NoError(t, d.ValidateTopology(dag.TopoFanIn))
}

func TestValidateTopology_Diamond(t *testing.T) {
	d, _ := dag.New(
		node("a", nil, []string{"x"}),
		node("b", []string{"x"}, []string{"y"}),
		node("c", []string{"x"}, []string{"z"}),
		node("d", []string{"y", "z"}, []string{"out"}),
	)
	assert.NoError(t, d.ValidateTopology(dag.TopoDiamond))
}

func TestValidateTopology_Diamond_TooFewNodes(t *testing.T) {
	d, _ := dag.New(
		node("a", nil, []string{"x"}),
		node("b", []string{"x"}, []string{"y"}),
		node("c", []string{"y"}, []string{"z"}),
	)
	err := d.ValidateTopology(dag.TopoDiamond)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "diamond")
}

func TestValidateTopology_WideParallel(t *testing.T) {
	d, _ := dag.New(
		node("a", nil, []string{"x"}),
		node("b", nil, []string{"y"}),
		node("c", nil, []string{"z"}),
	)
	assert.NoError(t, d.ValidateTopology(dag.TopoWideParallel))
}

func TestValidateTopology_WideParallel_HasEdge(t *testing.T) {
	d, _ := dag.New(
		node("a", nil, []string{"x"}),
		node("b", []string{"x"}, []string{"y"}),
	)
	err := d.ValidateTopology(dag.TopoWideParallel)
	assert.Error(t, err)
}

func TestValidateTopology_Hourglass(t *testing.T) {
	d, _ := dag.New(
		node("a", nil, []string{"x"}),
		node("b", nil, []string{"y"}),
		node("d", []string{"x", "y"}, []string{"m"}),
		node("e", []string{"m"}, []string{"p"}),
		node("f", []string{"m"}, []string{"q"}),
	)
	assert.NoError(t, d.ValidateTopology(dag.TopoHourglass))
}

func TestValidateTopology_Hourglass_TooFewLayers(t *testing.T) {
	// Only 2 layers — not an hourglass
	d, _ := dag.New(
		node("a", nil, []string{"x"}),
		node("b", nil, []string{"y"}),
		node("c", []string{"x", "y"}, []string{"z"}),
	)
	err := d.ValidateTopology(dag.TopoHourglass)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "hourglass")
}

func TestValidateTopology_MismatchError_ContainsName(t *testing.T) {
	// Pipeline declared as diamond — error must name the topology
	d, _ := dag.New(
		node("a", nil, []string{"x"}),
		node("b", []string{"x"}, []string{"y"}),
	)
	err := d.ValidateTopology(dag.TopoDiamond)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "diamond")
}

func TestFindCycles_NoCycles(t *testing.T) {
	d, err := dag.New(
		&dag.Node{ID: "a", Produces: []string{"x"}},
		&dag.Node{ID: "b", Consumes: []string{"x"}},
	)
	require.NoError(t, err)
	assert.Empty(t, d.FindCycles())
}

func TestFindCycles_WithBackEdge(t *testing.T) {
	d, err := dag.New(
		&dag.Node{ID: "a", Produces: []string{"x"}},
		&dag.Node{ID: "b", Consumes: []string{"x"}, Produces: []string{"y"}},
		&dag.Node{ID: "c", Consumes: []string{"y"}},
	)
	require.NoError(t, err)
	require.NoError(t, d.AddBackEdge("c", "a", 3))

	cycles := d.FindCycles()
	require.Len(t, cycles, 1)
	assert.Equal(t, 3, cycles[0].MaxIterations)
	assert.Equal(t, []string{"a", "b", "c"}, cycles[0].Path)
}

func TestValidateBoundedCycles_Valid(t *testing.T) {
	d, _ := dag.New(
		&dag.Node{ID: "a", Produces: []string{"x"}},
		&dag.Node{ID: "b", Consumes: []string{"x"}},
	)
	require.NoError(t, d.AddBackEdge("b", "a", 5))
	assert.NoError(t, d.ValidateBoundedCycles())
}

func TestValidateBoundedCycles_DanglingBackEdge(t *testing.T) {
	// Back-edge from c→a but no forward path from a→c
	d, _ := dag.New(
		&dag.Node{ID: "a"},
		&dag.Node{ID: "b"},
		&dag.Node{ID: "c"},
	)
	require.NoError(t, d.AddBackEdge("c", "a", 2))
	err := d.ValidateBoundedCycles()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "does not close a cycle")
}
