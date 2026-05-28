package parser

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestClaudeParser_Format(t *testing.T) {
	p := &claudeParser{}
	assert.Equal(t, "claude", p.Format())
}

func TestClaudeParser_Detect(t *testing.T) {
	tests := []struct {
		name    string
		path    string
		content string
		want    bool
	}{
		{"claude agent path", ".claude/agents/code-reviewer.md", "---\nname: test\n---\nBody", true},
		{"claude skill path", ".claude/skills/linter/SKILL.md", "---\nname: linter\n---\nBody", true},
		{"CLAUDE.md", "CLAUDE.md", "# Project instructions", true},
		{"copilot agent", ".github/agents/test.agent.md", "---\nname: test\n---\nBody", false},
		{"random markdown", "docs/readme.md", "# Hello world", false},
		{"frontmatter with name+tools (no claude path)", "agent.md", "---\nname: test\ntools: Read\n---\nBody", true},
		{"frontmatter with name+model (no claude path)", "agent.md", "---\nname: test\nmodel: sonnet\n---\nBody", true},
		{"frontmatter name only (no tools/model)", "agent.md", "---\nname: test\n---\nBody", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p := &claudeParser{}
			got := p.Detect(tt.path, []byte(tt.content))
			assert.Equal(t, tt.want, got, "Detect(%s)", tt.path)
		})
	}
}

func TestClaudeParser_Parse_WithFrontmatter(t *testing.T) {
	content := []byte(`---
name: code-reviewer
description: Reviews code
tools: Read, Grep, Glob
model: sonnet
---

You are a code reviewer.

## Rules

- Do not approve without tests
- Always check for security issues

## Process

1. Read the diff
2. Check for issues
3. Write comments
`)

	p := &claudeParser{}
	analysis, err := p.Parse(content)
	require.NoError(t, err)

	assert.Equal(t, "claude", analysis.Format)
	assert.Equal(t, "code-reviewer", analysis.Frontmatter["name"])
	assert.Equal(t, "Reviews code", analysis.Frontmatter["description"])
	assert.Equal(t, "sonnet", analysis.Frontmatter["model"])
	assert.Equal(t, []string{"Read", "Grep", "Glob"}, analysis.Tools)
	assert.GreaterOrEqual(t, len(analysis.Sections), 2) // At least Rules + Process
	assert.Equal(t, content, analysis.RawContent)
}

func TestClaudeParser_Parse_WithoutFrontmatter(t *testing.T) {
	content := []byte(`You are a code reviewer.

## Rules

- Check for bugs
`)

	p := &claudeParser{}
	analysis, err := p.Parse(content)
	require.NoError(t, err)

	assert.Equal(t, "claude", analysis.Format)
	assert.Empty(t, analysis.Frontmatter)
	assert.Empty(t, analysis.Tools)
	assert.NotEmpty(t, analysis.Sections)
}

func TestClaudeParser_Parse_UnquotedColonsInDescription(t *testing.T) {
	// Real-world pattern: description contains colons that break yaml.v3
	content := []byte(`---
name: qa
description: Use this agent when you need to test. Examples: <example>user: "test it"<br/>assistant: "ok"</example>
model: sonnet
color: blue
---

You are a QA specialist.
`)
	p := &claudeParser{}
	analysis, err := p.Parse(content)
	require.NoError(t, err)
	assert.Equal(t, "qa", analysis.Frontmatter["name"])
	assert.Equal(t, "sonnet", analysis.Frontmatter["model"])
	assert.Equal(t, "blue", analysis.Frontmatter["color"])
	assert.NotEmpty(t, analysis.Frontmatter["description"])
}

func TestClaudeParser_Detect_UnquotedColonsInDescription(t *testing.T) {
	// Files with colons in description should still be detected via frontmatter
	content := []byte(`---
name: tester
description: Test agent. Examples: user: "run tests" assistant: "ok"
model: haiku
---
Body
`)
	p := &claudeParser{}
	got := p.Detect("random/agent.md", content)
	assert.True(t, got, "should detect Claude file even with colons in description")
}

func TestClaudeParser_Parse_MultilineEscapedDescription(t *testing.T) {
	// Real-world: description has literal \n sequences (not actual newlines)
	content := []byte("---\nname: tester\ndescription: Use this agent for testing.\\n\\n<example>\\nuser: \"test\"\\nassistant: \"ok\"\\n</example>\nmodel: haiku\n---\nBody")

	p := &claudeParser{}
	analysis, err := p.Parse(content)
	require.NoError(t, err)
	assert.Equal(t, "tester", analysis.Frontmatter["name"])
	assert.Equal(t, "haiku", analysis.Frontmatter["model"])
	assert.NotEmpty(t, analysis.Frontmatter["description"])
}

func TestClaudeParser_Parse_EmptyFrontmatter(t *testing.T) {
	content := []byte("---\n---\nBody text here")

	p := &claudeParser{}
	analysis, err := p.Parse(content)
	require.NoError(t, err)
	assert.NotNil(t, analysis.Frontmatter)
}

