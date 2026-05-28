package analysis

import (
	"regexp"
	"strings"
)

// PatternHit represents a suspicious code pattern found by heuristic scanning.
type PatternHit struct {
	Category string // "null_safety", "concurrency", etc.
	Rule     string // specific rule text
	File     string
	Line     int
	Snippet  string // the suspicious code line
	Severity string // "high", "medium", "low"
}

// patternRule defines a single heuristic pattern to check against diff lines.
type patternRule struct {
	Category string
	Rule     string
	Severity string
	Regex    *regexp.Regexp
	// LangFilter restricts matching to specific languages ("" matches all).
	LangFilter string
}

// rules is the compiled set of forensic heuristic patterns.
var rules = []patternRule{
	// null_safety
	{
		Category: "null_safety",
		Rule:     "field access on value that can be nil",
		Severity: "high",
		Regex:    regexp.MustCompile(`\.\w+\s*[.(]`),
	},
	{
		Category:   "null_safety",
		Rule:       "missing nil check before dereference",
		Severity:   "high",
		Regex:      regexp.MustCompile(`(?:^|[^!]=\s*)\w+\.\w+`),
		LangFilter: "go",
	},

	// type_errors
	{
		Category:   "type_errors",
		Rule:       "type assertion without ok-check",
		Severity:   "high",
		Regex:      regexp.MustCompile(`\.\([\w.*]+\)(?:\s*$|[^,])`),
		LangFilter: "go",
	},

	// logic_bugs
	{
		Category: "logic_bugs",
		Rule:     "inverted boolean condition (negation)",
		Severity: "medium",
		Regex:    regexp.MustCompile(`!\s*\w+\s*[!=]=`),
	},

	// concurrency
	{
		Category:   "concurrency",
		Rule:       "goroutine started — verify variable initialisation",
		Severity:   "medium",
		Regex:      regexp.MustCompile(`\bgo\s+func\s*\(`),
		LangFilter: "go",
	},
	{
		Category:   "concurrency",
		Rule:       "shared state without sync — map access in goroutine",
		Severity:   "high",
		Regex:      regexp.MustCompile(`\bgo\s+func.*\bmap\b`),
		LangFilter: "go",
	},

	// resource_handling
	{
		Category:   "resource_handling",
		Rule:       "error return ignored",
		Severity:   "medium",
		Regex:      regexp.MustCompile(`^\s*\w+[\w.]*\(.*\)\s*$`),
		LangFilter: "go",
	},
	{
		Category:   "resource_handling",
		Rule:       "error return ignored (underscore discard)",
		Severity:   "medium",
		Regex:      regexp.MustCompile(`\b_\s*,?\s*(?::?=)\s*\w+\(`),
		LangFilter: "go",
	},

	// security
	{
		Category: "security",
		Rule:     "possible SQL injection — string concatenation in query",
		Severity: "high",
		Regex:    regexp.MustCompile(`(?i)(?:SELECT|INSERT|UPDATE|DELETE|WHERE)\s.*[+]`),
	},
	{
		Category: "security",
		Rule:     "possible SQL injection — format string in query",
		Severity: "high",
		Regex:    regexp.MustCompile(`(?i)(?:SELECT|INSERT|UPDATE|DELETE|WHERE).*(?:fmt\.Sprintf|%s|%v|f")`),
	},
	{
		Category: "security",
		Rule:     "secret or credential may be logged",
		Severity: "high",
		Regex:    regexp.MustCompile(`(?i)(?:log|print|fmt\.Print).*(?:password|secret|token|key|credential|api.?key)`),
	},
}

// ScanPatterns applies forensic heuristic rules to added/modified lines in a diff,
// using AST context to enrich results. Only added lines are scanned (removed lines
// are no longer in the codebase).
func ScanPatterns(files []DiffFile, asts map[string]*ASTContext) []PatternHit {
	var hits []PatternHit

	for _, f := range files {
		ast := asts[f.Path]

		for _, h := range f.Hunks {
			for _, line := range h.Lines {
				// Only scan added lines — these are the new code.
				if line.Kind != LineAdded {
					continue
				}

				for _, rule := range rules {
					if rule.LangFilter != "" && rule.LangFilter != f.Language {
						continue
					}
					if rule.Regex.MatchString(line.Content) {
						hit := PatternHit{
							Category: rule.Category,
							Rule:     rule.Rule,
							File:     f.Path,
							Line:     line.NewLine,
							Snippet:  strings.TrimSpace(line.Content),
							Severity: rule.Severity,
						}

						// Boost severity if inside a changed function
						if ast != nil {
							for _, sym := range ast.Symbols {
								if sym.Changed && line.NewLine >= sym.StartLine && line.NewLine <= sym.EndLine {
									// Confirmed inside a changed function — keep severity
									break
								}
							}
						}

						hits = append(hits, hit)
					}
				}
			}
		}
	}

	return hits
}

// DeduplicateHits removes duplicate pattern hits on the same file+line+category.
func DeduplicateHits(hits []PatternHit) []PatternHit {
	type key struct {
		File     string
		Line     int
		Category string
	}
	seen := make(map[key]bool)
	var result []PatternHit

	for _, h := range hits {
		k := key{File: h.File, Line: h.Line, Category: h.Category}
		if !seen[k] {
			seen[k] = true
			result = append(result, h)
		}
	}
	return result
}
