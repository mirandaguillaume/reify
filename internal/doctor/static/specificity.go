package static

import (
	"strings"

	"github.com/mirandaguillaume/reify/internal/doctor/llmutil"
	"github.com/mirandaguillaume/reify/internal/doctor/parser"
)

func init() {
	RegisterCheck(&specificityCheck{})
}

var directiveWords = []string{"must", "always", "never", "required", "ensure", "shall", "will"}
var advisoryWords = []string{"may", "consider", "could", "optional", "suggested", "might", "perhaps"}

type specificityCheck struct{}

func (s *specificityCheck) ID() string              { return "specificity-ratio" }
func (s *specificityCheck) Tags() []string          { return []string{"thorough"} }
func (s *specificityCheck) Category() string        { return "specificity" }
func (s *specificityCheck) DefaultSeverity() string { return "low" }

// Run returns no findings — specificity is an indicator only.
func (s *specificityCheck) Run(_ *parser.AgentAnalysis) []llmutil.Finding {
	return nil
}

// Indicators computes the directive/advisory word ratio.
func (s *specificityCheck) Indicators(analysis *parser.AgentAnalysis) []Indicator {
	if analysis == nil || len(analysis.RawContent) == 0 {
		return nil
	}

	content := strings.ToLower(string(analysis.RawContent))
	words := strings.Fields(content)

	dirCount := 0
	advCount := 0
	for _, w := range words {
		clean := strings.Trim(w, ".,;:!?\"'()[]")
		for _, d := range directiveWords {
			if clean == d {
				dirCount++
				break
			}
		}
		for _, a := range advisoryWords {
			if clean == a {
				advCount++
				break
			}
		}
	}

	total := dirCount + advCount
	if total == 0 {
		return nil
	}

	ratio := float64(dirCount) / float64(total)

	guidance := "balanced"
	if ratio > 0.8 {
		guidance = "highly directive"
	} else if ratio > 0.6 {
		guidance = "directive-leaning"
	} else if ratio < 0.3 {
		guidance = "too advisory — add more specific requirements"
	}

	return []Indicator{
		{
			Name:     "Specificity",
			Value:    ratio,
			Unit:     "directive ratio",
			Guidance: guidance,
		},
	}
}
