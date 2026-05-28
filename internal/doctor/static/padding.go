package static

import (
	"fmt"
	"strings"

	"github.com/mirandaguillaume/reify/internal/doctor/llmutil"
	"github.com/mirandaguillaume/reify/internal/doctor/parser"
)

func init() {
	RegisterCheck(&paddingCheck{})
}

var paddingPatterns = []string{
	"make sure to",
	"it is important to",
	"please note that",
	"keep in mind that",
	"it's worth noting",
	"be sure to",
	"don't forget to",
	"remember to",
	"you should always",
	"it is essential to",
	"it is worth mentioning",
	"it should be noted",
}

type paddingCheck struct{}

func (p *paddingCheck) ID() string              { return "compressible-padding" }
func (p *paddingCheck) Tags() []string          { return []string{"thorough"} }
func (p *paddingCheck) Category() string        { return "redundancy" }
func (p *paddingCheck) DefaultSeverity() string { return "low" }

func (p *paddingCheck) Run(analysis *parser.AgentAnalysis) []llmutil.Finding {
	if analysis == nil || len(analysis.RawContent) == 0 {
		return nil
	}

	lines := strings.Split(NormalizeContent(string(analysis.RawContent)), "\n")
	inCodeFence := false
	var findings []llmutil.Finding

	for lineNum, line := range lines {
		trimmed := strings.TrimSpace(line)
		if IsCodeFenceLine(trimmed) {
			inCodeFence = !inCodeFence
			continue
		}
		if inCodeFence {
			continue
		}

		lower := strings.ToLower(line)
		for _, pattern := range paddingPatterns {
			if strings.Contains(lower, pattern) {
				findings = append(findings, llmutil.Finding{
					Category:             "redundancy",
					Issue:                fmt.Sprintf("Compressible filler: %q on line %d", pattern, lineNum+1),
					Confidence:           "low",
					CitationID:           "redundancy",
					CurrentState:         fmt.Sprintf("Line %d: %s", lineNum+1, strings.TrimSpace(line)),
					SuggestedImprovement: fmt.Sprintf("Remove %q — state the instruction directly", pattern),
				})
				break
			}
		}
	}

	return findings
}
