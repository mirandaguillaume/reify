package scanner

import (
	"math"
	"path/filepath"
	"sort"
	"strings"
)

// Stratum represents a top-level group of directories with its budget allocation.
type Stratum struct {
	Prefix      string   // top-level segment: "apps", "libs", "src", "."
	Dirs        []string // all dirs in this stratum
	SourceCount int      // total source files across all dirs
	Budget      int      // allocated entries (set by allocateBudget)
	IsTest      bool     // true if entire stratum is test dirs
}

// stratumKey returns the top-level segment of a directory path.
// "apps/big-app/pages" → "apps", "." → "."
func stratumKey(dir string) string {
	parts := strings.Split(dir, string(filepath.Separator))
	return parts[0]
}

// identifyStrata groups directories by their top-level segment and computes
// aggregate source counts. Sorted by SourceCount descending.
func identifyStrata(dirs []string, dirSourceCount map[string]int) []Stratum {
	byKey := map[string]*Stratum{}
	var order []string

	for _, d := range dirs {
		key := stratumKey(d)
		s, exists := byKey[key]
		if !exists {
			s = &Stratum{Prefix: key}
			byKey[key] = s
			order = append(order, key)
		}
		s.Dirs = append(s.Dirs, d)
		s.SourceCount += dirSourceCount[d]
	}

	// Determine if a stratum is entirely test dirs.
	for _, s := range byKey {
		allTest := true
		for _, d := range s.Dirs {
			if !isTestDir(d) {
				allTest = false
				break
			}
		}
		s.IsTest = allTest
	}

	// Sort by source count descending.
	strata := make([]Stratum, 0, len(order))
	for _, key := range order {
		strata = append(strata, *byKey[key])
	}
	sort.Slice(strata, func(i, j int) bool {
		if strata[i].SourceCount != strata[j].SourceCount {
			return strata[i].SourceCount > strata[j].SourceCount
		}
		return strata[i].Prefix < strata[j].Prefix
	})

	return strata
}

// allocateBudget distributes totalBudget across strata using Neyman proportional
// allocation. Source strata share (1-testRatio) of the budget proportionally to
// their SourceCount. Test strata share testRatio of the budget.
// Each stratum gets at minimum 1 entry.
func allocateBudget(strata []Stratum, totalBudget int, testRatio float64) {
	if len(strata) == 0 {
		return
	}

	// Separate source and test strata.
	var sourceStrata, testStrata []*Stratum
	for i := range strata {
		if strata[i].IsTest {
			testStrata = append(testStrata, &strata[i])
		} else {
			sourceStrata = append(sourceStrata, &strata[i])
		}
	}

	// Compute pool sizes.
	sourceBudget := int(math.Round(float64(totalBudget) * (1.0 - testRatio)))
	testBudget := totalBudget - sourceBudget

	// Allocate within each pool.
	allocatePool(sourceStrata, sourceBudget)
	allocatePool(testStrata, testBudget)

	// Write back from pointers to slice.
	for i := range strata {
		for _, sp := range sourceStrata {
			if sp.Prefix == strata[i].Prefix {
				strata[i].Budget = sp.Budget
			}
		}
		for _, tp := range testStrata {
			if tp.Prefix == strata[i].Prefix {
				strata[i].Budget = tp.Budget
			}
		}
	}
}

// allocatePool distributes budget proportionally to SourceCount within a pool.
func allocatePool(pool []*Stratum, budget int) {
	if len(pool) == 0 {
		return
	}

	// Total source count in pool.
	totalSource := 0
	for _, s := range pool {
		totalSource += s.SourceCount
	}

	if totalSource == 0 {
		// No source files — distribute evenly.
		each := budget / len(pool)
		if each < 1 {
			each = 1
		}
		for _, s := range pool {
			s.Budget = each
		}
		return
	}

	// Proportional allocation with minimum 1.
	allocated := 0
	for _, s := range pool {
		share := float64(s.SourceCount) / float64(totalSource) * float64(budget)
		s.Budget = int(math.Round(share))
		if s.Budget < 1 {
			s.Budget = 1
		}
		allocated += s.Budget
	}

	// Adjust rounding residual: add/remove from largest stratum.
	if allocated != budget && len(pool) > 0 {
		// Find largest by SourceCount.
		largest := pool[0]
		for _, s := range pool[1:] {
			if s.SourceCount > largest.SourceCount {
				largest = s
			}
		}
		largest.Budget += budget - allocated
		if largest.Budget < 1 {
			largest.Budget = 1
		}
	}
}
