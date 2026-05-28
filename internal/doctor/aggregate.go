package doctor

import (
	"github.com/mirandaguillaume/reify/internal/doctor/llmutil"
)

// FileResult holds the analysis outcome for a single file.
type FileResult struct {
	Path     string
	Format   string
	Findings []llmutil.Finding
	Error    error
}

// AggregateReport summarizes findings across multiple files.
type AggregateReport struct {
	TotalFiles        int
	AnalyzedFiles     int
	FailedFiles       int
	TotalFindings     int
	ByCategory        map[string]int
	FilesNoGuardrails []string
	FilesNoSecurity   []string
}

// Aggregate produces an AggregateReport from a slice of FileResults.
func Aggregate(results []FileResult) *AggregateReport {
	report := &AggregateReport{
		TotalFiles: len(results),
		ByCategory: make(map[string]int),
	}

	for _, r := range results {
		if r.Error != nil {
			report.FailedFiles++
			continue
		}
		report.AnalyzedFiles++
		report.TotalFindings += len(r.Findings)

		hasGuardrails := false
		hasSecurity := false

		for _, f := range r.Findings {
			report.ByCategory[f.Category]++
			if f.Category == "guardrails" {
				hasGuardrails = true
			}
			if f.Category == "security" {
				hasSecurity = true
			}
		}

		if !hasGuardrails {
			report.FilesNoGuardrails = append(report.FilesNoGuardrails, r.Path)
		}
		if !hasSecurity {
			report.FilesNoSecurity = append(report.FilesNoSecurity, r.Path)
		}
	}

	return report
}
