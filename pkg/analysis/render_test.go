package analysis

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRender_BasicOutput(t *testing.T) {
	files := []DiffFile{
		{Path: "auth.go", Language: "go"},
	}
	asts := map[string]*ASTContext{
		"auth.go": {
			Symbols: []ASTSymbol{
				{Kind: "function", Name: "HandleLogin", StartLine: 10, EndLine: 20,
					Body: "func HandleLogin() {\n\treturn\n}", Changed: true},
			},
		},
	}
	patterns := []PatternHit{
		{Category: "security", Rule: "SQL injection", File: "auth.go",
			Line: 15, Snippet: `db.Query("SELECT " + x)`, Severity: "high"},
	}

	out := Render(files, asts, patterns)

	assert.Contains(t, out, "## File: auth.go (go)")
	assert.Contains(t, out, "### Changed Symbols")
	assert.Contains(t, out, "`function HandleLogin`")
	assert.Contains(t, out, "### Flagged Patterns")
	assert.Contains(t, out, "[security]")
	assert.Contains(t, out, "severity: high")
}

func TestRender_NoAST(t *testing.T) {
	files := []DiffFile{
		{
			Path:     "config.xyz",
			Language: "",
			Hunks: []DiffHunk{
				{Lines: []DiffLine{
					{Kind: LineAdded, Content: "new line"},
					{Kind: LineRemoved, Content: "old line"},
				}},
			},
		},
	}

	out := Render(files, nil, nil)
	assert.Contains(t, out, "No AST available")
	assert.Contains(t, out, "+1/-1")
}

func TestRender_MultipleFiles(t *testing.T) {
	files := []DiffFile{
		{Path: "a.go", Language: "go"},
		{Path: "b.py", Language: "python"},
	}

	out := Render(files, nil, nil)
	assert.Contains(t, out, "## File: a.go")
	assert.Contains(t, out, "## File: b.py")
}

func TestRender_LongBodyTruncated(t *testing.T) {
	// Build a body with 50 lines
	var lines []string
	for i := 0; i < 50; i++ {
		lines = append(lines, "    line content")
	}
	body := strings.Join(lines, "\n")

	files := []DiffFile{{Path: "big.go", Language: "go"}}
	asts := map[string]*ASTContext{
		"big.go": {
			Symbols: []ASTSymbol{
				{Kind: "function", Name: "BigFunc", StartLine: 1, EndLine: 50,
					Body: body, Changed: true},
			},
		},
	}

	out := Render(files, asts, nil)
	assert.Contains(t, out, "lines omitted")
}

func TestAnalyze_EndToEnd(t *testing.T) {
	diff := `diff --git a/handler.go b/handler.go
--- a/handler.go
+++ b/handler.go
@@ -1,3 +1,5 @@
 package main
+
+func Handle() { log.Printf("token: %s", secret) }
+
 func main() {}
`
	result, err := Analyze(diff, nil)
	require.NoError(t, err)
	assert.NotEmpty(t, result)
	assert.Contains(t, result, "handler.go")
}

func TestAnalyze_EmptyDiff(t *testing.T) {
	result, err := Analyze("", nil)
	require.NoError(t, err)
	assert.Empty(t, result)
}

func TestAnalyze_WithSourceFiles(t *testing.T) {
	diff := `diff --git a/app.go b/app.go
--- a/app.go
+++ b/app.go
@@ -3,3 +3,4 @@ package main
 func Hello() {
+	fmt.Println("world")
 }
`
	source := map[string][]byte{
		"app.go": []byte(`package main

func Hello() {
	fmt.Println("world")
}
`),
	}
	result, err := Analyze(diff, source)
	require.NoError(t, err)
	assert.Contains(t, result, "app.go")
	assert.Contains(t, result, "Hello")
}
