package context

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/mirandaguillaume/reify/internal/doctor/parser"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// makeGoProject creates a minimal Go project in a temp directory.
func makeGoProject(t *testing.T) string {
	t.Helper()
	root := t.TempDir()

	// go.mod
	require.NoError(t, os.WriteFile(filepath.Join(root, "go.mod"),
		[]byte("module example.com/myapp\n\ngo 1.22\n"), 0644))

	// Source files
	require.NoError(t, os.MkdirAll(filepath.Join(root, "cmd"), 0755))
	require.NoError(t, os.WriteFile(filepath.Join(root, "cmd", "main.go"),
		[]byte("package main\n\nfunc main() {}\n"), 0644))

	require.NoError(t, os.MkdirAll(filepath.Join(root, "internal", "handler"), 0755))
	require.NoError(t, os.WriteFile(filepath.Join(root, "internal", "handler", "handler.go"),
		[]byte("package handler\n\ntype Handler interface {\n\tServe()\n}\n"), 0644))

	// Test files
	require.NoError(t, os.WriteFile(filepath.Join(root, "internal", "handler", "handler_test.go"),
		[]byte("package handler\n\nimport \"testing\"\n\nfunc TestHandler(t *testing.T) {}\n"), 0644))

	require.NoError(t, os.MkdirAll(filepath.Join(root, "pkg", "model"), 0755))
	require.NoError(t, os.WriteFile(filepath.Join(root, "pkg", "model", "model.go"),
		[]byte("package model\n\ntype User struct {\n\tName string\n}\n"), 0644))
	require.NoError(t, os.WriteFile(filepath.Join(root, "pkg", "model", "model_test.go"),
		[]byte("package model\n\nimport \"testing\"\n\nfunc TestUser(t *testing.T) {}\n"), 0644))

	return root
}

func analysis(content string) *parser.AgentAnalysis {
	return &parser.AgentAnalysis{
		RawContent: []byte(content),
	}
}

// --- DetectProjectRoot ---

func TestDetectProjectRoot_GoMod(t *testing.T) {
	root := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(root, "go.mod"), []byte("module x"), 0644))
	sub := filepath.Join(root, "internal", "deep")
	require.NoError(t, os.MkdirAll(sub, 0755))

	assert.Equal(t, root, DetectProjectRoot(sub))
}

func TestDetectProjectRoot_Git(t *testing.T) {
	root := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(root, ".git"), 0755))

	assert.Equal(t, root, DetectProjectRoot(root))
}

func TestDetectProjectRoot_PackageJSON(t *testing.T) {
	root := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(root, "package.json"), []byte("{}"), 0644))

	assert.Equal(t, root, DetectProjectRoot(root))
}

func TestDetectProjectRoot_NoMarkers(t *testing.T) {
	root := t.TempDir()
	assert.Equal(t, "", DetectProjectRoot(root))
}

func TestDetectProjectRoot_AtRoot(t *testing.T) {
	assert.Equal(t, "", DetectProjectRoot("/"))
}

// --- Enrich ---

func TestEnrich_EmptyRoot(t *testing.T) {
	findings, err := Enrich(analysis("some agent content"), "")
	require.NoError(t, err)
	assert.Nil(t, findings)
}

func TestEnrich_NoProjectRoot(t *testing.T) {
	root := t.TempDir() // empty dir, no project files
	findings, err := Enrich(analysis("some agent content"), root)
	require.NoError(t, err)
	// Scanner may or may not fail on empty dir — either way, findings should be nil or empty
	assert.Empty(t, findings)
}

func TestEnrich_AllGaps(t *testing.T) {
	root := makeGoProject(t)
	// Agent content that doesn't mention anything about the project
	a := analysis("This agent reviews code and suggests improvements.")
	findings, err := Enrich(a, root)
	require.NoError(t, err)

	// Should find gaps in multiple categories
	assert.NotEmpty(t, findings, "should find context gaps")

	categories := map[string]bool{}
	for _, f := range findings {
		categories[f.Category] = true
		assert.Equal(t, "context", f.Category)
		assert.NotEmpty(t, f.Issue)
		assert.NotEmpty(t, f.Confidence)
		assert.NotEmpty(t, f.SuggestedImprovement)
	}
}

