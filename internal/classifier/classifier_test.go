package classifier_test

import (
	"testing"

	"github.com/mirandaguillaume/reify/internal/classifier"
	"github.com/stretchr/testify/assert"
)

func TestClassify_BulletList(t *testing.T) {
	content := `## Commands
- Run tests with go test ./...
- Build with go build`

	r := classifier.Classify(content, "")
	assert.Len(t, r.Items, 2)
	assert.Equal(t, "Run tests with go test ./...", r.Items[0].Text)
	assert.Equal(t, classifier.FacetStrategy, r.Items[0].Facet)
	assert.Equal(t, "Commands", r.Items[0].Section)
}

func TestClassify_NumberedList(t *testing.T) {
	content := `## Steps
1. First step
2. Second step
10. Tenth step`
	r := classifier.Classify(content, "")
	assert.Len(t, r.Items, 3)
	assert.Equal(t, "First step", r.Items[0].Text)
	assert.Equal(t, "Tenth step", r.Items[2].Text)
}

func TestClassify_StarAndPlusBullets(t *testing.T) {
	content := `## Tools
* First
+ Second
- Third`
	r := classifier.Classify(content, "")
	assert.Len(t, r.Items, 3)
	assert.Equal(t, "First", r.Items[0].Text)
	assert.Equal(t, "Second", r.Items[1].Text)
	assert.Equal(t, "Third", r.Items[2].Text)
}

func TestClassify_SkipsCodeBlocks(t *testing.T) {
	content := "## Commands\n```\ngo test ./...\n```\n- A real instruction"
	r := classifier.Classify(content, "")
	assert.Len(t, r.Items, 1)
	assert.Equal(t, "A real instruction", r.Items[0].Text)
}

func TestClassify_SkipsHorizontalRules(t *testing.T) {
	content := `## Section
---
===
- A real instruction`
	r := classifier.Classify(content, "")
	assert.Len(t, r.Items, 1)
}

func TestClassify_InlineCodeIsInstruction(t *testing.T) {
	content := "## Commands\n`go test ./...`"
	r := classifier.Classify(content, "")
	assert.Len(t, r.Items, 1)
	assert.Equal(t, "`go test ./...`", r.Items[0].Text)
}

func TestClassify_SkipsProseLines(t *testing.T) {
	content := `## About
This is a description sentence.
- Actual bullet`
	r := classifier.Classify(content, "")
	assert.Len(t, r.Items, 1)
	assert.Equal(t, "Actual bullet", r.Items[0].Text)
}

func TestClassify_SectionFacetMapping(t *testing.T) {
	cases := []struct {
		heading string
		want    classifier.Facet
	}{
		{"Guardrails", classifier.FacetGuardrails},
		{"Rules", classifier.FacetGuardrails},
		{"Constraints", classifier.FacetGuardrails},
		{"Security", classifier.FacetSecurity},
		{"Permissions", classifier.FacetSecurity},
		{"Observability", classifier.FacetObservability},
		{"Metrics", classifier.FacetObservability},
		{"Logging", classifier.FacetObservability},
		{"Commands", classifier.FacetStrategy},
		{"Dev Workflow", classifier.FacetStrategy},
		{"Tools", classifier.FacetStrategy},
		{"Tech Stack", classifier.FacetContext},
		{"Architecture", classifier.FacetContext},
		{"Project", classifier.FacetContext},
		{"UnknownHeader", classifier.FacetContext}, // default
	}
	for _, tc := range cases {
		t.Run(tc.heading, func(t *testing.T) {
			content := "## " + tc.heading + "\n- one item"
			r := classifier.Classify(content, "")
			assert.Len(t, r.Items, 1)
			assert.Equal(t, tc.want, r.Items[0].Facet, "heading %q", tc.heading)
		})
	}
}

func TestClassify_LineSignalOverridesSection(t *testing.T) {
	// Bullet inside a "Commands" section but framed as prohibition → guardrails wins.
	content := `## Commands
- Never commit secrets to the repository`
	r := classifier.Classify(content, "")
	assert.Len(t, r.Items, 1)
	assert.Equal(t, classifier.FacetGuardrails, r.Items[0].Facet)
}

func TestClassify_SecuritySignalOverrides(t *testing.T) {
	content := `## Commands
- Set the API key via the environment`
	r := classifier.Classify(content, "")
	assert.Len(t, r.Items, 1)
	assert.Equal(t, classifier.FacetSecurity, r.Items[0].Facet)
}

func TestClassify_ObservabilitySignalOverrides(t *testing.T) {
	content := `## Commands
- Log every request with trace id`
	r := classifier.Classify(content, "")
	assert.Len(t, r.Items, 1)
	assert.Equal(t, classifier.FacetObservability, r.Items[0].Facet)
}

func TestClassify_EmptyContent(t *testing.T) {
	r := classifier.Classify("", "claude")
	assert.Empty(t, r.Items)
	assert.Equal(t, "claude", r.Format)
}

func TestResult_ByFacet_PreservesAllFacetsOrder(t *testing.T) {
	content := `## Security
- Use SSO
## Guardrails
- Never bypass auth
## Stack
- Go 1.22`
	r := classifier.Classify(content, "")
	grouped := r.ByFacet()

	// Every facet key must be present, even when empty.
	for _, f := range classifier.AllFacets {
		_, ok := grouped[f]
		assert.True(t, ok, "facet %q should be a key", f)
	}
	assert.Len(t, grouped[classifier.FacetSecurity], 1)
	assert.Len(t, grouped[classifier.FacetGuardrails], 1)
	assert.Len(t, grouped[classifier.FacetContext], 1)
	assert.Empty(t, grouped[classifier.FacetObservability])
	assert.Empty(t, grouped[classifier.FacetStrategy])
}

func TestClassify_TracksSectionAcrossItems(t *testing.T) {
	content := `## First Section
- item A
## Second Section
- item B`
	r := classifier.Classify(content, "")
	assert.Equal(t, "First Section", r.Items[0].Section)
	assert.Equal(t, "Second Section", r.Items[1].Section)
}

func TestClassify_FormatPreserved(t *testing.T) {
	r := classifier.Classify("- x", "copilot")
	assert.Equal(t, "copilot", r.Format)
}
