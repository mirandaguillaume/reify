package qualitygate

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ─── extractStructuralRequirements ────────────────────────────────────────────

func TestExtractStructuralRequirements_Empty(t *testing.T) {
	assert.Nil(t, extractStructuralRequirements(""))
}

func TestExtractStructuralRequirements_NoHeadings_ReturnsNil(t *testing.T) {
	// Flat template — no ## or ### headings → nil → heuristic fallback
	tmpl := "- item one\n- **bold**: value\nSome prose here."
	assert.Nil(t, extractStructuralRequirements(tmpl))
}

func TestExtractStructuralRequirements_TwoHeadingsBothSatisfied(t *testing.T) {
	tmpl := "## Verdict\n\nApproved.\n\n## Issues\n\nNone found."
	reqs := extractStructuralRequirements(tmpl)
	require.Len(t, reqs, 2)
	assert.Equal(t, "## verdict", reqs[0].heading)
	assert.True(t, reqs[0].needsBody)
	assert.False(t, reqs[0].needsList)
	assert.Equal(t, "## issues", reqs[1].heading)
	assert.True(t, reqs[1].needsBody)
}

func TestExtractStructuralRequirements_HeadingWithList(t *testing.T) {
	tmpl := "## Issues\n\n- bug one\n- bug two\n"
	reqs := extractStructuralRequirements(tmpl)
	require.Len(t, reqs, 1)
	assert.Equal(t, "## issues", reqs[0].heading)
	assert.True(t, reqs[0].needsList)
}

func TestExtractStructuralRequirements_HeadingOnlyNoContent(t *testing.T) {
	// Heading with no sibling content → needsBody=false, needsList=false
	tmpl := "## Verdict\n## Summary\n"
	reqs := extractStructuralRequirements(tmpl)
	require.Len(t, reqs, 2)
	assert.False(t, reqs[0].needsBody)
	assert.False(t, reqs[0].needsList)
}

func TestExtractStructuralRequirements_H3Heading(t *testing.T) {
	tmpl := "### Details\n\nSome detail text.\n"
	reqs := extractStructuralRequirements(tmpl)
	require.Len(t, reqs, 1)
	assert.Equal(t, "### details", reqs[0].heading)
	assert.True(t, reqs[0].needsBody)
}

func TestExtractStructuralRequirements_H1Ignored(t *testing.T) {
	// H1 headings are not tracked — only H2/H3
	tmpl := "# Top-level title\n\n## Subsection\n\nContent here."
	reqs := extractStructuralRequirements(tmpl)
	require.Len(t, reqs, 1)
	assert.Equal(t, "## subsection", reqs[0].heading)
}

func TestExtractStructuralRequirements_CasePreservedLowercased(t *testing.T) {
	reqs := extractStructuralRequirements("## Verdict\n\nBody.")
	require.Len(t, reqs, 1)
	assert.Equal(t, "## verdict", reqs[0].heading)
}

// ─── validateASTStructure ─────────────────────────────────────────────────────

func TestValidateASTStructure_AllSatisfied(t *testing.T) {
	tmpl := "## Verdict\n\nDecision text.\n\n## Issues\n\n- item one\n"
	reqs := extractStructuralRequirements(tmpl)

	output := "## Verdict\n\nApproved.\n\n## Issues\n\n- nothing significant\n"
	src := []byte(output)
	outputDoc := parseMarkdown(src)

	err := validateASTStructure(reqs, outputDoc, src)
	assert.NoError(t, err)
}

func TestValidateASTStructure_HeadingAbsent(t *testing.T) {
	tmpl := "## Verdict\n\nBody.\n"
	reqs := extractStructuralRequirements(tmpl)

	output := "Some unstructured text."
	src := []byte(output)
	outputDoc := parseMarkdown(src)

	err := validateASTStructure(reqs, outputDoc, src)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "quality gate: heading")
	assert.Contains(t, err.Error(), "## verdict")
	assert.Contains(t, err.Error(), "not found")
}

