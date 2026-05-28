package validate

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// --- Check 1: Frontmatter validity ---

func TestValidate_ValidFrontmatter(t *testing.T) {
	orig := []byte("---\nname: test\ntools: Read\n---\n\n## Rules\n\n- Rule 1\n")
	rewritten := []byte("---\nname: test\ntools: Read\nguidelines: added\n---\n\n## Rules\n\n- Rule 1 (improved)\n")

	result := Validate(orig, rewritten, GateOptions{Force: true})
	assert.True(t, result.Passed)
	assert.Empty(t, result.Failures)
}

func TestValidate_BrokenFrontmatter(t *testing.T) {
	orig := []byte("---\nname: test\n---\nBody")
	rewritten := []byte("---\n: {invalid yaml [[\n---\nBody")

	result := Validate(orig, rewritten, GateOptions{})
	assert.False(t, result.Passed)
	hasCheck := false
	for _, f := range result.Failures {
		if f.Check == "frontmatter_valid" {
			hasCheck = true
		}
	}
	assert.True(t, hasCheck, "should have frontmatter_valid failure")
}

func TestValidate_NoFrontmatter(t *testing.T) {
	orig := []byte("Just markdown text")
	rewritten := []byte("Just markdown text with changes")

	result := Validate(orig, rewritten, GateOptions{Force: true})
	assert.True(t, result.Passed)
}

// --- Check 2: Field preservation ---

func TestValidate_LostField(t *testing.T) {
	orig := []byte("---\nname: test\ntools: Read\nmodel: sonnet\n---\nBody")
	rewritten := []byte("---\nname: test\nmodel: sonnet\n---\nBody") // lost 'tools'

	result := Validate(orig, rewritten, GateOptions{})
	assert.False(t, result.Passed)
	hasCheck := false
	for _, f := range result.Failures {
		if f.Check == "field_preservation" {
			hasCheck = true
			assert.Contains(t, f.Detail, "tools")
		}
	}
	assert.True(t, hasCheck)
}

func TestValidate_AllFieldsPreserved(t *testing.T) {
	orig := []byte("---\nname: test\ntools: Read\n---\nBody")
	rewritten := []byte("---\nname: test\ntools: Read\nextra: new\n---\nBody with more content")

	result := Validate(orig, rewritten, GateOptions{})
	// Adding fields is fine, only losing them is a failure
	fieldFails := 0
	for _, f := range result.Failures {
		if f.Check == "field_preservation" {
			fieldFails++
		}
	}
	assert.Equal(t, 0, fieldFails)
}

func TestValidate_LostAllFrontmatter(t *testing.T) {
	orig := []byte("---\nname: test\ntools: Read\n---\nBody")
	rewritten := []byte("Body without frontmatter")

	result := Validate(orig, rewritten, GateOptions{})
	assert.False(t, result.Passed)
}

// --- Check 3: Markdown structure ---

func TestValidate_UnbalancedCodeFences(t *testing.T) {
	orig := []byte("---\nname: test\n---\n\n## Code\n\n```go\nfmt.Println()\n```\n")
	rewritten := []byte("---\nname: test\n---\n\n## Code\n\n```go\nfmt.Println()\n") // missing closing fence

	result := Validate(orig, rewritten, GateOptions{})
	assert.False(t, result.Passed)
	hasCheck := false
	for _, f := range result.Failures {
		if f.Check == "markdown_structure" && assert.Contains(t, f.Detail, "code fences") {
			hasCheck = true
		}
	}
	assert.True(t, hasCheck)
}

func TestValidate_MalformedHeader(t *testing.T) {
	orig := []byte("## Rules\n\nContent\n")
	rewritten := []byte("##Rules\n\nContent\n") // missing space after ##

	result := Validate(orig, rewritten, GateOptions{})
	assert.False(t, result.Passed)
	hasCheck := false
	for _, f := range result.Failures {
		if f.Check == "markdown_structure" && assert.Contains(t, f.Detail, "missing space") {
			hasCheck = true
		}
	}
	assert.True(t, hasCheck)
}

func TestValidate_HeaderInCodeBlock(t *testing.T) {
	// Headers inside code blocks should NOT be flagged
	orig := []byte("## Rules\n\n```\n##notaheader\n```\n")
	rewritten := []byte("## Rules\n\n```\n##notaheader\n```\n")

	result := Validate(orig, rewritten, GateOptions{})
	structureFails := 0
	for _, f := range result.Failures {
		if f.Check == "markdown_structure" {
			structureFails++
		}
	}
	assert.Equal(t, 0, structureFails)
}

// --- Check 4: Section preservation ---

func TestValidate_LostSection(t *testing.T) {
	orig := []byte("---\nname: test\n---\n\n## Rules\n\nContent\n\n## Process\n\nContent\n")
	rewritten := []byte("---\nname: test\n---\n\n## Rules\n\nContent\n") // lost Process

	result := Validate(orig, rewritten, GateOptions{})
	assert.False(t, result.Passed)
	hasCheck := false
	for _, f := range result.Failures {
		if f.Check == "section_preservation" {
			hasCheck = true
			assert.Contains(t, f.Detail, "Process")
		}
	}
	assert.True(t, hasCheck)
}

