package main

import (
	"testing"

	"github.com/mirandaguillaume/reify/internal/classifier"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAggChooseLabels(t *testing.T) {
	// Plural wins even when singular is also set.
	assert.Equal(t, []string{"context", "security"},
		chooseLabels([]string{"context", "security"}, "strategy"))

	// Empty plural + non-empty singular -> singleton slice.
	assert.Equal(t, []string{"strategy"}, chooseLabels(nil, "strategy"))
	assert.Equal(t, []string{"strategy"}, chooseLabels([]string{}, "strategy"))

	// Both empty -> nil.
	assert.Nil(t, chooseLabels(nil, ""))
	assert.Nil(t, chooseLabels([]string{}, ""))
}

func TestAggSplitValid(t *testing.T) {
	// Mix of valid + invalid, partitioned in order.
	valid, invalid := splitValid([]string{"context", "bogus", "security", "nope"})
	assert.Equal(t, []string{"context", "security"}, valid)
	assert.Equal(t, []string{"bogus", "nope"}, invalid)

	// All valid -> invalid is nil/empty.
	valid, invalid = splitValid([]string{"strategy", "guardrails", "observability"})
	assert.Equal(t, []string{"strategy", "guardrails", "observability"}, valid)
	assert.Empty(t, invalid)

	// All invalid -> valid is nil/empty.
	valid, invalid = splitValid([]string{"foo", "bar"})
	assert.Empty(t, valid)
	assert.Equal(t, []string{"foo", "bar"}, invalid)

	// Empty input -> both empty.
	valid, invalid = splitValid(nil)
	assert.Empty(t, valid)
	assert.Empty(t, invalid)
}

func TestAggAnyLabelSet(t *testing.T) {
	getGold := func(it calibrateItem) []string { return it.GoldLabels }

	// One item has gold -> true.
	items := []calibrateItem{
		{ID: "a"},
		{ID: "b", GoldLabels: []string{"context"}},
		{ID: "c"},
	}
	assert.True(t, anyLabelSet(items, getGold))

	// No item has gold -> false.
	none := []calibrateItem{
		{ID: "a"},
		{ID: "b", LLMLabels: []string{"context"}}, // populated, but not gold
	}
	assert.False(t, anyLabelSet(none, getGold))

	// Empty slice -> false.
	assert.False(t, anyLabelSet(nil, getGold))
}

func TestAggRankVocabulary(t *testing.T) {
	// {"a":1,"b":3,"c":3} -> b,c (count 3, tag asc), then a (count 1).
	ranked := rankVocabulary(map[string]int{"a": 1, "b": 3, "c": 3})
	require.Len(t, ranked, 3)
	assert.Equal(t, tagCount{tag: "b", count: 3}, ranked[0])
	assert.Equal(t, tagCount{tag: "c", count: 3}, ranked[1])
	assert.Equal(t, tagCount{tag: "a", count: 1}, ranked[2])

	// Empty vocab -> empty result.
	assert.Empty(t, rankVocabulary(map[string]int{}))
}

func TestAggComputeMultiPerFacetAndSetMetrics(t *testing.T) {
	// item1: pred {context}, gold {context}            -> exact match
	// item2: pred {security}, gold {context, security} -> partial
	// item3: pred {} (skipped), gold {strategy}
	golded := []calibrateItem{
		{ID: "i1", GoldLabels: []string{"context"}},
		{ID: "i2", GoldLabels: []string{"context", "security"}},
		{ID: "i3", GoldLabels: []string{"strategy"}},
	}
	predict := func(it calibrateItem) []string {
		switch it.ID {
		case "i1":
			return []string{"context"}
		case "i2":
			return []string{"security"}
		default:
			return nil // i3 has empty prediction -> skipped
		}
	}

	mc := computeMulti(golded, predict)
	require.NotNil(t, mc)

	// N excludes the item with an empty prediction.
	assert.Equal(t, 2, mc.N)

	// --- per-facet context: TP=1, FP=0, FN=1, TN=0, support=2 ---
	ctx := mc.PerFacet[classifier.FacetContext]
	assert.Equal(t, 1, ctx.TP)
	assert.Equal(t, 0, ctx.FP)
	assert.Equal(t, 1, ctx.FN)
	assert.Equal(t, 0, ctx.TN)
	assert.Equal(t, 2, ctx.Support)
	assert.InDelta(t, 1.0, ctx.Precision, 1e-9)
	assert.InDelta(t, 0.5, ctx.Recall, 1e-9)
	assert.InDelta(t, 2.0/3.0, ctx.F1, 1e-9)
	assert.InDelta(t, 0.0, ctx.Kappa, 1e-9)

	// --- per-facet security: TP=1, FP=0, FN=0, TN=1, support=1 ---
	sec := mc.PerFacet[classifier.FacetSecurity]
	assert.Equal(t, 1, sec.TP)
	assert.Equal(t, 0, sec.FP)
	assert.Equal(t, 0, sec.FN)
	assert.Equal(t, 1, sec.TN)
	assert.Equal(t, 1, sec.Support)
	assert.InDelta(t, 1.0, sec.Precision, 1e-9)
	assert.InDelta(t, 1.0, sec.Recall, 1e-9)
	assert.InDelta(t, 1.0, sec.F1, 1e-9)
	assert.InDelta(t, 1.0, sec.Kappa, 1e-9)

	// --- per-facet strategy: all TN, no support (item with strategy gold was skipped) ---
	strat := mc.PerFacet[classifier.FacetStrategy]
	assert.Equal(t, 0, strat.TP)
	assert.Equal(t, 0, strat.FP)
	assert.Equal(t, 0, strat.FN)
	assert.Equal(t, 2, strat.TN)
	assert.Equal(t, 0, strat.Support)

	// --- macro metrics over support>0 facets (context, security) ---
	// macroF1 = (2/3 + 1) / 2
	assert.InDelta(t, (2.0/3.0+1.0)/2.0, mc.MacroF1, 1e-9)
	// macroKappa = (0 + 1) / 2
	assert.InDelta(t, 0.5, mc.MacroKappa, 1e-9)

	// --- micro metrics from pooled tp/fp/fn: tp=2, fp=0, fn=1 ---
	// microP = 2/2 = 1, microR = 2/3, microF1 = 2*1*(2/3)/(1+2/3)
	microP := 1.0
	microR := 2.0 / 3.0
	expectedMicroF1 := 2 * microP * microR / (microP + microR)
	assert.InDelta(t, expectedMicroF1, mc.MicroF1, 1e-9)

	// --- set-level metrics ---
	assert.InDelta(t, 0.5, mc.ExactMatch, 1e-9)   // 1 of 2 exact
	assert.InDelta(t, 0.75, mc.MeanJaccard, 1e-9) // (1.0 + 0.5) / 2
	assert.InDelta(t, 0.1, mc.MeanHamming, 1e-9)  // (0/5 + 1/5) / 2
}

func TestAggComputeMultiAllEmptyPredictReturnsNil(t *testing.T) {
	golded := []calibrateItem{
		{ID: "i1", GoldLabels: []string{"context"}},
		{ID: "i2", GoldLabels: []string{"security"}},
	}
	predictEmpty := func(it calibrateItem) []string { return nil }
	assert.Nil(t, computeMulti(golded, predictEmpty))

	// No items at all -> nil too.
	assert.Nil(t, computeMulti(nil, func(it calibrateItem) []string { return []string{"context"} }))
}

func TestAggComputePairAgreement(t *testing.T) {
	// item1: a {context},          b {context}            -> exact, jaccard 1.0
	// item2: a {context,security}, b {security}           -> not exact, jaccard 0.5
	// item3: a {strategy},         b {} (empty)           -> skipped
	items := []calibrateItem{
		{ID: "i1", LLMLabels: []string{"context"}, JudgeLabels: []string{"context"}},
		{ID: "i2", LLMLabels: []string{"context", "security"}, JudgeLabels: []string{"security"}},
		{ID: "i3", LLMLabels: []string{"strategy"}},
	}
	a := func(it calibrateItem) []string { return it.LLMLabels }
	b := func(it calibrateItem) []string { return it.JudgeLabels }

	pa := computePairAgreement(items, a, b)
	require.NotNil(t, pa)
	assert.Equal(t, 2, pa.N) // i3 skipped (b empty)
	assert.InDelta(t, 0.5, pa.ExactMatch, 1e-9)
	assert.InDelta(t, 0.75, pa.MeanJaccard, 1e-9) // (1.0 + 0.5) / 2
}

func TestAggComputePairAgreementNilWhenNoOverlap(t *testing.T) {
	// Every item has at most one of the two getters populated -> n==0 -> nil.
	items := []calibrateItem{
		{ID: "i1", LLMLabels: []string{"context"}},
		{ID: "i2", JudgeLabels: []string{"security"}},
		{ID: "i3"},
	}
	a := func(it calibrateItem) []string { return it.LLMLabels }
	b := func(it calibrateItem) []string { return it.JudgeLabels }
	assert.Nil(t, computePairAgreement(items, a, b))

	// Empty input -> nil.
	assert.Nil(t, computePairAgreement(nil, a, b))
}

func TestAggComputeTaxonomyEmptyReturnsNil(t *testing.T) {
	assert.Nil(t, computeTaxonomy(nil))
	assert.Nil(t, computeTaxonomy([]calibrateItem{}))
}

func TestAggComputeTaxonomy(t *testing.T) {
	// item1: gold {context}            -> cardinality 1, context singleton
	// item2: gold {context, security}  -> cardinality 2, no singleton
	golded := []calibrateItem{
		{ID: "i1", GoldLabels: []string{"context"}},
		{ID: "i2", GoldLabels: []string{"context", "security"}},
	}
	tr := computeTaxonomy(golded)
	require.NotNil(t, tr)

	assert.Equal(t, 2, tr.N)

	// Cardinality histogram: one item with 1 facet, one with 2.
	assert.Equal(t, map[int]int{1: 1, 2: 1}, tr.Cardinality)

	// Singleton rate = singletonCount / facetCount.
	// context: appears in 2 items, singleton in 1 -> 0.5
	assert.InDelta(t, 0.5, tr.SingletonRate[classifier.FacetContext], 1e-9)
	// security: appears in 1 item, never alone -> 0
	assert.InDelta(t, 0.0, tr.SingletonRate[classifier.FacetSecurity], 1e-9)
	// strategy: never appears -> safeDiv(0,0) = 0
	assert.InDelta(t, 0.0, tr.SingletonRate[classifier.FacetStrategy], 1e-9)

	// Co-occurrence: diagonal = facetCount; off-diagonal = joint count.
	assert.Equal(t, 2, tr.Cooccurrence[classifier.FacetContext][classifier.FacetContext])
	assert.Equal(t, 1, tr.Cooccurrence[classifier.FacetSecurity][classifier.FacetSecurity])
	assert.Equal(t, 1, tr.Cooccurrence[classifier.FacetContext][classifier.FacetSecurity])
	assert.Equal(t, 1, tr.Cooccurrence[classifier.FacetSecurity][classifier.FacetContext])
	// A facet that never appears stays at 0 everywhere.
	assert.Equal(t, 0, tr.Cooccurrence[classifier.FacetStrategy][classifier.FacetStrategy])

	// PMI is 0 wherever any factor (joint or marginal) is 0.
	assert.InDelta(t, 0.0, tr.PMI[classifier.FacetStrategy][classifier.FacetContext], 1e-9)
}
