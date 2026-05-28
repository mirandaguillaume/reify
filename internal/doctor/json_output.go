package doctor

import (
	"encoding/json"
)

// JSONReport is the structured output for --format json.
type JSONReport struct {
	File     string        `json:"file"`
	Format   string        `json:"format"`
	Findings []JSONFinding `json:"findings"`
	Gate     GateResult    `json:"gate"`
}

// JSONFinding is a finding in JSON format.
type JSONFinding struct {
	Category   string `json:"category"`
	Issue      string `json:"issue"`
	Severity   string `json:"severity"`
	Suggestion string `json:"suggestion,omitempty"`
}

// ToJSON converts a Report to JSON bytes.
func ToJSON(report *Report, filePath string) ([]byte, error) {
	jr := JSONReport{
		File:     filePath,
		Format:   report.Format,
		Findings: make([]JSONFinding, 0),
		Gate:     report.GateResult,
	}

	for _, f := range report.AllFindings {
		sev := f.Severity
		if sev == "" {
			sev = f.Confidence
		}
		jr.Findings = append(jr.Findings, JSONFinding{
			Category:   f.Category,
			Issue:      f.Issue,
			Severity:   sev,
			Suggestion: f.SuggestedImprovement,
		})
	}

	return json.MarshalIndent(jr, "", "  ")
}
