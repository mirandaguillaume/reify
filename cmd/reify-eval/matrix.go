package main

import (
	"fmt"
	"sort"
	"strings"
)

// Cell is the measured result for one (formulation) under one couple.
type Cell struct {
	N             int // counted (non-error) attempts
	Violations    int
	ViolationRate float64
}

// runCell repeats attempt() until the sequential stop fires, counting
// only non-error runs (AgentError attempts are technical waste, excluded
// per spec §3b). attempt() returns one AttemptResult per call.
func runCell(attempt func() (*AttemptResult, error), targetHalfWidth float64, nMax int) Cell {
	var c Cell
	for {
		res, err := attempt()
		if err != nil || res == nil || res.AgentError {
			// Technical waste: do not count, but guard against infinite
			// loops by still checking nMax against total tries.
			if c.N >= nMax {
				break
			}
			continue
		}
		c.N++
		if res.Violated {
			c.Violations++
		}
		if shouldStop(c.Violations, c.N, targetHalfWidth, nMax) {
			break
		}
	}
	if c.N > 0 {
		c.ViolationRate = float64(c.Violations) / float64(c.N)
	}
	return c
}

// FormulationResult pairs a formulation name with its measured cell.
type FormulationResult struct {
	Name string
	Cell Cell
}

// renderMatrix formats the formulation -> violation-rate table, sorted by
// violation rate ascending (most-obeyed first). `absent` is the control.
func renderMatrix(model, effort, intention string, results []FormulationResult) string {
	sort.Slice(results, func(i, j int) bool {
		return results[i].Cell.ViolationRate < results[j].Cell.ViolationRate
	})
	var b strings.Builder
	fmt.Fprintf(&b, "intention=%s  model=%s  effort=%s\n", intention, model, effort)
	fmt.Fprintf(&b, "%-12s %12s %6s\n", "formulation", "violation%", "n")
	for _, r := range results {
		fmt.Fprintf(&b, "%-12s %11.1f%% %6d\n", r.Name, 100*r.Cell.ViolationRate, r.Cell.N)
	}
	return b.String()
}
