package scaffold

import (
	"strings"
	"testing"

	"github.com/mirandaguillaume/reify/internal/doctor/parser"
	"github.com/mirandaguillaume/reify/internal/scanner"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestScaffold_MinimalFile(t *testing.T) {
	analysis := &parser.AgentAnalysis{
		Format:     "claude",
		RawContent: []byte("## Rules\nBe nice"),
		Sections:   []parser.Section{{Header: "Rules", Level: 2, Content: "Be nice"}},
	}

	result, err := Scaffold(analysis, nil)
	require.NoError(t, err)

	// Should generate index + 6 files
	assert.NotEmpty(t, result.IndexContent)
	assert.Len(t, result.Files, 6)
	assert.True(t, result.TemplatedCount >= 5, "most files should be templated for minimal input")

	// Index should have links to all 6 files
	idx := string(result.IndexContent)
	assert.Contains(t, idx, ".agents/identity.md")
	assert.Contains(t, idx, ".agents/security.md")
	assert.Contains(t, idx, ".agents/testing.md")
	assert.Contains(t, idx, ".agents/architecture.md")
	assert.Contains(t, idx, ".agents/guardrails.md")
	assert.Contains(t, idx, ".agents/error-handling.md")
}

func TestScaffold_RichFile(t *testing.T) {
	analysis := &parser.AgentAnalysis{
		Format: "claude",
		RawContent: []byte("## Identity\nYou are a Go developer.\n\n## Security\nFilesystem restricted.\n\n## Testing\nRun go test ./...\n\n## Architecture\nLayered.\n\n## Guardrails\nTimeout 5min.\n\n## Error Handling\nRetry once."),
		Sections: []parser.Section{
			{Header: "Identity", Level: 2, Content: "You are a Go developer."},
			{Header: "Security", Level: 2, Content: "Filesystem restricted."},
			{Header: "Testing", Level: 2, Content: "Run go test ./..."},
			{Header: "Architecture", Level: 2, Content: "Layered."},
			{Header: "Guardrails", Level: 2, Content: "Timeout 5min."},
			{Header: "Error Handling", Level: 2, Content: "Retry once."},
		},
	}

	result, err := Scaffold(analysis, nil)
	require.NoError(t, err)

	// All sections should be migrated
	assert.True(t, result.MigratedCount >= 5, "most sections should migrate, got %d", result.MigratedCount)

	// Identity file should contain original content
	identity := string(result.Files[".agents/identity.md"])
	assert.Contains(t, identity, "You are a Go developer")

	// Security file should contain original content
	security := string(result.Files[".agents/security.md"])
	assert.Contains(t, security, "Filesystem restricted")
}

func TestScaffold_WithCodebaseContext(t *testing.T) {
	analysis := &parser.AgentAnalysis{
		Format:     "claude",
		RawContent: []byte("## Rules\nBe nice"),
		Sections:   []parser.Section{{Header: "Rules", Level: 2, Content: "Be nice"}},
	}

	ctx := &scanner.CodebaseContext{
		Stack: scanner.StackInfo{
			Languages: []scanner.LangInfo{{Name: "Go", Extension: ".go", Percentage: 85}},
		},
		Commands: []scanner.CommandInfo{
			{Name: "test", Command: "go test ./...", Source: "Makefile"},
			{Name: "build", Command: "go build ./cmd/reify", Source: "Makefile"},
		},
		Structure: []scanner.DirEntry{
			{Path: "cmd"},
			{Path: "internal"},
			{Path: "pkg"},
		},
	}

	result, err := Scaffold(analysis, ctx)
	require.NoError(t, err)

	// Index should include language and commands
	idx := string(result.IndexContent)
	assert.Contains(t, idx, "Go")
	assert.Contains(t, idx, "go test")
	assert.Contains(t, idx, "go build")

	// Testing file should include test command from scanner
	testing := string(result.Files[".agents/testing.md"])
	assert.Contains(t, testing, "go test")

	// Architecture file should include directory structure
	arch := string(result.Files[".agents/architecture.md"])
	assert.Contains(t, arch, "cmd")
	assert.Contains(t, arch, "internal")
}

func TestScaffold_NilAnalysis(t *testing.T) {
	_, err := Scaffold(nil, nil)
	assert.Error(t, err)
}

func TestScaffold_TemplateFilesHaveContent(t *testing.T) {
	analysis := &parser.AgentAnalysis{
		Format:     "claude",
		RawContent: []byte("minimal"),
		Sections:   nil,
	}

	result, err := Scaffold(analysis, nil)
	require.NoError(t, err)

	// All files should have meaningful content (at least a heading)
	for path, content := range result.Files {
		assert.True(t, len(content) > 20, "file %s should have content, got %d bytes", path, len(content))
		assert.Contains(t, string(content), "#", "file %s should have a markdown heading", path)
	}

	// Identity and security should have TODO markers (they need project-specific info)
	assert.Contains(t, string(result.Files[".agents/identity.md"]), "TODO")
	assert.Contains(t, string(result.Files[".agents/security.md"]), "TODO")
}

func TestGenerateIndex_Structure(t *testing.T) {
	analysis := &parser.AgentAnalysis{Format: "claude"}
	idx := generateIndex(analysis, nil)

	// Should have header, quick reference, and file table
	assert.Contains(t, idx, "# Agent Configuration")
	assert.Contains(t, idx, "## Quick Reference")
	assert.Contains(t, idx, "## Specialized Guides")
	assert.Contains(t, idx, "| File | Covers |")

	// Should link to all 6 default files
	for _, sf := range DefaultFiles {
		assert.Contains(t, idx, sf.Name)
		assert.Contains(t, idx, sf.Description)
	}
}

func TestMigrateSections_RoutesToCorrectFiles(t *testing.T) {
	analysis := &parser.AgentAnalysis{
		Sections: []parser.Section{
			{Header: "Security Policy", Level: 2, Content: "Filesystem restricted."},
			{Header: "Test Guide", Level: 2, Content: "Run npm test."},
			{Header: "Random Section", Level: 2, Content: "Something custom."},
		},
	}

	migrations := migrateSections(analysis)

	// Security section → security.md
	assert.NotEmpty(t, migrations["security.md"])
	assert.True(t, strings.Contains(migrations["security.md"][0], "Filesystem restricted"))

	// Test section → testing.md
	assert.NotEmpty(t, migrations["testing.md"])
	assert.True(t, strings.Contains(migrations["testing.md"][0], "Run npm test"))

	// Unmatched → identity.md (catch-all)
	assert.NotEmpty(t, migrations["identity.md"])
	assert.True(t, strings.Contains(migrations["identity.md"][0], "Something custom"))
}