func TestClaudeParser_Parse_ToolsAsList(t *testing.T) {
	content := []byte("---\nname: test\ntools:\n  - Read\n  - Grep\n---\nBody")

	p := &claudeParser{}
	analysis, err := p.Parse(content)
	require.NoError(t, err)
	assert.Equal(t, []string{"Read", "Grep"}, analysis.Tools)
}

func TestClaudeParser_Parse_AllTestdata(t *testing.T) {
	files, err := filepath.Glob("testdata/claude/*.md")
	if err != nil || len(files) == 0 {
		t.Skip("testdata/claude/ not found — run from parser package directory")
	}

	p := &claudeParser{}
	for _, f := range files {
		t.Run(filepath.Base(f), func(t *testing.T) {
			content, err := os.ReadFile(f)
			require.NoError(t, err)

			// Detect with simulated Claude path (testdata paths don't have .claude/)
			claudePath := ".claude/agents/" + filepath.Base(f)
			assert.True(t, p.Detect(claudePath, content), "Detect should return true for %s (as %s)", f, claudePath)

			// All should parse without error
			analysis, err := p.Parse(content)
			require.NoError(t, err, "Parse failed for %s", f)
			assert.Equal(t, "claude", analysis.Format)
			assert.NotNil(t, analysis.RawContent)
			assert.NotNil(t, analysis)
		})
	}

	t.Logf("Successfully parsed %d Claude testdata files", len(files))
}

func TestClaudeParser_Parse_TestdataFrontmatterExtracted(t *testing.T) {
	// Verify that files with unquoted colons in description now have frontmatter
	// properly extracted (previously fell through to empty map).
	tests := []struct {
		file     string
		wantName string
		wantKey  string // extra key to check
	}{
		{"testdata/claude/zen-qa.md", "qa", "model"},
		{"testdata/claude/shadcn-tester.md", "tester", "model"},
	}

	p := &claudeParser{}
	for _, tt := range tests {
		t.Run(filepath.Base(tt.file), func(t *testing.T) {
			content, err := os.ReadFile(tt.file)
			if err != nil {
				t.Skipf("testdata not found: %s", tt.file)
			}

			analysis, err := p.Parse(content)
			require.NoError(t, err)
			assert.Equal(t, tt.wantName, analysis.Frontmatter["name"],
				"frontmatter name should be extracted")
			assert.NotNil(t, analysis.Frontmatter[tt.wantKey],
				"frontmatter %s should be extracted", tt.wantKey)
			assert.NotEmpty(t, analysis.Frontmatter["description"],
				"frontmatter description should be extracted")
		})
	}
}

func TestClaudeParser_Detect_FrontmatterOnly(t *testing.T) {
	// Files with frontmatter should be detected even without .claude/ path
	p := &claudeParser{}

	// Has name + description (Claude-typical)
	tests := []struct {
		name    string
		content string
		want    bool
	}{
		{"name+tools", "---\nname: x\ntools: Read\n---\n", true},
		{"name+model", "---\nname: x\nmodel: sonnet\n---\n", true},
		{"name+description", "---\nname: x\ndescription: y\n---\n", true},
		{"name+color", "---\nname: x\ncolor: blue\n---\n", true},
		{"name only", "---\nname: x\n---\n", false},
		{"no frontmatter", "Just text", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := p.Detect("random/file.md", []byte(tt.content))
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestClaudeParser_Validate_PreservesStructure(t *testing.T) {
	original := []byte("---\nname: test\ntools: Read\n---\n\n## Rules\n\n- Rule 1\n\n## Process\n\n1. Step 1\n")
	rewritten := []byte("---\nname: test\ntools: Read\nguidelines: added\n---\n\n## Rules\n\n- Rule 1 (improved)\n\n## Process\n\n1. Step 1\n")

	p := &claudeParser{}
	err := p.Validate(original, rewritten)
	assert.NoError(t, err)
}

func TestClaudeParser_Validate_LostField(t *testing.T) {
	original := []byte("---\nname: test\ntools: Read\n---\nBody")
	rewritten := []byte("---\nname: test\n---\nBody") // Lost 'tools'

	p := &claudeParser{}
	err := p.Validate(original, rewritten)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "lost frontmatter field")
}

func TestClaudeParser_Validate_LostSection(t *testing.T) {
	original := []byte("---\nname: test\n---\n\n## Rules\n\nContent\n\n## Process\n\nContent\n")
	rewritten := []byte("---\nname: test\n---\n\n## Rules\n\nContent\n") // Lost Process

	p := &claudeParser{}
	err := p.Validate(original, rewritten)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "lost section")
}

func TestClaudeParser_Validate_BrokenFrontmatter(t *testing.T) {
	original := []byte("---\nname: test\n---\nBody")
	rewritten := []byte("---\n: invalid yaml {{{\n---\nBody")

	p := &claudeParser{}
	err := p.Validate(original, rewritten)
	require.Error(t, err)
	// Tolerant parser parses broken YAML to empty map, so validation
	// catches the problem as a lost field rather than a parse error.
	assert.Contains(t, err.Error(), "lost frontmatter field")
}
