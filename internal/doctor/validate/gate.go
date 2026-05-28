// Package validate implements the quality gate for doctor --fix rewrites.
// The gate runs 4 deterministic checks plus an optional diff-size sanity check.
// All checks are fail-closed: any failure rejects the rewrite.
package validate

import (
	"bytes"
	"fmt"
	"strings"

	"gopkg.in/yaml.v3"
)

// GateFailure describes a single quality gate check failure.
type GateFailure struct {
	Check  string // check identifier (e.g. "frontmatter_valid", "field_preservation")
	Detail string // human-readable failure description
}

// GateResult holds the outcome of quality gate validation.
type GateResult struct {
	Passed   bool
	Failures []GateFailure
}

// GateOptions configures quality gate behavior.
type GateOptions struct {
	Force bool // if true, diff_size check produces a warning, not a failure
}

// Validate runs all quality gate checks on original vs rewritten content.
// Returns a GateResult with all failures collected (checks run independently).
func Validate(original, rewritten []byte, opts GateOptions) *GateResult {
	var failures []GateFailure

	// Normalize for comparison
	origNorm := normalizeLineEndings(original)
	rewriteNorm := normalizeLineEndings(rewritten)

	// Check 1: frontmatter validity
	failures = append(failures, checkFrontmatterValid(rewriteNorm)...)

	// Check 2: field preservation
	failures = append(failures, checkFieldPreservation(origNorm, rewriteNorm)...)

	// Check 3: markdown structure
	failures = append(failures, checkMarkdownStructure(rewriteNorm)...)

	// Check 4: section preservation
	failures = append(failures, checkSectionPreservation(origNorm, rewriteNorm)...)

	// Check 5: diff-size sanity
	if !opts.Force {
		failures = append(failures, checkDiffSize(origNorm, rewriteNorm)...)
	}

	return &GateResult{
		Passed:   len(failures) == 0,
		Failures: failures,
	}
}

// DiffRatio computes the proportion of changed lines between original and rewritten.
func DiffRatio(original, rewritten []byte) float64 {
	origLines := strings.Split(string(normalizeLineEndings(original)), "\n")
	rewriteLines := strings.Split(string(normalizeLineEndings(rewritten)), "\n")

	maxLen := len(origLines)
	if len(rewriteLines) > maxLen {
		maxLen = len(rewriteLines)
	}
	if maxLen == 0 {
		return 0
	}

	changed := 0
	for i := 0; i < maxLen; i++ {
		if i >= len(origLines) || i >= len(rewriteLines) || origLines[i] != rewriteLines[i] {
			changed++
		}
	}
	return float64(changed) / float64(maxLen)
}

func normalizeLineEndings(content []byte) []byte {
	return bytes.ReplaceAll(content, []byte("\r\n"), []byte("\n"))
}

// --- Check 1: Frontmatter validity ---

func checkFrontmatterValid(rewritten []byte) []GateFailure {
	fm := extractYAMLFrontmatter(rewritten)
	if fm == "" {
		return nil // no frontmatter is valid (plain markdown)
	}

	var parsed map[string]interface{}
	if err := yaml.Unmarshal([]byte(fm), &parsed); err != nil {
		return []GateFailure{{
			Check:  "frontmatter_valid",
			Detail: fmt.Sprintf("frontmatter is not valid YAML: %v", err),
		}}
	}
	return nil
}

// --- Check 2: Field preservation ---

func checkFieldPreservation(original, rewritten []byte) []GateFailure {
	origFM := parseFrontmatter(original)
	if origFM == nil {
		return nil // no original frontmatter to check
	}

	rewriteFM := parseFrontmatter(rewritten)
	if rewriteFM == nil && len(origFM) > 0 {
		return []GateFailure{{
			Check:  "field_preservation",
			Detail: "rewritten file lost all frontmatter",
		}}
	}

	var failures []GateFailure
	for key := range origFM {
		if _, ok := rewriteFM[key]; !ok {
			failures = append(failures, GateFailure{
				Check:  "field_preservation",
				Detail: fmt.Sprintf("lost frontmatter field: %s", key),
			})
		}
	}
	return failures
}

// --- Check 3: Markdown structure ---

