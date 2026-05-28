package importer

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestExtractFrontmatter(t *testing.T) {
	content := `---
name: code-reviewer
description: Reviews pull requests
tools:
  - Read
  - Write
  - Bash
model: claude-sonnet-4-20250514
---
You are a code reviewer.
`
	fm, body, err := ExtractFrontmatter(content)

	require.NoError(t, err)
	assert.Equal(t, "code-reviewer", fm.Name)
	assert.Equal(t, "Reviews pull requests", fm.Description)
	assert.Equal(t, "claude-sonnet-4-20250514", fm.Model)
	assert.Equal(t, []string{"Read", "Write", "Bash"}, fm.Tools)
	assert.Contains(t, body, "You are a code reviewer.")
	assert.NotContains(t, body, "---")
}

func TestExtractFrontmatterNoFrontmatter(t *testing.T) {
	content := "Just a plain markdown document.\nNo frontmatter here."

	fm, body, err := ExtractFrontmatter(content)

	require.NoError(t, err)
	assert.Empty(t, fm.Name)
	assert.Empty(t, fm.Tools)
	assert.Equal(t, content, body)
}

func TestExtractFrontmatterToolsAsString(t *testing.T) {
	content := `---
name: helper
description: A helper agent
tools: "Read, Write, Bash"
model: gpt-4
---
Body text here.
`
	fm, body, err := ExtractFrontmatter(content)

	require.NoError(t, err)
	assert.Equal(t, "helper", fm.Name)
	assert.Equal(t, []string{"Read", "Write", "Bash"}, fm.Tools)
	assert.Contains(t, body, "Body text here.")
}

func TestExtractFrontmatter_OnlyOpeningMarker(t *testing.T) {
	content := "---\nname: broken\nNo closing marker."

	fm, body, err := ExtractFrontmatter(content)

	require.NoError(t, err)
	assert.Empty(t, fm.Name)
	assert.Equal(t, content, body)
}

func TestExtractFrontmatter_EmptyTools(t *testing.T) {
	content := `---
name: minimal
description: No tools defined
---
Body.
`
	fm, body, err := ExtractFrontmatter(content)

	require.NoError(t, err)
	assert.Equal(t, "minimal", fm.Name)
	assert.Nil(t, fm.Tools)
	assert.Contains(t, body, "Body.")
}
