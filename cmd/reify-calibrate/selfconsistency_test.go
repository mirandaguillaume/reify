package main

import (
	"testing"

	"github.com/mirandaguillaume/reify/internal/classifier"
	"github.com/stretchr/testify/assert"
)

func TestCanonicalFacets(t *testing.T) {
	// Valid facets returned in AllFacets order, deduped, invalid dropped.
	got := canonicalFacets([]string{"security", "context", "context", "bogus"})
	assert.Equal(t, []string{"context", "security"}, got)

	assert.Nil(t, canonicalFacets(nil))
	assert.Nil(t, canonicalFacets([]string{"nonsense"}))
}

func TestLabelsetKey(t *testing.T) {
	// Same members, different order/dupes => same key.
	a := labelsetKey([]string{"security", "context"})
	b := labelsetKey([]string{"context", "security", "context"})
	assert.Equal(t, a, b)
	assert.Equal(t, "context,security", a)

	// Different members => different key.
	assert.NotEqual(t, a, labelsetKey([]string{"context"}))
}

func TestModalLabelset(t *testing.T) {
	// Plurality wins: "context" appears 3x, "context security" 2x.
	runs := [][]string{
		{"context"},
		{"context"},
		{"security", "context"},
		{"context", "security"},
		{"context"},
	}
	got := modalLabelset(runs)
	assert.Equal(t, []string{"context"}, got)
}

func TestModalLabelsetDeterministicTieBreak(t *testing.T) {
	// 2 vs 2 tie between {context} and {security}. Tie-break is by the
	// canonical key string ascending: "context" < "security".
	runs := [][]string{
		{"security"},
		{"context"},
		{"security"},
		{"context"},
	}
	got := modalLabelset(runs)
	assert.Equal(t, []string{"context"}, got)
}

func TestMeanPairwiseJaccard(t *testing.T) {
	// All identical => 1.0
	identical := [][]string{{"context"}, {"context"}, {"context"}}
	assert.InDelta(t, 1.0, meanPairwiseJaccard(identical), 1e-9)

	// Fewer than 2 runs => 1.0 (nothing to disagree on)
	assert.InDelta(t, 1.0, meanPairwiseJaccard([][]string{{"context"}}), 1e-9)

	// Two runs: {context,security} vs {context} => |∩|/|∪| = 1/2
	half := [][]string{{"context", "security"}, {"context"}}
	assert.InDelta(t, 0.5, meanPairwiseJaccard(half), 1e-9)

	// Three runs, pairs: (AB)=1/2, (AC)=0, (BC)=0 => mean = (0.5+0+0)/3
	mixed := [][]string{
		{"context", "security"},
		{"context"},
		{"strategy"},
	}
	assert.InDelta(t, 0.5/3.0, meanPairwiseJaccard(mixed), 1e-9)
}

func TestComputeSelfConsistencyPerfectStable(t *testing.T) {
	items := []calibrateItem{{ID: "a", Text: "x"}, {ID: "b", Text: "y"}}
	// Both items identical across 3 runs => everything perfectly stable.
	results := [][][]string{
		{{"context"}, {"context"}, {"context"}},
		{{"security"}, {"security"}, {"security"}},
	}
	rep := computeSelfConsistency(items, results, 3)
	assert.Equal(t, 2, rep.Items)
	assert.InDelta(t, 1.0, rep.PerfectStable, 1e-9)
	assert.InDelta(t, 1.0, rep.MeanModalAgree, 1e-9)
	assert.InDelta(t, 1.0, rep.MeanPairJaccard, 1e-9)
	for _, f := range classifier.AllFacets {
		assert.InDelta(t, 0.0, rep.FacetFlipRate[f], 1e-9, "facet %s should never flip", f)
	}
}

func TestComputeSelfConsistencyUnstable(t *testing.T) {
	items := []calibrateItem{{ID: "a", Text: "x"}}
	// Item flips between {context} (2x) and {context,security} (1x).
	// modal = {context}. modal agreement = 2/3. Not perfect.
	results := [][][]string{
		{{"context"}, {"context", "security"}, {"context"}},
	}
	rep := computeSelfConsistency(items, results, 3)
	assert.Equal(t, 1, rep.Items)
	assert.InDelta(t, 0.0, rep.PerfectStable, 1e-9)
	assert.InDelta(t, 2.0/3.0, rep.MeanModalAgree, 1e-9)
	// security flips in 1 of 3 runs vs modal; context never flips.
	assert.InDelta(t, 1.0/3.0, rep.FacetFlipRate[classifier.FacetSecurity], 1e-9)
	assert.InDelta(t, 0.0, rep.FacetFlipRate[classifier.FacetContext], 1e-9)
}

func TestComputeSelfConsistencySkipsInsufficientRuns(t *testing.T) {
	items := []calibrateItem{{ID: "a", Text: "x"}, {ID: "b", Text: "y"}}
	// Item a has only 1 successful run (rest nil) => skipped.
	// Item b has 2 successful runs => scored.
	results := [][][]string{
		{{"context"}, nil, nil},
		{{"security"}, {"security"}, nil},
	}
	rep := computeSelfConsistency(items, results, 3)
	assert.Equal(t, 1, rep.Items, "only item b has >=2 successful runs")
	assert.Len(t, rep.PerItem, 1)
	assert.Equal(t, "b", rep.PerItem[0].ID)
}

func TestTruncate(t *testing.T) {
	assert.Equal(t, "abc", truncate("abc", 5))
	assert.Equal(t, "abcde…", truncate("abcdefgh", 5))
}