func TestEnrich_AgentMentionsEverything(t *testing.T) {
	root := makeGoProject(t)
	// Agent that mentions testing, structure, symbols, security, Go
	a := analysis(`This Go agent reviews code.
It runs testing suites and asserts correctness.
It operates within the internal/handler and pkg/model directories.
It understands the Handler interface and User struct.
It has security restrictions on filesystem and network access.`)

	findings, err := Enrich(a, root)
	require.NoError(t, err)
	assert.Empty(t, findings, "agent covers all categories — no gaps expected")
}

// --- Individual gap analysis functions ---

func TestCheckTestCoverage_NoTests(t *testing.T) {
	root := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(root, "go.mod"), []byte("module x"), 0644))
	require.NoError(t, os.MkdirAll(filepath.Join(root, "cmd"), 0755))
	require.NoError(t, os.WriteFile(filepath.Join(root, "cmd", "main.go"),
		[]byte("package main"), 0644))

	a := analysis("some content")
	findings, err := Enrich(a, root)
	require.NoError(t, err)

	for _, f := range findings {
		assert.NotContains(t, f.Issue, "testing",
			"no test files exist — should not flag testing gap")
	}
}

func TestCheckTestCoverage_HasTestsMentioned(t *testing.T) {
	root := makeGoProject(t)
	a := analysis("Run tests with go test ./...")
	findings, err := Enrich(a, root)
	require.NoError(t, err)

	for _, f := range findings {
		assert.NotContains(t, f.Issue, "testing guidance",
			"agent mentions testing — should not flag")
	}
}

func TestCheckTestCoverage_HasTestsNotMentioned(t *testing.T) {
	root := makeGoProject(t)
	a := analysis("This agent reviews code.")
	findings, err := Enrich(a, root)
	require.NoError(t, err)

	found := false
	for _, f := range findings {
		if f.Issue == "Agent has no testing guidance" {
			found = true
			assert.Equal(t, "high", f.Confidence)
			assert.Contains(t, f.CurrentState, "test file")
		}
	}
	assert.True(t, found, "should flag missing testing guidance")
}

func TestCheckStackAwareness_MentionsGo(t *testing.T) {
	root := makeGoProject(t)
	a := analysis("This Go code reviewer follows Go conventions.")
	findings, err := Enrich(a, root)
	require.NoError(t, err)

	for _, f := range findings {
		assert.NotContains(t, f.Issue, "Go conventions",
			"agent mentions Go — should not flag stack awareness")
	}
}

func TestCheckStackAwareness_MentionsGolang(t *testing.T) {
	root := makeGoProject(t)
	a := analysis("This agent follows golang best practices.")
	findings, err := Enrich(a, root)
	require.NoError(t, err)

	for _, f := range findings {
		assert.NotContains(t, f.Issue, "Go conventions",
			"agent mentions golang alias — should not flag")
	}
}

func TestCheckStackAwareness_NotMentioned(t *testing.T) {
	root := makeGoProject(t)
	a := analysis("This agent reviews code.")
	findings, err := Enrich(a, root)
	require.NoError(t, err)

	found := false
	for _, f := range findings {
		if f.Confidence == "high" && containsStr(f.Issue, "Go") {
			found = true
		}
	}
	assert.True(t, found, "should flag missing stack awareness")
}

func TestCheckStructureAwareness_ReferencesDir(t *testing.T) {
	root := makeGoProject(t)
	a := analysis("Look at the internal directory for implementation details.")
	findings, err := Enrich(a, root)
	require.NoError(t, err)

	for _, f := range findings {
		assert.NotContains(t, f.Issue, "project directories",
			"agent references 'internal' — should not flag")
	}
}

func TestCheckStructureAwareness_NoDirReferences(t *testing.T) {
	root := makeGoProject(t)
	a := analysis("This agent reviews code.")
	findings, err := Enrich(a, root)
	require.NoError(t, err)

	found := false
	for _, f := range findings {
		if f.Issue == "Agent doesn't reference any project directories" {
			found = true
			assert.Equal(t, "moderate", f.Confidence)
		}
	}
	assert.True(t, found, "should flag missing directory references")
}

