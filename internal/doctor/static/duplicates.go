package static

import (
	"fmt"
	"strings"

	"github.com/mirandaguillaume/reify/internal/doctor/llmutil"
	"github.com/mirandaguillaume/reify/internal/doctor/parser"
)

func init() {
	RegisterCheck(&duplicatesCheck{})
}

const jaccardThreshold = 0.5

type duplicatesCheck struct{}

func (d *duplicatesCheck) ID() string              { return "duplicate-detection" }
func (d *duplicatesCheck) Tags() []string          { return []string{"thorough"} }
func (d *duplicatesCheck) Category() string        { return "redundancy" }
func (d *duplicatesCheck) DefaultSeverity() string { return "moderate" }

type instructionLine struct {
	lineNum int
	text    string
	words   map[string]bool
}

func (d *duplicatesCheck) Run(analysis *parser.AgentAnalysis) []llmutil.Finding {
	if analysis == nil || len(analysis.RawContent) == 0 {
		return nil
	}

	instructions := extractInstructions(string(analysis.RawContent))
	if len(instructions) < 2 {
		return nil
	}

	var findings []llmutil.Finding
	seen := make(map[string]bool) // "lineA:lineB" to avoid duplicate pairs

	const maxFindings = 50

outer:
	for i := 0; i < len(instructions); i++ {
		for j := i + 1; j < len(instructions); j++ {
			if len(findings) >= maxFindings {
				break outer
			}
			sim := jaccard(instructions[i].words, instructions[j].words)
			if sim >= jaccardThreshold {
				key := fmt.Sprintf("%d:%d", instructions[i].lineNum, instructions[j].lineNum)
				if seen[key] {
					continue
				}
				seen[key] = true
				findings = append(findings, llmutil.Finding{
					Category:   "redundancy",
					Issue:      fmt.Sprintf("Near-duplicate instructions on lines %d and %d (%.0f%% similar)", instructions[i].lineNum, instructions[j].lineNum, sim*100),
					Confidence: "moderate",
					CitationID: "redundancy",
					CurrentState: fmt.Sprintf("Line %d: %s\nLine %d: %s",
						instructions[i].lineNum, truncate(instructions[i].text, 80),
						instructions[j].lineNum, truncate(instructions[j].text, 80)),
					SuggestedImprovement: "Merge or remove one of these duplicate instructions",
				})
			}
		}
	}

	return findings
}

func extractInstructions(content string) []instructionLine {
	lines := strings.Split(NormalizeContent(content), "\n")
	inCodeFence := false
	var instructions []instructionLine

	for lineNum, line := range lines {
		trimmed := strings.TrimSpace(line)
		if IsCodeFenceLine(trimmed) {
			inCodeFence = !inCodeFence
			continue
		}
		if inCodeFence {
			continue
		}

		// Extract instruction-like lines: bullets, numbered items, imperatives
		if strings.HasPrefix(trimmed, "- ") || strings.HasPrefix(trimmed, "* ") {
			trimmed = trimmed[2:]
		} else if len(trimmed) > 2 && trimmed[0] >= '0' && trimmed[0] <= '9' && strings.Contains(trimmed[:3], ".") {
			idx := strings.IndexByte(trimmed, '.')
			if idx > 0 && idx < 3 {
				trimmed = strings.TrimSpace(trimmed[idx+1:])
			}
		} else {
			continue // skip non-instruction lines
		}

		if len(trimmed) < 10 {
			continue // too short to be meaningful
		}

		words := tokenize(trimmed)
		if len(words) < 3 {
			continue
		}

		instructions = append(instructions, instructionLine{
			lineNum: lineNum + 1,
			text:    trimmed,
			words:   words,
		})
	}

	return instructions
}

func tokenize(text string) map[string]bool {
	words := make(map[string]bool)
	for _, w := range strings.Fields(strings.ToLower(text)) {
		clean := strings.Trim(w, ".,;:!?\"'()[]`")
		if len(clean) > 2 { // skip stopwords-like short words
			words[clean] = true
		}
	}
	return words
}

func jaccard(a, b map[string]bool) float64 {
	if len(a) == 0 && len(b) == 0 {
		return 0
	}
	intersection := 0
	for w := range a {
		if b[w] {
			intersection++
		}
	}
	union := len(a) + len(b) - intersection
	if union == 0 {
		return 0
	}
	return float64(intersection) / float64(union)
}

// truncate shortens s to maxLen runes (not bytes) followed by "..." when needed.
// Operating on runes prevents slicing multi-byte UTF-8 sequences in the middle,
// which would produce invalid UTF-8 output and corrupt JSON serialization.
// Story 4-0 AC #7.
func truncate(s string, maxLen int) string {
	if maxLen < 0 {
		return ""
	}
	runes := []rune(s)
	if len(runes) <= maxLen {
		return s
	}
	if maxLen <= 3 {
		return string(runes[:maxLen])
	}
	return string(runes[:maxLen-3]) + "..."
}
