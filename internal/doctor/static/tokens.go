package static

import (
	"fmt"

	"github.com/mirandaguillaume/reify/internal/doctor/llmutil"
	"github.com/mirandaguillaume/reify/internal/doctor/parser"
)

func init() {
	RegisterCheck(&tokensCheck{})
}

const (
	tokenWarnTotal    = 8000
	tokenCritTotal    = 16000
	tokenWarnSection  = 2000
	tokenCorrectionFactor = 0.78 // chars/4 overestimates by ~22%
)

type tokensCheck struct{}

func (t *tokensCheck) ID() string              { return "token-budget" }
func (t *tokensCheck) Tags() []string          { return []string{"default"} }
func (t *tokensCheck) Category() string        { return "redundancy" }
func (t *tokensCheck) DefaultSeverity() string { return "moderate" }

func (t *tokensCheck) Run(analysis *parser.AgentAnalysis) []llmutil.Finding {
	if analysis == nil || len(analysis.RawContent) == 0 {
		return nil
	}

	totalTokens := estimateTokens(len(analysis.RawContent))
	var findings []llmutil.Finding

	if totalTokens > tokenCritTotal {
		findings = append(findings, llmutil.Finding{
			Category:             "redundancy",
			Issue:                fmt.Sprintf("File uses ~%d tokens (critical threshold: %d)", totalTokens, tokenCritTotal),
			Confidence:           "high",
			CitationID:           "redundancy",
			CurrentState:         fmt.Sprintf("Total: ~%d tokens across %d sections", totalTokens, len(analysis.Sections)),
			SuggestedImprovement: "Reduce file size — redundant context costs +20% tokens with no benefit (Gloaguen et al.)",
		})
	} else if totalTokens > tokenWarnTotal {
		findings = append(findings, llmutil.Finding{
			Category:             "redundancy",
			Issue:                fmt.Sprintf("File uses ~%d tokens (warning threshold: %d)", totalTokens, tokenWarnTotal),
			Confidence:           "moderate",
			CitationID:           "redundancy",
			CurrentState:         fmt.Sprintf("Total: ~%d tokens across %d sections", totalTokens, len(analysis.Sections)),
			SuggestedImprovement: "Review for redundant content that the model can infer from project files",
		})
	}

	// Per-section warnings
	for _, s := range analysis.Sections {
		sectionTokens := estimateTokens(len(s.Content))
		if sectionTokens > tokenWarnSection {
			findings = append(findings, llmutil.Finding{
				Category:             "redundancy",
				Issue:                fmt.Sprintf("Section %q uses ~%d tokens (threshold: %d)", s.Header, sectionTokens, tokenWarnSection),
				Confidence:           "moderate",
				CitationID:           "redundancy",
				CurrentState:         fmt.Sprintf("Section %q: ~%d tokens", s.Header, sectionTokens),
				SuggestedImprovement: "Consider splitting this section or removing redundant content",
			})
		}
	}

	return findings
}

// Indicators returns token count metrics.
func (t *tokensCheck) Indicators(analysis *parser.AgentAnalysis) []Indicator {
	if analysis == nil || len(analysis.RawContent) == 0 {
		return nil
	}

	totalTokens := estimateTokens(len(analysis.RawContent))
	avgPerSection := 0
	if len(analysis.Sections) > 0 {
		avgPerSection = totalTokens / len(analysis.Sections)
	}

	return []Indicator{
		{
			Name:     "Tokens",
			Value:    float64(totalTokens),
			Unit:     "estimated",
			Guidance: fmt.Sprintf("~%d per section avg", avgPerSection),
		},
	}
}

func estimateTokens(charCount int) int {
	return int(float64(charCount) / 4.0 * tokenCorrectionFactor)
}
