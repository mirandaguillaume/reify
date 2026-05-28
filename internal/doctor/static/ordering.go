package static

import (
	"fmt"
	"sort"
	"strings"

	"github.com/mirandaguillaume/reify/internal/doctor/llmutil"
	"github.com/mirandaguillaume/reify/internal/doctor/parser"
)

func init() {
	RegisterCheck(&orderingCheck{})
}

// criticalSections are sections that should appear in the first third of the file
// due to primacy bias (Liu et al., 2024).
var criticalSections = map[string][]string{
	"identity":   {"identity", "persona", "role", "you are"},
	"guardrails": {"guardrails", "constraints", "limits", "boundaries"},
	"security":   {"security", "permissions", "access"},
}

type orderingCheck struct{}

func (o *orderingCheck) ID() string              { return "instruction-ordering" }
func (o *orderingCheck) Tags() []string          { return []string{"default"} }
func (o *orderingCheck) Category() string        { return "ordering" }
func (o *orderingCheck) DefaultSeverity() string { return "moderate" }

func (o *orderingCheck) Run(analysis *parser.AgentAnalysis) []llmutil.Finding {
	if analysis == nil || len(analysis.Sections) == 0 {
		return nil
	}

	totalSections := len(analysis.Sections)
	if totalSections < 3 {
		return nil // too few sections to evaluate ordering
	}

	firstThirdEnd := (totalSections + 2) / 3 // ceiling division

	// Sort for deterministic output order
	names := make([]string, 0, len(criticalSections))
	for n := range criticalSections {
		names = append(names, n)
	}
	sort.Strings(names)

	var findings []llmutil.Finding
	for _, name := range names {
		keywords := criticalSections[name]
		idx := findSectionIndex(analysis.Sections, keywords)
		if idx < 0 {
			continue // section not present — handled by presence check
		}
		if idx >= firstThirdEnd {
			findings = append(findings, llmutil.Finding{
				Category:             "ordering",
				Issue:                fmt.Sprintf("Critical section %q is at position %d/%d (should be in first third)", name, idx+1, totalSections),
				Confidence:           "moderate",
				CitationID:           "ordering",
				CurrentState:         fmt.Sprintf("Section %q found at position %d of %d sections", analysis.Sections[idx].Header, idx+1, totalSections),
				SuggestedImprovement: fmt.Sprintf("Move %s section to the top of the file to exploit primacy bias", name),
			})
		}
	}

	return findings
}

func findSectionIndex(sections []parser.Section, keywords []string) int {
	for i, s := range sections {
		lower := strings.ToLower(s.Header)
		for _, kw := range keywords {
			if strings.Contains(lower, kw) {
				return i
			}
		}
	}
	return -1
}