func checkMarkdownStructure(rewritten []byte) []GateFailure {
	var failures []GateFailure
	content := string(rewritten)

	// Skip frontmatter for markdown checks
	body := bodyAfterFrontmatter(content)

	// Check code fence balance
	fenceCount := strings.Count(body, "```")
	if fenceCount%2 != 0 {
		failures = append(failures, GateFailure{
			Check:  "markdown_structure",
			Detail: fmt.Sprintf("unbalanced code fences: %d ``` markers (expected even number)", fenceCount),
		})
	}

	// Check headers are well-formed (# must be followed by space)
	lines := strings.Split(body, "\n")
	inCodeFence := false
	for i, line := range lines {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "```") {
			inCodeFence = !inCodeFence
		}
		if !inCodeFence && strings.HasPrefix(trimmed, "#") {
			// Count # characters
			level := 0
			for _, c := range trimmed {
				if c == '#' {
					level++
				} else {
					break
				}
			}
			// Must be followed by space or end of line
			rest := trimmed[level:]
			if rest != "" && !strings.HasPrefix(rest, " ") {
				failures = append(failures, GateFailure{
					Check:  "markdown_structure",
					Detail: fmt.Sprintf("malformed header at line %d: missing space after #", i+1),
				})
			}
		}
	}

	return failures
}

// --- Check 4: Section preservation ---

func checkSectionPreservation(original, rewritten []byte) []GateFailure {
	origSections := extractSectionHeaders(string(original))
	if len(origSections) == 0 {
		return nil
	}

	rewriteSections := extractSectionHeaders(string(rewritten))
	rewriteSet := make(map[string]bool, len(rewriteSections))
	for _, h := range rewriteSections {
		rewriteSet[h] = true
	}

	var failures []GateFailure
	for _, h := range origSections {
		if !rewriteSet[h] {
			failures = append(failures, GateFailure{
				Check:  "section_preservation",
				Detail: fmt.Sprintf("lost section: %s", h),
			})
		}
	}
	return failures
}

// --- Check 5: Diff-size sanity ---

func checkDiffSize(original, rewritten []byte) []GateFailure {
	ratio := DiffRatio(original, rewritten)
	if ratio > 0.5 {
		return []GateFailure{{
			Check:  "diff_size",
			Detail: fmt.Sprintf("rewrite changes %.0f%% of lines (threshold: 50%%). Use --force to override", ratio*100),
		}}
	}
	return nil
}

// --- Helpers ---

func extractYAMLFrontmatter(content []byte) string {
	s := strings.TrimSpace(string(content))
	if !strings.HasPrefix(s, "---") {
		return ""
	}
	rest := s[3:]
	idx := strings.Index(rest, "\n---")
	if idx < 0 {
		return ""
	}
	return rest[:idx]
}

func parseFrontmatter(content []byte) map[string]interface{} {
	fm := extractYAMLFrontmatter(content)
	if fm == "" {
		return nil
	}
	var parsed map[string]interface{}
	if err := yaml.Unmarshal([]byte(fm), &parsed); err != nil {
		// Try tolerant parsing — just extract top-level keys
		parsed = make(map[string]interface{})
		for _, line := range strings.Split(fm, "\n") {
			line = strings.TrimSpace(line)
			if idx := strings.Index(line, ":"); idx > 0 {
				key := strings.TrimSpace(line[:idx])
				if key != "" && !strings.HasPrefix(key, "-") && !strings.HasPrefix(key, " ") {
					parsed[key] = strings.TrimSpace(line[idx+1:])
				}
			}
		}
	}
	return parsed
}

func bodyAfterFrontmatter(content string) string {
	s := strings.TrimSpace(content)
	if !strings.HasPrefix(s, "---") {
		return content
	}
	rest := s[3:]
	idx := strings.Index(rest, "\n---")
	if idx < 0 {
		return content
	}
	return rest[idx+4:]
}

func extractSectionHeaders(content string) []string {
	body := bodyAfterFrontmatter(content)
	lines := strings.Split(body, "\n")
	inCodeFence := false
	var headers []string

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "```") {
			inCodeFence = !inCodeFence
		}
		if !inCodeFence && strings.HasPrefix(trimmed, "#") {
			level := 0
			for _, c := range trimmed {
				if c == '#' {
					level++
				} else {
					break
				}
			}
			header := strings.TrimSpace(trimmed[level:])
			if header != "" {
				headers = append(headers, header)
			}
		}
	}
	return headers
}
