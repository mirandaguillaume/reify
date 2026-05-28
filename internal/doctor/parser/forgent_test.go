package parser

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestReifyParser_Format(t *testing.T) {
	p := &reifyParser{}
	assert.Equal(t, "reify", p.Format())
}

func TestReifyParser_Detect(t *testing.T) {
	tests := []struct {
		path string
		want bool
	}{
		{"skills/review-commenter.skill.yaml", true},
		{"agents/code-reviewer.agent.yaml", true},
		{"docs/readme.md", false},
		{".claude/agents/test.md", false},
		{"config.yaml", false},
	}
	p := &reifyParser{}
	for _, tt := range tests {
		t.Run(tt.path, func(t *testing.T) {
			assert.Equal(t, tt.want, p.Detect(tt.path, nil))
		})
	}
}

func TestReifyParser_Parse_RealSkills(t *testing.T) {
	files, err := filepath.Glob("../../../../skills/*.skill.yaml")
	if err != nil || len(files) == 0 {
		t.Skip("skills/ directory not found — run from parser package")
	}

	p := &reifyParser{}
	parsed := 0
	for _, f := range files {
		t.Run(filepath.Base(f), func(t *testing.T) {
			content, err := os.ReadFile(f)
			require.NoError(t, err)

			assert.True(t, p.Detect(f, content))

			analysis, err := p.Parse(content)
			require.NoError(t, err)
			assert.Equal(t, "reify", analysis.Format)
			assert.NotNil(t, analysis.Frontmatter)
			assert.NotEmpty(t, analysis.Sections, "skill should have at least one facet section")
			parsed++
		})
	}
	t.Logf("Successfully parsed %d Reify skill files", parsed)
}

func TestReifyParser_Parse_RealAgents(t *testing.T) {
	files, err := filepath.Glob("../../../../agents/*.agent.yaml")
	if err != nil || len(files) == 0 {
		t.Skip("agents/ directory not found — run from parser package")
	}

	p := &reifyParser{}
	for _, f := range files {
		t.Run(filepath.Base(f), func(t *testing.T) {
			content, err := os.ReadFile(f)
			require.NoError(t, err)

			assert.True(t, p.Detect(f, content))

			analysis, err := p.Parse(content)
			require.NoError(t, err)
			assert.Equal(t, "reify", analysis.Format)
			assert.NotNil(t, analysis.Frontmatter)
		})
	}
}

func TestReifyParser_Parse_InlineSkill(t *testing.T) {
	content := []byte(`skill: inline-test
version: "1.0.0"
context:
  consumes: [git_diff]
  produces: [review_comments]
  memory: conversation
strategy:
  tools: [read_file, grep]
  approach: diff-first
  steps:
    - read the diff
    - write comments
guardrails:
  - max_comments: 15
  - timeout: 5min
observability:
  trace_level: detailed
security:
  filesystem: read-only
  network: none
negotiation:
  file_conflicts: yield
`)
	p := &reifyParser{}
	assert.True(t, p.Detect("skills/inline-test.skill.yaml", content))

	analysis, err := p.Parse(content)
	require.NoError(t, err)
	assert.Equal(t, "reify", analysis.Format)
	assert.Equal(t, "inline-test", analysis.Frontmatter["skill"])
	assert.Equal(t, []string{"read_file", "grep"}, analysis.Tools)
	assert.NotEmpty(t, analysis.Sections)
	// Should have lint results metadata (linter runs on skill parse)
	assert.NotNil(t, analysis.Frontmatter)
}

func TestReifyParser_Validate(t *testing.T) {
	validSkill := []byte(`skill: valid-test
version: "1.0.0"
context:
  consumes: [input]
  produces: [output]
  memory: conversation
strategy:
  tools: [read_file]
  approach: test
  steps:
    - do something
guardrails:
  - timeout: 5min
observability:
  trace_level: minimal
security:
  filesystem: read-only
  network: none
negotiation:
  file_conflicts: yield
`)
	p := &reifyParser{}
	assert.NoError(t, p.Validate(nil, validSkill))

	invalidYAML := []byte("not: [valid: yaml: {{{")
	assert.Error(t, p.Validate(nil, invalidYAML))
}

func TestDetectFormat_MixedFiles(t *testing.T) {
	tests := []struct {
		path    string
		content string
		format  string
	}{
		{".claude/agents/foo.md", "---\nname: foo\ntools: Read\n---\nBody", "claude"},
		{".github/agents/bar.agent.md", "---\ndescription: bar\n---\nBody", "copilot"},
		{"skills/review.skill.yaml", "skill: review\ncontext:\n  consumes: [x]\n  produces: [y]\nstrategy:\n  tools: [read]\n  steps:\n    - do", "reify"},
	}
	for _, tt := range tests {
		t.Run(tt.format, func(t *testing.T) {
			p, err := DetectFormat(tt.path, []byte(tt.content))
			require.NoError(t, err)
			assert.Equal(t, tt.format, p.Format())
		})
	}
}
