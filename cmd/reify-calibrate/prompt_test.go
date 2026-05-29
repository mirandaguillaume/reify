package main

import (
	"testing"

	"github.com/mirandaguillaume/reify/internal/classifier"
	"github.com/stretchr/testify/assert"
)

// TestPromptEmergentHeaderHasNoContaminatingExamples is the keystone
// contamination-regression test. Per docs/calibration/findings.md (Run 2),
// an earlier open-coding prompt seeded positive intent-axis EXAMPLE tags
// ("domain_fact", "soft_recommendation", "hard_requirement",
// "fallback_directive"). The model dutifully echoed "domain_fact" back as
// its top tag 9 times — prompt contamination masquerading as an emergent
// finding. The open-coding prompt MUST therefore never contain positive
// intent-axis example vocabulary. This test fails the moment anyone
// re-introduces it.
func TestPromptEmergentHeaderHasNoContaminatingExamples(t *testing.T) {
	header := emergentTagPromptHeader()

	forbidden := []string{
		"domain_fact",
		"soft_recommendation",
		"hard_requirement",
		"fallback_directive",
	}
	for _, tok := range forbidden {
		assert.NotContains(t, header, tok,
			"open-coding prompt must not seed positive intent-axis example tag %q: "+
				"per findings.md Run 2 the model echoed such seeded tags (notably "+
				"\"domain_fact\" 9x) back as emergent labels — contamination, not signal",
			tok)
	}
}

// TestPromptEmergentHeaderKeepsAntiExamplesAndContrast asserts the prompt
// keeps the topical ANTI-examples (which anchor the WRONG axis on purpose),
// the same-topic/different-intent contrast pair, and the explicit
// instruction not to recycle prompt vocabulary.
func TestPromptEmergentHeaderKeepsAntiExamplesAndContrast(t *testing.T) {
	header := emergentTagPromptHeader()

	// Topical anti-examples that anchor the wrong (topic) axis.
	antiExamples := []string{
		"sql_injection_prevention",
		"null_safety",
		"logging_format",
	}
	for _, ex := range antiExamples {
		assert.Contains(t, header, ex,
			"prompt must retain topical anti-example %q to anchor the wrong axis", ex)
	}

	// Same-topic/different-intent contrast pair.
	assert.Contains(t, header, "parameterized queries",
		"prompt must keep the imperative half of the same-topic contrast")
	assert.Contains(t, header, "PostgreSQL 16",
		"prompt must keep the informative half of the same-topic contrast")

	// Explicit instruction not to recycle the prompt's own vocabulary.
	assert.Contains(t, header, "anti-examples",
		"prompt must label its examples as anti-examples on the wrong axis")
	assert.Contains(t, header, "Do NOT recycle",
		"prompt must explicitly forbid recycling its own example vocabulary")
}

// TestPromptEmergentHeaderDemandsSnakeCaseIntent asserts the prompt states
// the output contract: snake_case labels on the intent axis, 1 to 3 per item.
func TestPromptEmergentHeaderDemandsSnakeCaseIntent(t *testing.T) {
	header := emergentTagPromptHeader()

	assert.Contains(t, header, "snake_case",
		"prompt must require snake_case labels")
	assert.Contains(t, header, "intent",
		"prompt must frame the task on the intent axis")
	assert.Contains(t, header, "1 to 3 labels",
		"prompt must constrain the answer to 1 to 3 labels per item")
}

// TestPromptClusterRendersRankedVocabulary asserts the Round 2 clustering
// prompt renders each tagFreq as "- <tag> (x<count>)", emits the required
// JSON-shape instruction (clusters/outliers keys), and carries the
// intent-not-topic guidance.
func TestPromptClusterRendersRankedVocabulary(t *testing.T) {
	ranked := []tagFreq{
		{Tag: "risk_flagging", Count: 5},
		{Tag: "domain_scoping", Count: 2},
	}
	prompt := buildClusterPrompt(ranked)

	assert.Contains(t, prompt, "- risk_flagging (x5)",
		"each ranked tag must render as '- <tag> (x<count>)'")
	assert.Contains(t, prompt, "- domain_scoping (x2)",
		"each ranked tag must render as '- <tag> (x<count>)'")

	// Required JSON output shape.
	assert.Contains(t, prompt, "clusters",
		"clustering prompt must request a 'clusters' key")
	assert.Contains(t, prompt, "outliers",
		"clustering prompt must request an 'outliers' key")

	// Intent-not-topic guidance carried into Round 2 as well.
	assert.Contains(t, prompt, "not the topic",
		"clustering prompt must steer clusters onto the intent axis, not topic")
}

// TestPromptClusterEmptyVocabulary asserts buildClusterPrompt(nil) still
// returns the instruction scaffold (with the 'clusters' key) and does not
// panic when there are no ranked tags to render.
func TestPromptClusterEmptyVocabulary(t *testing.T) {
	assert.NotPanics(t, func() {
		prompt := buildClusterPrompt(nil)
		assert.Contains(t, prompt, "clusters",
			"empty input must still emit the JSON-shape scaffold with 'clusters'")
	})
}

// TestPromptJudgeHeaderListsFiveFacetsClosedVocabulary asserts the judge
// prompt enumerates all five canonical facets and enforces the closed
// vocabulary (no "other"/"general" escape hatch).
func TestPromptJudgeHeaderListsFiveFacetsClosedVocabulary(t *testing.T) {
	header := judgePromptHeader()

	// Guard against drift between the rubric and classifier.AllFacets.
	assert.Len(t, classifier.AllFacets, 5,
		"this test assumes the canonical 5-facet taxonomy")
	for _, facet := range classifier.AllFacets {
		assert.Contains(t, header, facet,
			"judge prompt must list canonical facet %q", facet)
	}

	// Closed-vocabulary guard: no escape-hatch label allowed.
	assert.Contains(t, header, "There is no",
		"judge prompt must explicitly close the vocabulary")
	assert.Contains(t, header, `no "other"`,
		"judge prompt must forbid an 'other' escape-hatch facet")
}
