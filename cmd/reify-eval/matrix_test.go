package main

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestRunCellStopsAndCounts(t *testing.T) {
	// Fake attempt fn: always violated, never error -> rate should be 1.0
	calls := 0
	attempt := func() (*AttemptResult, error) {
		calls++
		return &AttemptResult{Violated: true}, nil
	}
	cell := runCell(attempt, 0.20, 100)
	assert.Equal(t, cell.N, calls)
	assert.InDelta(t, 1.0, cell.ViolationRate, 1e-9)
	assert.Less(t, cell.N, 100, "a decisive 100% stream should stop before nMax")
}

func TestRunCellDiscardsAgentErrors(t *testing.T) {
	seq := []*AttemptResult{
		{AgentError: true},  // discarded
		{Violated: false},   // counted
		{Violated: false},   // counted
	}
	i := 0
	attempt := func() (*AttemptResult, error) { r := seq[i%len(seq)]; i++; return r, nil }
	cell := runCell(attempt, 0.49, 6) // loose width, small nMax to bound it
	assert.Greater(t, cell.N, 0)
	assert.LessOrEqual(t, cell.ViolationRate, 0.0001, "only non-error runs (all clean) count")
}

func TestRenderMatrixSortsByViolationRate(t *testing.T) {
	out := renderMatrix("haiku", "medium", "scope-restriction", []FormulationResult{
		{Name: "absent", Cell: Cell{N: 10, ViolationRate: 0.8}},
		{Name: "strong", Cell: Cell{N: 10, ViolationRate: 0.1}},
	})
	assert.Less(t, strings.Index(out, "strong"), strings.Index(out, "absent"))
	assert.Contains(t, out, "intention=scope-restriction")
}
