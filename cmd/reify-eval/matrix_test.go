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
	// effect column: strong's net effect = absent(0.8) - strong(0.1) = +70.0pp
	assert.Contains(t, out, "effect(pp)")
	assert.Contains(t, out, "+70.0")
}

func TestBaseName(t *testing.T) {
	assert.Equal(t, "absent", baseName("absent"))
	assert.Equal(t, "absent", baseName("absent@head"))
	assert.Equal(t, "neutral", baseName("neutral@mid-50%"))
	assert.Equal(t, "strong", baseName("strong@tail"))
}

func TestRenderMatrixEffectWithLITMLabels(t *testing.T) {
	// LITM-suffixed labels must still find the absent baseline.
	out := renderMatrix("haiku", "medium", "no-new-deps", []FormulationResult{
		{Name: "absent@mid-50%", Cell: Cell{N: 6, ViolationRate: 1.0}},
		{Name: "neutral@mid-50%", Cell: Cell{N: 6, ViolationRate: 0.5}},
	})
	// effect for neutral = absent(1.0) - neutral(0.5) = +50.0pp, not n/a
	assert.Contains(t, out, "+50.0")
	assert.NotContains(t, out, "neutral@mid-50%        50.0%      6            n/a")
}
