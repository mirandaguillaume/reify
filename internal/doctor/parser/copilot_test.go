package parser

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCopilotParser_Format(t *testing.T) {
	p := &copilotParser{}
	assert.Equal(t, "copilot", p.Format())
}

func TestCopilotParser_Detect(t *testing.T) {
	tests := []struct {
		name    string
		path    string
		content string
		want    bool
	}{
		{"copilot agent path", ".github/agents/dash.agent.md", "---\ndescription: test\n---", true},
		{"copilot skill path", ".github/skills/foo/SKILL.md", "---\nname: foo\n---", true},
		{"copilot-instructions.md", ".github/copilot-instructions.md", "# Instructions", true},
		{"claude agent", ".claude/agents/test.md", "---\nname: test\n---", false},
		{"random markdown", "docs/readme.md", "# Hello", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p := &copilotParser{}
			got := p.Detect(tt.path, []byte(tt.content))
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestCopilotParser_Parse_AllTestdata(t *testing.T) {
	files, err := filepath.Glob("testdata/copilot/*.md")
	if err != nil || len(files) == 0 {
		t.Skip("testdata/copilot/ not found")
	}

	p := &copilotParser{}
	for _, f := range files {
		t.Run(filepath.Base(f), func(t *testing.T) {
			content, err := os.ReadFile(f)
			require.NoError(t, err)

			// Detect with simulated Copilot path
			copilotPath := ".github/agents/" + filepath.Base(f)
			assert.True(t, p.Detect(copilotPath, content), "Detect should return true for %s", f)

			analysis, err := p.Parse(content)
			require.NoError(t, err)
			assert.Equal(t, "copilot", analysis.Format)
			assert.NotNil(t, analysis.RawContent)
		})
	}
	t.Logf("Successfully parsed %d Copilot testdata files", len(files))
}

func TestCopilotParser_Parse_JSONTools(t *testing.T) {
	content := []byte(`---
description: "Expert in performance"
tools: ["read", "search", "bash"]
---

# Performance Agent

Review code for performance.
`)
	p := &copilotParser{}
	analysis, err := p.Parse(content)
	require.NoError(t, err)
	assert.Equal(t, []string{"read", "search", "bash"}, analysis.Tools)
}