func TestValidateASTStructure_SectionEmpty(t *testing.T) {
	// Template: ## Verdict with body content → needsBody=true
	// Output: ## Verdict immediately followed by another heading (no body)
	tmpl := "## Verdict\n\nDecision text.\n"
	reqs := extractStructuralRequirements(tmpl)

	output := "## Verdict\n## Summary\n"
	src := []byte(output)
	outputDoc := parseMarkdown(src)

	err := validateASTStructure(reqs, outputDoc, src)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "quality gate: section")
	assert.Contains(t, err.Error(), "## verdict")
	assert.Contains(t, err.Error(), "is empty")
}

func TestValidateASTStructure_SectionMissingList(t *testing.T) {
	// Template: ## Issues with list → needsList=true
	// Output: ## Issues with paragraph but no list
	tmpl := "## Issues\n\n- bug one\n"
	reqs := extractStructuralRequirements(tmpl)

	output := "## Issues\n\nNo issues found in this run.\n"
	src := []byte(output)
	outputDoc := parseMarkdown(src)

	err := validateASTStructure(reqs, outputDoc, src)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "quality gate: section")
	assert.Contains(t, err.Error(), "## issues")
	assert.Contains(t, err.Error(), "requires a list")
}

func TestValidateASTStructure_CaseInsensitiveHeadingMatch(t *testing.T) {
	// Template: ## Verdict → req.heading = "## verdict"
	// Output: ## VERDICT (different case) → should still match
	tmpl := "## Verdict\n\nBody.\n"
	reqs := extractStructuralRequirements(tmpl)

	output := "## VERDICT\n\nApproved.\n"
	src := []byte(output)
	outputDoc := parseMarkdown(src)

	err := validateASTStructure(reqs, outputDoc, src)
	assert.NoError(t, err)
}

func TestValidateASTStructure_NoRequirements_NoError(t *testing.T) {
	err := validateASTStructure(nil, parseMarkdown([]byte("anything")), []byte("anything"))
	assert.NoError(t, err)
}

// ─── Review patch: P2 headingText inline markup ───────────────────────────────

// Bold heading in output matches plain heading in template (Walk-based extraction).
func TestExtractOutputSection_BoldHeadingInOutput_Matches(t *testing.T) {
	tmpl := "## Verdict\n\nBody.\n"
	reqs := extractStructuralRequirements(tmpl)
	require.Len(t, reqs, 1)

	output := "## **Verdict**\n\nApproved.\n"
	src := []byte(output)
	err := validateASTStructure(reqs, parseMarkdown(src), src)
	assert.NoError(t, err, "bold heading in output must match plain heading requirement")
}

// Bold heading in template generates the correct requirement text.
func TestExtractStructuralRequirements_BoldHeadingInTemplate(t *testing.T) {
	tmpl := "## **Verdict**\n\nDecision.\n"
	reqs := extractStructuralRequirements(tmpl)
	require.Len(t, reqs, 1)
	assert.Equal(t, "## verdict", reqs[0].heading)
	assert.True(t, reqs[0].needsBody)
}

// ─── Review patch: P3 paragraphNonEmpty inline markup ────────────────────────

// A section whose template body is bold-only satisfies needsBody in the output.
func TestValidateASTStructure_BoldOnlyBody_SatisfiesNeedsBody(t *testing.T) {
	// Template: plain paragraph → needsBody=true.
	tmpl := "## Verdict\n\nDecision.\n"
	reqs := extractStructuralRequirements(tmpl)
	require.Len(t, reqs, 1)
	assert.True(t, reqs[0].needsBody)

	// Output: bold-only paragraph — must satisfy the body requirement.
	output := "## Verdict\n\n**Approved.**\n"
	src := []byte(output)
	err := validateASTStructure(reqs, parseMarkdown(src), src)
	assert.NoError(t, err, "bold-only paragraph must satisfy needsBody (Walk-based paragraphNonEmpty)")
}

// ─── Review patch: P4 whitespace-only template guard ─────────────────────────

func TestExtractStructuralRequirements_WhitespaceOnly_ReturnsNil(t *testing.T) {
	assert.Nil(t, extractStructuralRequirements("   \n\t\n"))
	assert.Nil(t, extractStructuralRequirements("  "))
}

