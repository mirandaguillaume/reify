package static

import (
	"fmt"
	"strings"

	"github.com/mirandaguillaume/reify/internal/doctor/llmutil"
	"github.com/mirandaguillaume/reify/internal/doctor/parser"
)

func init() {
	RegisterCheck(&vagueCheck{})
}

// vaguePatterns are phrases that indicate vague, non-actionable instructions.
var vaguePatterns = []string{
	"be thorough",
	"if needed",
	"when appropriate",
	"be helpful",
	"be careful",
	"as needed",
	"when possible",
	"if appropriate",
	"make sure",
	"try to",
	"be sure to",
	"keep in mind",
	"it is important",
	"please note",
	"do your best",
	"be mindful",
	"use your judgment",
}

type vagueCheck struct{}

func (v *vagueCheck) ID() string              { return "vague-patterns" }
func (v *vagueCheck) Tags() []string          { return []string{"default"} }
func (v *vagueCheck) Category() string        { return "specificity" }
func (v *vagueCheck) DefaultSeverity() string { return "moderate" }

func (v *vagueCheck) Run(analysis *parser.AgentAnalysis) []llmutil.Finding {
	if analysis == nil || len(analysis.RawContent) == 0 {
		return nil
	}

	lines := strings.Split(NormalizeContent(string(analysis.RawContent)), "\n")
	inCodeFence := false
	var findings []llmutil.Finding

	for lineNum, line := range lines {
		trimmed := strings.TrimSpace(line)

		// Track code fence state
		if IsCodeFenceLine(trimmed) {
			inCodeFence = !inCodeFence
			continue
		}
		if inCodeFence {
			continue
		}

		lower := strings.ToLower(line)
		for _, pattern := range vaguePatterns {
			if strings.Contains(lower, pattern) {
				findings = append(findings, llmutil.Finding{
					Category:             "specificity",
					Issue:                fmt.Sprintf("Vague instruction: %q on line %d", pattern, lineNum+1),
					Confidence:           "moderate",
					CitationID:           "specificity",
					CurrentState:         fmt.Sprintf("Line %d: %s", lineNum+1, strings.TrimSpace(line)),
					SuggestedImprovement: fmt.Sprintf("Replace %q with specific, measurable criteria", pattern),
				})
				break // one finding per line
			}
		}
	}

	return findings
}
