package main

import (
	"testing"

	"github.com/mirandaguillaume/reify/internal/classifier"
	"github.com/stretchr/testify/assert"
)

const metricDelta = 1e-9

func TestMetricSafeDiv(t *testing.T) {
	assert.InDelta(t, 3.0, safeDiv(6, 2), metricDelta)
	assert.InDelta(t, 0.0, safeDiv(7, 0), metricDelta)
	assert.InDelta(t, 0.0, safeDiv(0, 5), metricDelta)
	assert.InDelta(t, 0.5, safeDiv(1, 2), metricDelta)
}

func TestMetricHarmonic(t *testing.T) {
	// p+r == 0 short-circuits to 0.
	assert.InDelta(t, 0.0, harmonic(0, 0), metricDelta)
	// Perfect precision and recall.
	assert.InDelta(t, 1.0, harmonic(1, 1), metricDelta)
	// 2*0.5*1/(0.5+1) = 1/1.5 = 0.6666...
	assert.InDelta(t, 2.0/3.0, harmonic(0.5, 1), metricDelta)
	// Asymmetry: one term zero yields zero numerator.
	assert.InDelta(t, 0.0, harmonic(1, 0), metricDelta)
	assert.InDelta(t, 0.0, harmonic(0, 1), metricDelta)
}

func TestMetricIfPositive(t *testing.T) {
	// count 0 forces 0 regardless of val.
	assert.InDelta(t, 0.0, ifPositive(0, 42.5), metricDelta)
	assert.InDelta(t, 0.0, ifPositive(0, -3.0), metricDelta)
	// count > 0 passes val through unchanged.
	assert.InDelta(t, 42.5, ifPositive(1, 42.5), metricDelta)
	assert.InDelta(t, -3.0, ifPositive(7, -3.0), metricDelta)
}

func TestMetricSetsEqual(t *testing.T) {
	a := map[classifier.Facet]bool{
		classifier.FacetContext:  true,
		classifier.FacetSecurity: true,
	}
	bEqual := map[classifier.Facet]bool{
		classifier.FacetContext:  true,
		classifier.FacetSecurity: true,
	}
	assert.True(t, setsEqual(a, bEqual))

	// Different size.
	bSmaller := map[classifier.Facet]bool{
		classifier.FacetContext: true,
	}
	assert.False(t, setsEqual(a, bSmaller))

	// Same size, different keys.
	bDiffKeys := map[classifier.Facet]bool{
		classifier.FacetContext:  true,
		classifier.FacetStrategy: true,
	}
	assert.False(t, setsEqual(a, bDiffKeys))

	// Two empty maps are equal.
	assert.True(t, setsEqual(map[classifier.Facet]bool{}, map[classifier.Facet]bool{}))
}

func TestMetricAsSet(t *testing.T) {
	// Unknown labels are dropped, valid ones retained.
	got := asSet([]string{"bogus", "security"})
	assert.Equal(t, map[classifier.Facet]bool{classifier.FacetSecurity: true}, got)

	// Duplicate valid label collapses to a single key.
	dup := asSet([]string{"context", "context"})
	assert.Equal(t, map[classifier.Facet]bool{classifier.FacetContext: true}, dup)
	assert.Len(t, dup, 1)

	// Empty input yields an empty (non-nil) map.
	empty := asSet(nil)
	assert.NotNil(t, empty)
	assert.Len(t, empty, 0)

	// All-unknown input yields an empty map.
	assert.Len(t, asSet([]string{"foo", "bar"}), 0)
}

func TestMetricJaccard(t *testing.T) {
	// Identical sets.
	assert.InDelta(t, 1.0, jaccard([]string{"context", "security"}, []string{"context", "security"}), metricDelta)
	// Disjoint sets: inter 0 / union 2 = 0.
	assert.InDelta(t, 0.0, jaccard([]string{"context"}, []string{"security"}), metricDelta)
	// Both empty (after dropping unknowns) -> 1.0.
	assert.InDelta(t, 1.0, jaccard(nil, nil), metricDelta)
	assert.InDelta(t, 1.0, jaccard([]string{"bogus"}, []string{"unknown"}), metricDelta)
	// Partial: {context,security} vs {security} -> inter 1 / union 2 = 0.5.
	assert.InDelta(t, 0.5, jaccard([]string{"context", "security"}, []string{"security"}), metricDelta)
}

