// Package context provides codebase context enrichment for the doctor command.
// It scans the project around an agent file and produces gap-analysis findings
// by comparing agent content against the codebase structure.
package context

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/mirandaguillaume/reify/internal/doctor/llmutil"
	"github.com/mirandaguillaume/reify/internal/doctor/parser"
	"github.com/mirandaguillaume/reify/internal/scanner"
)

// projectMarkers are files/dirs that indicate a project root.
var projectMarkers = []string{".git", "go.mod", "package.json", "Makefile", "Cargo.toml", "pyproject.toml"}

// Enrich scans the codebase at root and produces context-aware findings by
// comparing the agent analysis against project structure, stack, and symbols.
// If root is empty or scanning fails, it returns nil findings gracefully.
func Enrich(analysis *parser.AgentAnalysis, root string) ([]llmutil.Finding, error) {
	if root == "" {
		return nil, nil
	}

	ctx, err := scanner.ScanCodebase(root)
	if err != nil {
		return nil, nil // graceful degradation
	}

	content := normalizeContent(analysis)

	var findings []llmutil.Finding
	findings = append(findings, checkTestCoverage(ctx, content)...)
	findings = append(findings, checkStructureAwareness(ctx, content)...)
	findings = append(findings, checkKeySymbols(ctx, content)...)
	findings = append(findings, checkSecurityContext(ctx, content)...)
	findings = append(findings, checkStackAwareness(ctx, content)...)
	findings = append(findings, CheckTextOrdering(analysis)...)

	return findings, nil
}

// DetectProjectRoot walks upward from startDir looking for project markers.
// Returns "" if no project root is found.
func DetectProjectRoot(startDir string) string {
	dir := startDir
	for {
		for _, marker := range projectMarkers {
			if _, err := os.Stat(filepath.Join(dir, marker)); err == nil {
				return dir
			}
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return ""
		}
		dir = parent
	}
}

// normalizeContent returns the agent's raw content as a lowercase string
// with CRLF normalized to LF.
func normalizeContent(analysis *parser.AgentAnalysis) string {
	s := string(analysis.RawContent)
	s = strings.ReplaceAll(s, "\r\n", "\n")
	return strings.ToLower(s)
}

// checkTestCoverage checks if the agent mentions testing when the project has test files.
func checkTestCoverage(ctx *scanner.CodebaseContext, content string) []llmutil.Finding {
	testFiles := 0
	for _, dir := range ctx.Structure {
		for _, f := range dir.Files {
			if isTestFile(f) {
				testFiles++
			}
		}
	}
	if testFiles == 0 {
		return nil
	}

	testKeywords := []string{"test", "testing", "spec", "assert", "expect", "mock", "stub"}
	for _, kw := range testKeywords {
		if strings.Contains(content, kw) {
			return nil
		}
	}

	return []llmutil.Finding{{
		Category:             "context",
		Issue:                "Agent has no testing guidance",
		Confidence:           "high",
		CurrentState:         formatCount(testFiles, "test file"),
		SuggestedImprovement: "Add a testing section describing how to run tests, what to test, and testing conventions",
	}}
}

// checkStructureAwareness checks if the agent references any project directories.
func checkStructureAwareness(ctx *scanner.CodebaseContext, content string) []llmutil.Finding {
	if len(ctx.Structure) == 0 {
		return nil
	}

	referenced := 0
	for _, dir := range ctx.Structure {
		if dir.Path == "." || dir.Path == "" {
			continue
		}
		// Check each segment of the path (e.g., "internal/handler" matches on "internal" or "handler")
		for _, seg := range strings.Split(filepath.ToSlash(dir.Path), "/") {
			if seg == "" {
				continue
			}
			if strings.Contains(content, strings.ToLower(seg)) {
				referenced++
				break
			}
		}
	}

	if referenced > 0 {
		return nil
	}

	return []llmutil.Finding{{
		Category:             "context",
		Issue:                "Agent doesn't reference any project directories",
		Confidence:           "moderate",
		CurrentState:         formatCount(len(ctx.Structure), "project directory"),
		SuggestedImprovement: "Reference key project directories so the agent understands the codebase layout",
	}}
}

// checkKeySymbols checks if the agent references key exported types/interfaces.
func checkKeySymbols(ctx *scanner.CodebaseContext, content string) []llmutil.Finding {
	exported := 0
	interfaces := 0
	referenced := 0

	for _, sym := range ctx.Symbols {
		if !sym.Exported {
			continue
		}
		exported++
		if sym.Kind == "interface" {
			interfaces++
		}
		if strings.Contains(content, strings.ToLower(sym.Name)) {
			referenced++
		}
	}

	if exported == 0 || referenced > 0 {
		return nil
	}

	issue := "Agent doesn't reference any exported symbols from the codebase"
	if interfaces > 0 {
		issue = fmt.Sprintf("Codebase has %d exported interfaces — agent doesn't reference any", interfaces)
	}

	return []llmutil.Finding{{
		Category:             "context",
		Issue:                issue,
		Confidence:           "low",
		CurrentState:         formatCount(exported, "exported symbol"),
		SuggestedImprovement: "Reference key types and interfaces so the agent can provide contextual guidance",
	}}
}

