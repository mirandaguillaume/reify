package doctor

import (
	"encoding/json"
)

// JSONReport is the structured output for --format json.
type JSONReport struct {
	File            string           `json:"file"`
	Format          string           `json:"format"`
	StructuralScore JSONStructural   `json:"structural_score"`
	Indicators      []JSONIndicator  `json:"indicators"`
	Findings        []JSONFinding    `json:"findings"`
	Gate            GateResult       `json:"gate"`
}

// JSONStructural is the structural score in JSON format.
type JSONStructural struct {
	Passed     int      `json:"passed"`
	Total      int      `json:"total"`
	Percentage int      `json:"percentage"`
	Missing    []string `json:"missing"`
}

// JSONIndicator is an indicator in JSON format.
type JSONIndicator struct {
	Name     string  `json:"name"`
	Value    float64 `json:"value"`
	Unit     string  `json:"unit"`
	Guidance string  `json:"guidance"`
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
	pct := 0
	if report.StructuralScore.Total > 0 {
		pct = report.StructuralScore.Passed * 100 / report.StructuralScore.Total
	}

	missing := report.StructuralScore.Missing
	if missing == nil {
		missing = []string{}
	}

	jr := JSONReport{
		File:   filePath,
		Format: report.Format,
		StructuralScore: JSONStructural{
			Passed:     report.StructuralScore.Passed,
			Total:      report.StructuralScore.Total,
			Percentage: pct,
			Missing:    missing,
		},
		Indicators: make([]JSONIndicator, 0),
		Findings:   make([]JSONFinding, 0),
		Gate:       report.GateResult,
	}

	for _, ind := range report.Indicators {
		jr.Indicators = append(jr.Indicators, JSONIndicator{
			Name:     ind.Name,
			Value:    ind.Value,
			Unit:     ind.Unit,
			Guidance: ind.Guidance,
		})
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
