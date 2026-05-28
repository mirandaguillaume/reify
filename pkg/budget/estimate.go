package budget

// EstimateTokens estimates the number of tokens in a string using the char/4
// approximation. Consistent with the project convention used in
// internal/bench/experiment/ (text_runner.go, runtime_runner.go).
func EstimateTokens(s string) int {
	return len(s) / 4
}

// TruncateToTokens truncates a string to approximately maxTokens tokens.
// Uses rune-aware truncation to avoid splitting multi-byte UTF-8 sequences.
// Returns the truncated string and true if truncation occurred.
func TruncateToTokens(s string, maxTokens int) (string, bool) {
	maxChars := maxTokens * 4
	if len(s) <= maxChars {
		return s, false
	}
	// Truncate by rune to preserve valid UTF-8
	runes := []rune(s)
	if len(runes) <= maxChars {
		return s, false
	}
	return string(runes[:maxChars]), true
}
