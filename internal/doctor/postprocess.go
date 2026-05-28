package doctor

import (
	"fmt"
	"sort"
	"strings"

	"github.com/mirandaguillaume/reify/internal/doctor/llmutil"
)

const defaultMaxFindings = 20

// PostProcess deduplicates, sorts, and limits findings.
func PostProcess(findings []llmutil.Finding, maxFindings int) []llmutil.Finding {
	if maxFindings <= 0 {
		maxFindings = defaultMaxFindings
	}

	findings = dedup(findings)
	sortFindings(findings)

	if len(findings) > maxFindings {
		truncated := findings[:maxFindings:maxFindings] // cap capacity to prevent backing array corruption
		truncated = append(truncated, llmutil.Finding{
			Category:   "summary",
			Issue:      fmt.Sprintf("[%d more findings not shown]", len(findings)-maxFindings),
			Confidence: "low",
		})
		return truncated
	}

	return findings
}

func dedup(findings []llmutil.Finding) []llmutil.Finding {
	seen := make(map[string]bool)
	var result []llmutil.Finding
	for _, f := range findings {
		key := f.Category + "|" + f.Issue
		if seen[key] {
			continue
		}
		seen[key] = true
		result = append(result, f)
	}
	return result
}

var severityOrder = map[string]int{
	"high":     0,
	"moderate": 1,
	"low":      2,
}

func sortFindings(findings []llmutil.Finding) {
	sort.SliceStable(findings, func(i, j int) bool {
		si := severityOrder[strings.ToLower(findings[i].Confidence)]
		sj := severityOrder[strings.ToLower(findings[j].Confidence)]
		if si != sj {
			return si < sj
		}
		return findings[i].Category < findings[j].Category
	})
}