func TestMetricJaccardSets(t *testing.T) {
	csSet := map[classifier.Facet]bool{
		classifier.FacetContext:  true,
		classifier.FacetSecurity: true,
	}
	// Identical sets.
	identical := map[classifier.Facet]bool{
		classifier.FacetContext:  true,
		classifier.FacetSecurity: true,
	}
	assert.InDelta(t, 1.0, jaccardSets(csSet, identical), metricDelta)
	// Disjoint.
	assert.InDelta(t, 0.0,
		jaccardSets(
			map[classifier.Facet]bool{classifier.FacetContext: true},
			map[classifier.Facet]bool{classifier.FacetSecurity: true}),
		metricDelta)
	// Both empty -> 1.0.
	assert.InDelta(t, 1.0, jaccardSets(map[classifier.Facet]bool{}, map[classifier.Facet]bool{}), metricDelta)
	// Partial: {context,security} vs {security} -> 0.5.
	assert.InDelta(t, 0.5,
		jaccardSets(csSet, map[classifier.Facet]bool{classifier.FacetSecurity: true}),
		metricDelta)
}

func TestMetricHammingLoss(t *testing.T) {
	// Identical -> no disagreements.
	assert.InDelta(t, 0.0, hammingLoss([]string{"context", "security"}, []string{"context", "security"}), metricDelta)
	// One facet differs -> 1/5 = 0.2.
	assert.InDelta(t, 0.2, hammingLoss([]string{"context"}, nil), metricDelta)
	// Total disjoint of 1+1 facets: context differs, security differs -> 2/5 = 0.4.
	assert.InDelta(t, 0.4, hammingLoss([]string{"context"}, []string{"security"}), metricDelta)
	// Unknown labels are dropped before comparison, so they cause no disagreement.
	assert.InDelta(t, 0.0, hammingLoss([]string{"bogus"}, nil), metricDelta)
}

func TestMetricHammingSets(t *testing.T) {
	csSet := map[classifier.Facet]bool{
		classifier.FacetContext:  true,
		classifier.FacetSecurity: true,
	}
	identical := map[classifier.Facet]bool{
		classifier.FacetContext:  true,
		classifier.FacetSecurity: true,
	}
	// Identical -> 0.
	assert.InDelta(t, 0.0, hammingSets(csSet, identical), metricDelta)
	// One facet differs -> 1/5 = 0.2.
	assert.InDelta(t, 0.2,
		hammingSets(
			map[classifier.Facet]bool{classifier.FacetContext: true},
			map[classifier.Facet]bool{}),
		metricDelta)
	// Total disjoint of 1+1 facets -> 2/5 = 0.4.
	assert.InDelta(t, 0.4,
		hammingSets(
			map[classifier.Facet]bool{classifier.FacetContext: true},
			map[classifier.Facet]bool{classifier.FacetSecurity: true}),
		metricDelta)
}

func TestMetricKappaForBinary(t *testing.T) {
	// n == 0 -> 0.
	assert.InDelta(t, 0.0, kappaForBinary(0, 0, 0, 0), metricDelta)

	// Perfect agreement: fp=fn=0, tp>0, tn>0.
	// n=4, po=1, pa=0.5, pb=0.5, pe=0.5 -> (1-0.5)/(1-0.5) = 1.0.
	assert.InDelta(t, 1.0, kappaForBinary(2, 0, 0, 2), metricDelta)

	// pe >= 1 saturation: all tn (no positives on either side).
	// pa=0, pb=0, pe = 0 + 1*1 = 1.0 -> returns 1.0.
	assert.InDelta(t, 1.0, kappaForBinary(0, 0, 0, 4), metricDelta)

	// Saturation also when all tp.
	// pa=1, pb=1, pe = 1 + 0 = 1.0 -> returns 1.0.
	assert.InDelta(t, 1.0, kappaForBinary(4, 0, 0, 0), metricDelta)

	// Worse than chance: tp=0, fp=2, fn=2, tn=0.
	// n=4, po=0, pa=0.5, pb=0.5, pe=0.5 -> (0-0.5)/(1-0.5) = -1.0.
	assert.InDelta(t, -1.0, kappaForBinary(0, 2, 2, 0), metricDelta)
}

func TestMetricKappaInterpretation(t *testing.T) {
	assert.Equal(t, "poor (worse than chance)", kappaInterpretation(-0.1))
	assert.Equal(t, "slight", kappaInterpretation(0.1))
	assert.Equal(t, "fair", kappaInterpretation(0.3))
	assert.Equal(t, "moderate", kappaInterpretation(0.5))
	assert.Equal(t, "substantial", kappaInterpretation(0.7))
	assert.Equal(t, "almost perfect", kappaInterpretation(0.9))
	// Exact lower bound 0 falls into "slight" (k < 0 is false).
	assert.Equal(t, "slight", kappaInterpretation(0.0))
}