func TestValidate_AllSectionsPreserved(t *testing.T) {
	orig := []byte("## Rules\n\nContent\n\n## Process\n\nContent\n")
	rewritten := []byte("## Rules\n\nContent (improved)\n\n## Process\n\nContent\n\n## New Section\n\nNew\n")

	result := Validate(orig, rewritten, GateOptions{})
	sectionFails := 0
	for _, f := range result.Failures {
		if f.Check == "section_preservation" {
			sectionFails++
		}
	}
	assert.Equal(t, 0, sectionFails)
}

// --- Check 5: Diff size ---

func TestValidate_SmallDiff(t *testing.T) {
	orig := []byte("line1\nline2\nline3\nline4\nline5\nline6\nline7\nline8\nline9\nline10\n")
	rewritten := []byte("line1\nline2 changed\nline3\nline4\nline5\nline6\nline7\nline8\nline9\nline10\n")

	result := Validate(orig, rewritten, GateOptions{})
	diffFails := 0
	for _, f := range result.Failures {
		if f.Check == "diff_size" {
			diffFails++
		}
	}
	assert.Equal(t, 0, diffFails)
}

func TestValidate_LargeDiff(t *testing.T) {
	orig := []byte("line1\nline2\nline3\nline4\n")
	rewritten := []byte("completely\ndifferent\ncontent\nhere\nand more\n")

	result := Validate(orig, rewritten, GateOptions{})
	hasCheck := false
	for _, f := range result.Failures {
		if f.Check == "diff_size" {
			hasCheck = true
		}
	}
	assert.True(t, hasCheck, "should flag large diff")
}

func TestValidate_LargeDiff_Force(t *testing.T) {
	orig := []byte("line1\nline2\nline3\nline4\n")
	rewritten := []byte("completely\ndifferent\ncontent\nhere\nand more\n")

	result := Validate(orig, rewritten, GateOptions{Force: true})
	diffFails := 0
	for _, f := range result.Failures {
		if f.Check == "diff_size" {
			diffFails++
		}
	}
	assert.Equal(t, 0, diffFails, "force should skip diff_size check")
}

// --- DiffRatio ---

func TestDiffRatio_Identical(t *testing.T) {
	content := []byte("line1\nline2\nline3\n")
	assert.Equal(t, 0.0, DiffRatio(content, content))
}

func TestDiffRatio_CompletelyDifferent(t *testing.T) {
	orig := []byte("a\nb\nc")
	rewritten := []byte("x\ny\nz")
	assert.Equal(t, 1.0, DiffRatio(orig, rewritten))
}

func TestDiffRatio_PartialChange(t *testing.T) {
	orig := []byte("line1\nline2\nline3\nline4\n")
	rewritten := []byte("line1\nchanged\nline3\nline4\n")
	ratio := DiffRatio(orig, rewritten)
	assert.InDelta(t, 0.2, ratio, 0.01) // 1/5 lines changed (including trailing empty)
}

func TestDiffRatio_Empty(t *testing.T) {
	assert.Equal(t, 0.0, DiffRatio([]byte(""), []byte("")))
}

func TestDiffRatio_CRLF(t *testing.T) {
	orig := []byte("line1\r\nline2\r\n")
	rewritten := []byte("line1\nline2\n")
	// After normalization these should be identical
	assert.Equal(t, 0.0, DiffRatio(orig, rewritten))
}

// --- Multiple failures collected ---

func TestValidate_MultipleFailures(t *testing.T) {
	orig := []byte("---\nname: test\ntools: Read\n---\n\n## Rules\n\nContent\n\n## Process\n\nContent\n")
	// Lost 'tools', lost 'Process' section
	rewritten := []byte("---\nname: test\n---\n\n## Rules\n\nContent\n")

	result := Validate(orig, rewritten, GateOptions{})
	assert.False(t, result.Passed)
	require.GreaterOrEqual(t, len(result.Failures), 2, "should collect multiple failures")

	checks := make(map[string]bool)
	for _, f := range result.Failures {
		checks[f.Check] = true
	}
	assert.True(t, checks["field_preservation"])
	assert.True(t, checks["section_preservation"])
}

// --- Identity rewrite ---

func TestValidate_IdentityRewrite(t *testing.T) {
	content := []byte("---\nname: test\ntools: Read, Grep\nmodel: sonnet\n---\n\n## Rules\n\n- Rule 1\n- Rule 2\n\n## Process\n\n1. Step 1\n2. Step 2\n\n```go\nfmt.Println(\"hello\")\n```\n")

	result := Validate(content, content, GateOptions{})
	assert.True(t, result.Passed)
	assert.Empty(t, result.Failures)
}
