package static

import (
	"strings"
	"unicode"

	"github.com/mirandaguillaume/reify/internal/doctor/llmutil"
	"github.com/mirandaguillaume/reify/internal/doctor/parser"
)

func init() {
	RegisterCheck(&readabilityCheck{})
}

type readabilityCheck struct{}

func (r *readabilityCheck) ID() string              { return "readability" }
func (r *readabilityCheck) Tags() []string          { return []string{"thorough"} }
func (r *readabilityCheck) Category() string        { return "readability" }
func (r *readabilityCheck) DefaultSeverity() string { return "low" }

// Run returns no findings — readability is an indicator only.
func (r *readabilityCheck) Run(_ *parser.AgentAnalysis) []llmutil.Finding {
	return nil
}

// Indicators computes Flesch Reading Ease and Gunning Fog Index.
func (r *readabilityCheck) Indicators(analysis *parser.AgentAnalysis) []Indicator {
	if analysis == nil || len(analysis.RawContent) == 0 {
		return nil
	}

	prose := extractProse(string(analysis.RawContent))
	if len(prose) < 50 {
		return nil // too short for meaningful readability analysis
	}

	words := countWords(prose)
	sentences := countSentences(prose)
	syllables := countSyllables(prose)

	if words == 0 || sentences == 0 {
		return nil
	}

	flesch := 206.835 - 1.015*float64(words)/float64(sentences) - 84.6*float64(syllables)/float64(words)
	fog := 0.4 * (float64(words)/float64(sentences) + 100.0*float64(countComplexWords(prose))/float64(words))

	var indicators []Indicator

	fleschGuidance := "very difficult"
	if flesch > 60 {
		fleschGuidance = "standard"
	} else if flesch > 30 {
		fleschGuidance = "difficult"
	}
	indicators = append(indicators, Indicator{
		Name:     "Readability",
		Value:    flesch,
		Unit:     "Flesch RE",
		Guidance: fleschGuidance + " — target >30",
	})

	fogGuidance := "clear"
	if fog > 15 {
		fogGuidance = "overwritten"
	} else if fog > 12 {
		fogGuidance = "complex"
	}
	indicators = append(indicators, Indicator{
		Name:     "Fog Index",
		Value:    fog,
		Unit:     "Gunning Fog",
		Guidance: fogGuidance + " — target <15",
	})

	return indicators
}

// extractProse removes code blocks and returns only prose text.
func extractProse(content string) string {
	var prose strings.Builder
	inCodeFence := false
	for _, line := range strings.Split(NormalizeContent(content), "\n") {
		trimmed := strings.TrimSpace(line)
		if IsCodeFenceLine(trimmed) {
			inCodeFence = !inCodeFence
			continue
		}
		if inCodeFence {
			continue
		}
		// Skip markdown headers (just the marker)
		if strings.HasPrefix(trimmed, "#") {
			// Keep the text after #
			idx := strings.IndexByte(trimmed, ' ')
			if idx > 0 {
				prose.WriteString(trimmed[idx+1:])
				prose.WriteByte('\n')
			}
			continue
		}
		prose.WriteString(line)
		prose.WriteByte('\n')
	}
	return prose.String()
}

func countWords(text string) int {
	return len(strings.Fields(text))
}

func countSentences(text string) int {
	count := 0
	for i, r := range text {
		if r == '.' || r == '!' || r == '?' {
			// Check if followed by space, newline, or end
			if i+1 >= len(text) || text[i+1] == ' ' || text[i+1] == '\n' {
				count++
			}
		}
	}
	if count == 0 {
		return 1 // at least one sentence
	}
	return count
}

func countSyllables(text string) int {
	total := 0
	for _, word := range strings.Fields(text) {
		total += syllablesInWord(strings.ToLower(word))
	}
	return total
}

func syllablesInWord(word string) int {
	// Strip non-alpha
	cleaned := strings.Map(func(r rune) rune {
		if unicode.IsLetter(r) {
			return r
		}
		return -1
	}, word)

	if len(cleaned) <= 2 {
		return 1
	}

	count := 0
	prevVowel := false
	for _, r := range cleaned {
		isVowel := strings.ContainsRune("aeiouy", r)
		if isVowel && !prevVowel {
			count++
		}
		prevVowel = isVowel
	}

	// Silent-e correction — but not for -le endings (e.g., "simple", "table")
	if strings.HasSuffix(cleaned, "e") && !strings.HasSuffix(cleaned, "le") && count > 1 {
		count--
	}

	if count == 0 {
		return 1
	}
	return count
}

// countComplexWords counts words with 3+ syllables (for Gunning Fog).
func countComplexWords(text string) int {
	count := 0
	for _, word := range strings.Fields(text) {
		if syllablesInWord(strings.ToLower(word)) >= 3 {
			count++
		}
	}
	return count
}