func TestEnrich_AllFindingsHaveContextCategory(t *testing.T) {
	root := makeGoProject(t)
	a := analysis("blank agent")
	findings, err := Enrich(a, root)
	require.NoError(t, err)

	for _, f := range findings {
		assert.Equal(t, "context", f.Category)
	}
}

func TestEnrich_CRLFNormalization(t *testing.T) {
	root := makeGoProject(t)
	// Agent content with CRLF that mentions "test"
	a := analysis("This agent runs\r\ntesting.\r\n")
	findings, err := Enrich(a, root)
	require.NoError(t, err)

	for _, f := range findings {
		assert.NotContains(t, f.Issue, "testing guidance",
			"CRLF-normalized content should still match 'test' keyword")
	}
}

// --- isTestFile ---

func TestIsTestFile(t *testing.T) {
	tests := []struct {
		name string
		want bool
	}{
		{"handler_test.go", true},
		{"handler.go", false},
		{"app.test.js", true},
		{"app.test.ts", true},
		{"app.test.tsx", true},
		{"app.spec.js", true},
		{"app.spec.ts", true},
		{"test_utils.py", false},
		{"app_test.py", true},
		{"README.md", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, isTestFile(tt.name))
		})
	}
}

// --- formatCount ---

func TestFormatCount_Singular(t *testing.T) {
	assert.Equal(t, "Project has 1 test file", formatCount(1, "test file"))
}

func TestFormatCount_Plural(t *testing.T) {
	assert.Equal(t, "Project has 47 test files", formatCount(47, "test file"))
}

// --- Benchmark ---

func BenchmarkEnrich(b *testing.B) {
	// Walk up from the test package to find the project root
	wd, err := os.Getwd()
	if err != nil {
		b.Skip("cannot get working directory")
	}
	root := DetectProjectRoot(wd)
	if root == "" {
		b.Skip("no project root found")
	}
	a := analysis("This agent reviews code.")
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = Enrich(a, root)
	}
}

// --- CheckTextOrdering ---

func TestCheckTextOrdering_HeavyOrdering(t *testing.T) {
	a := analysis(`# Agent Workflow

### Step 1: Lint
First, run the linter on all files.

### Step 2: Review
Then, check the code for issues.

### Step 3: Deploy
Next, deploy to staging.

### Step 4: Verify
Finally, verify the deployment.

workflow:
  1. Lint
  2. Review
  3. Deploy
  4. Verify`)

	findings := CheckTextOrdering(a)
	require.Len(t, findings, 1)
	assert.Equal(t, "ordering", findings[0].Category)
	assert.Equal(t, "high", findings[0].Confidence)
	assert.Equal(t, "ordering", findings[0].CitationID)
	assert.Contains(t, findings[0].SuggestedImprovement, "structural enforcement")
}

func TestCheckTextOrdering_NoOrdering(t *testing.T) {
	a := analysis(`# Code Reviewer
This agent reviews code for bugs and style issues.
It uses Read and Grep tools to analyze the codebase.`)

	findings := CheckTextOrdering(a)
	assert.Empty(t, findings, "no ordering instructions — should not flag")
}

func TestCheckTextOrdering_BelowThreshold(t *testing.T) {
	// Only 2 indicators — below the >= 3 threshold
	a := analysis(`# Agent
First, read the file.
Then, analyze it.`)

	findings := CheckTextOrdering(a)
	assert.Empty(t, findings, "only 2 indicators — below threshold")
}

func TestCheckTextOrdering_NumberedSteps(t *testing.T) {
	a := analysis(`# Deployment Agent
Follow this process:
1. Build the project
2. Run tests
3. Deploy to staging
4. Monitor logs
Then verify the deployment worked.
Finally, notify the team.`)

	findings := CheckTextOrdering(a)
	require.Len(t, findings, 1)
	assert.Equal(t, "ordering", findings[0].Category)
}

func TestCheckTextOrdering_WorkflowMarkers(t *testing.T) {
	a := analysis(`# CI Agent
workflow:
  pipeline:
    first, lint all files
    then run tests
    next, deploy`)

	findings := CheckTextOrdering(a)
	require.Len(t, findings, 1)
}

func containsStr(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(substr) == 0 ||
		(len(s) > 0 && len(substr) > 0 && findSubstring(s, substr)))
}

func findSubstring(s, sub string) bool {
	for i := 0; i <= len(s)-len(sub); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}