// checkSecurityContext checks if the agent has security restrictions appropriate for the stack.
func checkSecurityContext(ctx *scanner.CodebaseContext, content string) []llmutil.Finding {
	hasNetworkDeps := false
	for _, dep := range ctx.Stack.Deps {
		name := strings.ToLower(dep.Name)
		if strings.Contains(name, "http") || strings.Contains(name, "net") ||
			strings.Contains(name, "grpc") || strings.Contains(name, "api") ||
			strings.Contains(name, "fetch") || strings.Contains(name, "axios") {
			hasNetworkDeps = true
			break
		}
	}

	if !hasNetworkDeps {
		return nil
	}

	securityKeywords := []string{"security", "permission", "restrict", "filesystem", "network", "sandbox"}
	for _, kw := range securityKeywords {
		if strings.Contains(content, kw) {
			return nil
		}
	}

	return []llmutil.Finding{{
		Category:             "context",
		Issue:                "Project uses network dependencies but agent has no security restrictions",
		Confidence:           "moderate",
		CurrentState:         "Project has network-related dependencies",
		SuggestedImprovement: "Add security constraints (filesystem access, network restrictions) appropriate for the project's stack",
	}}
}

// checkStackAwareness checks if the agent mentions the project's primary language/framework.
func checkStackAwareness(ctx *scanner.CodebaseContext, content string) []llmutil.Finding {
	if len(ctx.Stack.Languages) == 0 {
		return nil
	}

	primary := ctx.Stack.Languages[0]
	if primary.Percentage < 30 {
		return nil // no dominant language
	}

	langName := strings.ToLower(primary.Name)
	if strings.Contains(content, langName) {
		return nil
	}

	for _, alias := range langAliases(langName) {
		if strings.Contains(content, alias) {
			return nil
		}
	}

	return []llmutil.Finding{{
		Category:             "context",
		Issue:                fmt.Sprintf("Project is %.0f%% %s but agent doesn't mention %s conventions", primary.Percentage, primary.Name, primary.Name),
		Confidence:           "high",
		CurrentState:         fmt.Sprintf("Primary language: %s (%.0f%% of files)", primary.Name, primary.Percentage),
		SuggestedImprovement: fmt.Sprintf("Add %s-specific conventions, idioms, and best practices to the agent", primary.Name),
	}}
}

// orderingPatterns are text indicators of sequential ordering instructions.
var orderingPatterns = []string{
	"step 1", "step 2", "step 3",
	"phase 1", "phase 2",
	"### step", "## phase",
	"workflow:", "process:", "pipeline:",
}

// orderingWords are ordering transition words (checked as whole-word boundaries).
var orderingWords = []string{
	"first,", "first ", "then ", "then,",
	"next,", "next ", "after that", "finally,", "finally ",
}

// numberedStepRe matches lines like "1. Do something" (numbered lists indicating ordering).
var numberedStepPattern = []string{"1.", "2.", "3.", "4.", "5."}

// CheckTextOrdering detects if an agent relies on text-based ordering instructions
// without structural enforcement. Returns a finding if >= 3 indicators are found.
func CheckTextOrdering(analysis *parser.AgentAnalysis) []llmutil.Finding {
	content := strings.ToLower(strings.ReplaceAll(string(analysis.RawContent), "\r\n", "\n"))

	indicators := 0

	for _, pat := range orderingPatterns {
		if strings.Contains(content, pat) {
			indicators++
		}
	}

	for _, word := range orderingWords {
		if strings.Contains(content, word) {
			indicators++
		}
	}

	// Check for numbered steps (at least 3 consecutive numbers at line starts)
	numberedCount := 0
	for _, line := range strings.Split(content, "\n") {
		trimmed := strings.TrimSpace(line)
		for _, prefix := range numberedStepPattern {
			if strings.HasPrefix(trimmed, prefix) {
				numberedCount++
				break
			}
		}
	}
	if numberedCount >= 3 {
		indicators++
	}

	if indicators < 3 {
		return nil
	}

	return []llmutil.Finding{{
		Category:             "ordering",
		Issue:                "Agent relies on text instructions for execution ordering",
		Confidence:           "high",
		CitationID:           "ordering",
		CurrentState:         fmt.Sprintf("Found %d text-ordering indicators (numbered steps, sequence words, workflow markers)", indicators),
		SuggestedImprovement: "Text-based ordering instructions are unreliable. Research shows LLMs ignore instruction ordering. Consider Reify runtime for structural enforcement",
	}}
}

func isTestFile(name string) bool {
	if strings.HasSuffix(name, "_test.go") {
		return true
	}
	if strings.HasSuffix(name, ".test.js") || strings.HasSuffix(name, ".test.ts") ||
		strings.HasSuffix(name, ".test.jsx") || strings.HasSuffix(name, ".test.tsx") {
		return true
	}
	if strings.HasSuffix(name, "_test.py") || strings.HasSuffix(name, ".spec.js") ||
		strings.HasSuffix(name, ".spec.ts") {
		return true
	}
	return false
}

func langAliases(lang string) []string {
	switch lang {
	case "go":
		return []string{"golang"}
	case "javascript":
		return []string{"js", "node", "nodejs"}
	case "typescript":
		return []string{"ts", "node", "nodejs"}
	case "python":
		return []string{"py", "python3"}
	case "rust":
		return []string{"cargo", "rustc"}
	default:
		return nil
	}
}

func formatCount(n int, noun string) string {
	if n == 1 {
		return fmt.Sprintf("Project has 1 %s", noun)
	}
	return fmt.Sprintf("Project has %d %ss", n, noun)
}
