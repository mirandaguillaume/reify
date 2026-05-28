package analysis

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestScanPatterns_SQLInjection(t *testing.T) {
	files := []DiffFile{
		{
			Path:     "handler.go",
			Language: "go",
			Hunks: []DiffHunk{
				{
					Lines: []DiffLine{
						{Kind: LineAdded, Content: `	query := "SELECT * FROM users WHERE id=" + userID`, NewLine: 10},
					},
				},
			},
		},
	}
	hits := ScanPatterns(files, nil)

	var sqli []PatternHit
	for _, h := range hits {
		if h.Category == "security" {
			sqli = append(sqli, h)
		}
	}
	require.NotEmpty(t, sqli, "should detect SQL injection")
	assert.Equal(t, "high", sqli[0].Severity)
	assert.Equal(t, 10, sqli[0].Line)
}

func TestScanPatterns_SQLFmtSprintf(t *testing.T) {
	files := []DiffFile{
		{
			Path:     "db.go",
			Language: "go",
			Hunks: []DiffHunk{
				{
					Lines: []DiffLine{
						{Kind: LineAdded, Content: `	q := fmt.Sprintf("SELECT * FROM users WHERE name='%s'", name)`, NewLine: 5},
					},
				},
			},
		},
	}
	hits := ScanPatterns(files, nil)
	var found bool
	for _, h := range hits {
		if h.Category == "security" && h.Rule == "possible SQL injection — format string in query" {
			found = true
		}
	}
	assert.True(t, found, "should detect fmt.Sprintf SQL injection")
}

func TestScanPatterns_ErrorIgnored(t *testing.T) {
	files := []DiffFile{
		{
			Path:     "service.go",
			Language: "go",
			Hunks: []DiffHunk{
				{
					Lines: []DiffLine{
						{Kind: LineAdded, Content: `	db.UpdateLastLogin(user)`, NewLine: 15},
					},
				},
			},
		},
	}
	hits := ScanPatterns(files, nil)

	var found bool
	for _, h := range hits {
		if h.Category == "resource_handling" {
			found = true
		}
	}
	assert.True(t, found, "should detect ignored error return")
}

func TestScanPatterns_GoroutineStarted(t *testing.T) {
	files := []DiffFile{
		{
			Path:     "worker.go",
			Language: "go",
			Hunks: []DiffHunk{
				{
					Lines: []DiffLine{
						{Kind: LineAdded, Content: `	go func() { process(data) }()`, NewLine: 20},
					},
				},
			},
		},
	}
	hits := ScanPatterns(files, nil)

	var found bool
	for _, h := range hits {
		if h.Category == "concurrency" {
			found = true
		}
	}
	assert.True(t, found, "should flag goroutine start")
}

func TestScanPatterns_SecretLogged(t *testing.T) {
	files := []DiffFile{
		{
			Path:     "auth.go",
			Language: "go",
			Hunks: []DiffHunk{
				{
					Lines: []DiffLine{
						{Kind: LineAdded, Content: `	log.Printf("user token: %s", token)`, NewLine: 30},
					},
				},
			},
		},
	}
	hits := ScanPatterns(files, nil)

	var found bool
	for _, h := range hits {
		if h.Category == "security" && h.Rule == "secret or credential may be logged" {
			found = true
		}
	}
	assert.True(t, found, "should detect secret in log")
}

func TestScanPatterns_TypeAssertionWithoutOk(t *testing.T) {
	files := []DiffFile{
		{
			Path:     "handler.go",
			Language: "go",
			Hunks: []DiffHunk{
				{
					Lines: []DiffLine{
						{Kind: LineAdded, Content: `	val := x.(string)`, NewLine: 8},
					},
				},
			},
		},
	}
	hits := ScanPatterns(files, nil)

	var found bool
	for _, h := range hits {
		if h.Category == "type_errors" {
			found = true
		}
	}
	assert.True(t, found, "should detect type assertion without ok-check")
}

func TestScanPatterns_SkipsRemovedLines(t *testing.T) {
	files := []DiffFile{
		{
			Path:     "old.go",
			Language: "go",
			Hunks: []DiffHunk{
				{
					Lines: []DiffLine{
						{Kind: LineRemoved, Content: `	query := "SELECT * FROM users WHERE id=" + userID`, OldLine: 10},
					},
				},
			},
		},
	}
	hits := ScanPatterns(files, nil)
	assert.Empty(t, hits, "should not flag removed lines")
}

func TestScanPatterns_SkipsWrongLanguage(t *testing.T) {
	files := []DiffFile{
		{
			Path:     "app.py",
			Language: "python",
			Hunks: []DiffHunk{
				{
					Lines: []DiffLine{
						// Go-specific pattern should not match in Python
						{Kind: LineAdded, Content: `	go func() { process(data) }()`, NewLine: 5},
					},
				},
			},
		},
	}
	hits := ScanPatterns(files, nil)

	for _, h := range hits {
		if h.Category == "concurrency" && h.Rule == "goroutine started — verify variable initialisation" {
			t.Error("should not apply Go-specific rule to Python file")
		}
	}
}

func TestScanPatterns_WithASTContext(t *testing.T) {
	files := []DiffFile{
		{
			Path:     "auth.go",
			Language: "go",
			Hunks: []DiffHunk{
				{
					Lines: []DiffLine{
						{Kind: LineAdded, Content: `	log.Printf("password: %s", password)`, NewLine: 50},
					},
				},
			},
		},
	}
	asts := map[string]*ASTContext{
		"auth.go": {
			Symbols: []ASTSymbol{
				{Name: "HandleLogin", StartLine: 40, EndLine: 60, Changed: true},
			},
		},
	}
	hits := ScanPatterns(files, asts)
	require.NotEmpty(t, hits)
	var foundSecurity bool
	for _, h := range hits {
		if h.Category == "security" {
			foundSecurity = true
		}
	}
	assert.True(t, foundSecurity, "should find security pattern with AST context")
}

func TestDeduplicateHits(t *testing.T) {
	hits := []PatternHit{
		{Category: "security", File: "a.go", Line: 10, Rule: "rule1"},
		{Category: "security", File: "a.go", Line: 10, Rule: "rule2"}, // same file+line+category
		{Category: "security", File: "a.go", Line: 11, Rule: "rule1"}, // different line
	}
	result := DeduplicateHits(hits)
	assert.Len(t, result, 2) // deduped by file+line+category
}

func TestScanPatterns_EmptyDiff(t *testing.T) {
	hits := ScanPatterns(nil, nil)
	assert.Empty(t, hits)
}
