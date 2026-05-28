// Package static provides deterministic checks for agent specification files.
// Checks self-register via init() and produce llmutil.Finding objects identical
// to LLM output, enabling seamless merging of static and LLM results.
package static

import (
	"strings"

	"github.com/mirandaguillaume/reify/internal/doctor/llmutil"
	"github.com/mirandaguillaume/reify/internal/doctor/parser"
)

// Check is the interface for static analysis checks on agent files.
type Check interface {
	ID() string
	Tags() []string
	Category() string
	DefaultSeverity() string
	Run(analysis *parser.AgentAnalysis) []llmutil.Finding
}

var checks []Check

// RegisterCheck adds a check to the global registry. Called from init().
func RegisterCheck(c Check) {
	checks = append(checks, c)
}

// AllChecks returns all registered checks.
func AllChecks() []Check {
	return checks
}

// ResolveChecks returns checks matching the given mode.
// Modes: "default" (default-tagged), "thorough" (all), "security" (security-tagged), "quick" (default-tagged).
func ResolveChecks(mode string) []Check {
	var resolved []Check
	for _, c := range checks {
		if matchesMode(c.Tags(), mode) {
			resolved = append(resolved, c)
		}
	}
	return resolved
}

func matchesMode(tags []string, mode string) bool {
	for _, t := range tags {
		switch mode {
		case "default", "quick":
			if t == "default" {
				return true
			}
		case "thorough":
			return true // all checks run in thorough mode
		case "security":
			if t == "security" {
				return true
			}
		}
	}
	return false
}

// RunChecks executes all checks matching the mode against the analysis.
// Static check findings get Severity populated from DefaultSeverity().
func RunChecks(analysis *parser.AgentAnalysis, mode string) []llmutil.Finding {
	resolved := ResolveChecks(mode)
	var findings []llmutil.Finding
	for _, c := range resolved {
		checkFindings := c.Run(analysis)
		sev := c.DefaultSeverity()
		for i := range checkFindings {
			if checkFindings[i].Severity == "" {
				checkFindings[i].Severity = sev
			}
		}
		findings = append(findings, checkFindings...)
	}
	return findings
}

// NormalizeContent returns content with CRLF normalized to LF.
func NormalizeContent(content string) string {
	return strings.ReplaceAll(content, "\r\n", "\n")
}

// IsCodeFenceLine returns true if the trimmed line starts a code fence
// (triple backtick ``` or tilde ~~~).
func IsCodeFenceLine(trimmed string) bool {
	return strings.HasPrefix(trimmed, "```") || strings.HasPrefix(trimmed, "~~~")
}

// wordPunctuation are the characters trimmed from a token before whole-word
// comparison. Matches the set used historically by smells.go's local helper.
const wordPunctuation = ".,;:!?\"'()[]`"

// countWholeWord returns the number of times word appears in text as a complete
// word, where "word" is a token from strings.Fields(text) with leading and
// trailing punctuation trimmed via wordPunctuation. The match is case-sensitive
// — callers should normalize case before calling.
//
// Multi-word phrases (containing a space) are NOT supported and return 0.
// For phrase matching, use strings.Count(text, phrase) directly.
//
// Replaces ad-hoc strings.Count substring matching that produced false positives
// when target words appeared inside larger words (e.g., "do" matching "doing").
// Carried from Epic 7 retrospective action #2.
func countWholeWord(text, word string) int {
	if word == "" || text == "" {
		return 0
	}
	if strings.Contains(word, " ") {
		return 0
	}
	count := 0
	for _, token := range strings.Fields(text) {
		if strings.Trim(token, wordPunctuation) == word {
			count++
		}
	}
	return count
}

// countWholeWords returns the sum of countWholeWord for each target word in
// words. Convenience for the common case of counting several single-word
// targets in the same text.
func countWholeWords(text string, words []string) int {
	total := 0
	for _, w := range words {
		total += countWholeWord(text, w)
	}
	return total
}

// Indicator is a continuous metric displayed to the user but not counted in structural score.
type Indicator struct {
	Name     string  // e.g., "Readability"
	Value    float64 // e.g., 22.4
	Unit     string  // e.g., "Flesch RE"
	Guidance string  // e.g., "difficult — target >30"
}

// IndicatorCheck extends Check with indicator output.
type IndicatorCheck interface {
	Check
	Indicators(analysis *parser.AgentAnalysis) []Indicator
}

// CollectIndicators runs all indicator-capable checks and returns their metrics.
func CollectIndicators(analysis *parser.AgentAnalysis, mode string) []Indicator {
	resolved := ResolveChecks(mode)
	var indicators []Indicator
	for _, c := range resolved {
		if ic, ok := c.(IndicatorCheck); ok {
			indicators = append(indicators, ic.Indicators(analysis)...)
		}
	}
	return indicators
}

// ResetChecks clears the registry. Used only in tests.
func ResetChecks() {
	checks = nil
}
