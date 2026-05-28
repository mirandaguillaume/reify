package static

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/mirandaguillaume/reify/internal/doctor/llmutil"
	"github.com/mirandaguillaume/reify/internal/doctor/parser"
)

func init() {
	RegisterCheck(&driftCheck{})
}

// filePathRe matches common file path patterns in agent spec content.
var filePathRe = regexp.MustCompile(`(?:^|\s)((?:src|internal|pkg|lib|cmd|app|test|tests|docs|scripts|config)/[a-zA-Z0-9_\-./]+\.[a-zA-Z0-9]+)`)

// commandPatterns extract build/test/run commands.
var commandPatterns = []struct {
	name    string
	pattern *regexp.Regexp
	checker func(projectRoot, match string) bool
}{
	{"make", regexp.MustCompile(`\bmake\s+([a-zA-Z0-9_-]+)`), checkMakeTarget},
	{"npm", regexp.MustCompile(`\bnpm\s+(run\s+)?([a-zA-Z0-9_:-]+)`), nil}, // checked via package.json
	{"go test", regexp.MustCompile(`\bgo\s+test\s+(\./[a-zA-Z0-9_/.-]+)`), checkGoTestPath},
}

type driftCheck struct{}

func (d *driftCheck) ID() string              { return "codebase-drift" }
func (d *driftCheck) Tags() []string          { return []string{"default"} }
func (d *driftCheck) Category() string        { return "version_drift" }
func (d *driftCheck) DefaultSeverity() string { return "moderate" }

func (d *driftCheck) Run(analysis *parser.AgentAnalysis) []llmutil.Finding {
	if analysis == nil || len(analysis.RawContent) == 0 {
		return nil
	}

	// Need a project root to validate paths
	projectRoot := findProjectRoot()
	if projectRoot == "" {
		return nil
	}

	var findings []llmutil.Finding
	lines := strings.Split(NormalizeContent(string(analysis.RawContent)), "\n")
	inCodeFence := false

	for lineNum, line := range lines {
		trimmed := strings.TrimSpace(line)
		if IsCodeFenceLine(trimmed) {
			inCodeFence = !inCodeFence
			continue
		}

		// File path validation
		if matches := filePathRe.FindAllStringSubmatch(line, -1); len(matches) > 0 {
			for _, m := range matches {
				path := m[1]
				// Prevent path traversal outside project root
				fullPath := filepath.Join(projectRoot, path)
				cleanPath := filepath.Clean(fullPath)
				if !strings.HasPrefix(cleanPath, projectRoot) {
					continue // skip paths that escape project root
				}
				if _, err := os.Stat(cleanPath); os.IsNotExist(err) {
					// Skip if in code fence or preceded by example indicator
					if inCodeFence || isExampleContext(line) {
						continue
					}
					findings = append(findings, llmutil.Finding{
						Category:             "version_drift",
						Issue:                fmt.Sprintf("Referenced file not found: %s (line %d)", path, lineNum+1),
						Confidence:           "moderate",
						CitationID:           "version_drift",
						CurrentState:         fmt.Sprintf("Line %d references %s which does not exist", lineNum+1, path),
						SuggestedImprovement: "Update or remove this stale file reference",
					})
				}
			}
		}

		// Command validation (skip inside code fences for examples)
		if !inCodeFence {
			findings = append(findings, validateCommands(line, lineNum, projectRoot)...)
		}
	}

	return findings
}

func validateCommands(line string, lineNum int, projectRoot string) []llmutil.Finding {
	var findings []llmutil.Finding

	// make targets
	if matches := commandPatterns[0].pattern.FindStringSubmatch(line); len(matches) > 1 {
		target := matches[1]
		if !checkMakeTarget(projectRoot, target) {
			findings = append(findings, llmutil.Finding{
				Category:             "version_drift",
				Issue:                fmt.Sprintf("Make target %q not found in Makefile (line %d)", target, lineNum+1),
				Confidence:           "moderate",
				CitationID:           "version_drift",
				CurrentState:         fmt.Sprintf("Line %d: make %s", lineNum+1, target),
				SuggestedImprovement: "Update the command or add the target to Makefile",
			})
		}
	}

	// npm scripts
	if matches := commandPatterns[1].pattern.FindStringSubmatch(line); len(matches) > 2 {
		script := matches[2]
		if !checkNpmScript(projectRoot, script) {
			findings = append(findings, llmutil.Finding{
				Category:             "version_drift",
				Issue:                fmt.Sprintf("npm script %q not found in package.json (line %d)", script, lineNum+1),
				Confidence:           "moderate",
				CitationID:           "version_drift",
				CurrentState:         fmt.Sprintf("Line %d: npm %s", lineNum+1, script),
				SuggestedImprovement: "Update the command or add the script to package.json",
			})
		}
	}

	// go test paths
	if matches := commandPatterns[2].pattern.FindStringSubmatch(line); len(matches) > 1 {
		testPath := matches[1]
		if !checkGoTestPath(projectRoot, testPath) {
			findings = append(findings, llmutil.Finding{
				Category:             "version_drift",
				Issue:                fmt.Sprintf("go test path %q not found (line %d)", testPath, lineNum+1),
				Confidence:           "low",
				CitationID:           "version_drift",
				CurrentState:         fmt.Sprintf("Line %d: go test %s", lineNum+1, testPath),
				SuggestedImprovement: "Update the test path to match current directory structure",
			})
		}
	}

	return findings
}

var exampleIndicators = []string{"example", "e.g.", "for instance", "such as", "like:"}

func isExampleContext(line string) bool {
	lower := strings.ToLower(line)
	for _, indicator := range exampleIndicators {
		if strings.Contains(lower, indicator) {
			return true
		}
	}
	return false
}

func findProjectRoot() string {
	cwd, err := os.Getwd()
	if err != nil {
		return ""
	}
	markers := []string{".git", "go.mod", "package.json", "Makefile", "Cargo.toml"}
	dir := cwd
	for {
		for _, marker := range markers {
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

func checkMakeTarget(projectRoot, target string) bool {
	makefile := filepath.Join(projectRoot, "Makefile")
	data, err := os.ReadFile(makefile)
	if err != nil {
		return true // no Makefile — skip check
	}
	// Simple check: look for "target:" at start of line
	for _, line := range strings.Split(string(data), "\n") {
		if strings.HasPrefix(line, target+":") || strings.HasPrefix(line, ".PHONY:") && strings.Contains(line, target) {
			return true
		}
	}
	return false
}

func checkNpmScript(projectRoot, script string) bool {
	pkgFile := filepath.Join(projectRoot, "package.json")
	data, err := os.ReadFile(pkgFile)
	if err != nil {
		return true // no package.json — skip check
	}
	var pkg struct {
		Scripts map[string]string `json:"scripts"`
	}
	if err := json.Unmarshal(data, &pkg); err != nil {
		return true // malformed — skip
	}
	// npm run <script> or npm <builtin>
	if _, ok := pkg.Scripts[script]; ok {
		return true
	}
	// npm builtins
	builtins := map[string]bool{"test": true, "start": true, "build": true, "install": true, "publish": true}
	return builtins[script]
}

func checkGoTestPath(projectRoot, testPath string) bool {
	// Strip ./
	clean := strings.TrimPrefix(testPath, "./")
	// Strip /...
	clean = strings.TrimSuffix(clean, "/...")
	dir := filepath.Join(projectRoot, clean)
	info, err := os.Stat(dir)
	if err != nil {
		return false
	}
	return info.IsDir()
}
