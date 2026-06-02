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

func TestRawJaccard(t *testing.T) {
	// Raw strings, no facet filtering, no normalisation.
	a := map[string]bool{"risk_flag": true, "scope": true}
	b := map[string]bool{"risk_flag": true}
	assert.InDelta(t, 0.5, rawJaccard(a, b), 1e-9) // 1 shared / 2 union

	// Surface-distinct near-synonyms do NOT match (the whole point of "raw").
	c := map[string]bool{"risk_flag": true}
	d := map[string]bool{"risk_flagging": true}
	assert.InDelta(t, 0.0, rawJaccard(c, d), 1e-9)

	// Both empty => 1.0
	assert.InDelta(t, 1.0, rawJaccard(map[string]bool{}, map[string]bool{}), 1e-9)

	// Identical => 1.0
	assert.InDelta(t, 1.0, rawJaccard(a, a), 1e-9)
}

func TestMeanPairwiseJaccardRaw(t *testing.T) {
	// Fewer than 2 runs => 1.0
	assert.InDelta(t, 1.0, meanPairwiseJaccardRaw([][]string{{"a"}}), 1e-9)

	// Identical runs => 1.0
	identical := [][]string{{"risk", "scope"}, {"risk", "scope"}, {"scope", "risk"}}
	assert.InDelta(t, 1.0, meanPairwiseJaccardRaw(identical), 1e-9)

	// Disjoint runs => 0
	disjoint := [][]string{{"a"}, {"b"}, {"c"}}
	assert.InDelta(t, 0.0, meanPairwiseJaccardRaw(disjoint), 1e-9)

	// Within-run duplicates are deduped before comparison.
	dup := [][]string{{"a", "a", "b"}, {"a", "b"}}
	assert.InDelta(t, 1.0, meanPairwiseJaccardRaw(dup), 1e-9)
}

func TestComputeOpenConsistency(t *testing.T) {
	items := []calibrateItem{{ID: "stable", Text: "x"}, {ID: "noisy", Text: "y"}}
	results := [][][]string{
		// item "stable": identical tags across 3 runs => J=1
		{{"risk_flag"}, {"risk_flag"}, {"risk_flag"}},
		// item "noisy": all-disjoint => J=0, counts as zero-overlap
		{{"a"}, {"b"}, {"c"}},
	}
	rep := computeOpenConsistency(items, results, 3)
	assert.Equal(t, 2, rep.Items)
	// mean of (1.0, 0.0) = 0.5
	assert.InDelta(t, 0.5, rep.MeanPairJaccard, 1e-9)
	// one of two items had zero overlap
	assert.InDelta(t, 0.5, rep.ZeroOverlapRate, 1e-9)
	// each run emitted exactly 1 tag
	assert.InDelta(t, 1.0, rep.MeanTagsPerRun, 1e-9)
}

func TestComputeOpenConsistencySkipsInsufficientRuns(t *testing.T) {
	items := []calibrateItem{{ID: "a", Text: "x"}, {ID: "b", Text: "y"}}
	results := [][][]string{
		{{"tag"}, nil, nil},       // only 1 successful run => skipped
		{{"tag"}, {"tag"}, nil},   // 2 successful => scored
	}
	rep := computeOpenConsistency(items, results, 3)
	assert.Equal(t, 1, rep.Items)
	assert.Len(t, rep.PerItem, 1)
	assert.Equal(t, "b", rep.PerItem[0].ID)
}
