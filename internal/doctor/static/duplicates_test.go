package static

import (
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/mirandaguillaume/reify/internal/doctor/parser"
	"github.com/stretchr/testify/assert"
)

func TestDuplicates_DetectsNearDuplicates(t *testing.T) {
	content := `## Rules
- Never expose API keys or secrets in the output
- Always validate inputs before processing
- Do not expose secrets or API keys in responses
- Always validate user inputs before processing them`

	analysis := &parser.AgentAnalysis{RawContent: []byte(content)}
	check := &duplicatesCheck{}
	findings := check.Run(analysis)
	assert.True(t, len(findings) >= 1, "expected duplicate detection, got %d findings", len(findings))
}

func TestDuplicates_NoDuplicates(t *testing.T) {
	content := `## Rules
- Always run tests before committing
- Use gofmt for formatting
- Never skip the linter on pull requests`

	analysis := &parser.AgentAnalysis{RawContent: []byte(content)}
	check := &duplicatesCheck{}
	findings := check.Run(analysis)
	assert.Empty(t, findings)
}

func TestDuplicates_SkipsCodeBlocks(t *testing.T) {
	content := "## Rules\n- Always run tests\n```\n- Always run tests\n```\n"
	analysis := &parser.AgentAnalysis{RawContent: []byte(content)}
	check := &duplicatesCheck{}
	findings := check.Run(analysis)
	assert.Empty(t, findings, "duplicate inside code block should not match")
}

func TestDuplicates_NilAnalysis(t *testing.T) {
	check := &duplicatesCheck{}
	assert.Nil(t, check.Run(nil))
}

func TestJaccard(t *testing.T) {
	a := map[string]bool{"never": true, "expose": true, "api": true, "keys": true, "secrets": true}
	b := map[string]bool{"not": true, "expose": true, "secrets": true, "api": true, "keys": true}
	sim := jaccard(a, b)
	// intersection: expose, api, keys, secrets = 4
	// union: never, expose, api, keys, secrets, not = 6
	assert.InDelta(t, 4.0/6.0, sim, 0.01)
}

// TestTruncate_ASCII verifies the basic case is unaffected by the rune-safe rewrite.
func TestTruncate_ASCII(t *testing.T) {
	assert.Equal(t, "hello", truncate("hello", 10))
	assert.Equal(t, "he...", truncate("hello world", 5))
}

// TestTruncate_Multibyte verifies AC #7 (Story 4-0) — truncate must operate on
// runes, not bytes. Cutting a multibyte UTF-8 sequence in the middle produces
// invalid UTF-8 strings, which corrupt downstream processing and JSON output.
func TestTruncate_Multibyte(t *testing.T) {
	// Each emoji is 4 bytes but 1 rune
	emojiString := "🎉🎉🎉🎉🎉🎉🎉🎉🎉🎉" // 10 runes, 40 bytes
	result := truncate(emojiString, 5)
	// Result must be valid UTF-8 (no broken multibyte sequences)
	assert.True(t, utf8.ValidString(result), "truncate must produce valid UTF-8")
	// Result must be ≤ 5 runes (excluding the "..." suffix counted separately)
	// We expect: 2 emojis + "..." = 5 runes total
	assert.LessOrEqual(t, utf8.RuneCountInString(result), 5)
	assert.True(t, strings.HasSuffix(result, "..."))
}

// TestTruncate_FrenchAccents verifies the rune-safe truncate handles common
// 2-byte characters (é, è, à, etc.) without slicing them in half.
func TestTruncate_FrenchAccents(t *testing.T) {
	// "café résumé" — é is 2 bytes each, total 13 bytes, 11 runes
	result := truncate("café résumé", 8)
	assert.True(t, utf8.ValidString(result), "truncate must not split 2-byte characters")
	assert.LessOrEqual(t, utf8.RuneCountInString(result), 8)
}

// TestTruncate_ExactBoundary verifies no truncation happens when length equals max.
func TestTruncate_ExactBoundary(t *testing.T) {
	assert.Equal(t, "12345", truncate("12345", 5))
	// One rune over → truncate
	result := truncate("123456", 5)
	assert.True(t, strings.HasSuffix(result, "..."))
}

// TestTruncate_NegativeMaxLen verifies truncate does not panic on negative maxLen.
// A negative value should return an empty string rather than a slice-out-of-bounds panic.
// (Review patch: edge case guard)
func TestTruncate_NegativeMaxLen(t *testing.T) {
	assert.NotPanics(t, func() {
		result := truncate("hello world", -1)
		assert.Equal(t, "", result)
	})
}

func TestDuplicates_SkipsTildeFences(t *testing.T) {
	content := "## Rules\n- Always run tests before committing code changes\n~~~\n- Always run tests before committing code changes\n~~~\n"
	analysis := &parser.AgentAnalysis{RawContent: []byte(content)}
	check := &duplicatesCheck{}
	findings := check.Run(analysis)
	assert.Empty(t, findings, "duplicate inside tilde fence should not match")
}

func TestDuplicates_CappedAt50(t *testing.T) {
	// Generate 100+ duplicate instruction pairs
	var lines []string
	lines = append(lines, "## Rules")
	for i := 0; i < 80; i++ {
		lines = append(lines, "- Always validate user inputs before processing them carefully and thoroughly in production")
	}
	content := strings.Join(lines, "\n")
	analysis := &parser.AgentAnalysis{RawContent: []byte(content)}
	check := &duplicatesCheck{}
	findings := check.Run(analysis)
	assert.LessOrEqual(t, len(findings), 50, "duplicates should be capped at 50")
}
