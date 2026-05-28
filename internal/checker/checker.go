package checker

import (
	"strings"

	"github.com/mirandaguillaume/reify/internal/classifier"
)

// Harnesses supported by the compliance checker.
var Harnesses = []string{"claude-code", "copilot", "cursor"}

// InstructionResult holds compliance risk for one instruction.
type InstructionResult struct {
	Text         string
	Facet        classifier.Facet
	Section      string
	Position     float64 // 0.0 = top of file, 1.0 = bottom
	IsNegative   bool
	IsVerifiable bool
	Risks        map[string]RiskLevel   // harness → risk level
	Factors      map[string]RiskFactors // harness → active factors
	Suggestions  []string
}

// CheckResult is the output of the check command.
type CheckResult struct {
	Instructions []InstructionResult
	Overall      map[string]RiskLevel // harness → worst risk seen
	HighRiskCount map[string]int       // harness → count of high-risk instructions
}

// Check classifies and assesses compliance risk for each instruction.
func Check(content, format string, targets []string, classification classifier.Result) CheckResult {
	if len(targets) == 0 {
		targets = Harnesses
	}

	items := classification.Items
	if len(items) == 0 {
		return CheckResult{}
	}

	lines := strings.Split(content, "\n")
	total := len(lines)
	results := make([]InstructionResult, 0, len(items))

	for _, item := range items {
		pos := estimatePosition(item.Text, lines, total)
		isNeg := isNegativeInstruction(item.Text)
		isVerif := isVerifiableInstruction(item.Text, item.Facet)

		ir := InstructionResult{
			Text:         item.Text,
			Facet:        item.Facet,
			Section:      item.Section,
			Position:     pos,
			IsNegative:   isNeg,
			IsVerifiable: isVerif,
			Risks:        make(map[string]RiskLevel, len(targets)),
			Factors:      make(map[string]RiskFactors, len(targets)),
		}

		for _, h := range targets {
			level, factors := AssessRisk(ir, h)
			ir.Risks[h] = level
			ir.Factors[h] = factors
		}

		ir.Suggestions = buildSuggestions(ir, targets)
		results = append(results, ir)
	}

	overall, highCount := computeOverall(results, targets)

	return CheckResult{
		Instructions:  results,
		Overall:       overall,
		HighRiskCount: highCount,
	}
}

func estimatePosition(text string, lines []string, total int) float64 {
	if total == 0 {
		return 0.5
	}
	prefix := text
	if len(prefix) > 30 {
		prefix = prefix[:30]
	}
	for i, line := range lines {
		if strings.Contains(line, prefix) {
			return float64(i) / float64(total)
		}
	}
	return 0.5
}

func computeOverall(results []InstructionResult, targets []string) (map[string]RiskLevel, map[string]int) {
	worst := make(map[string]RiskLevel, len(targets))
	highCount := make(map[string]int, len(targets))
	for _, r := range results {
		for _, h := range targets {
			if level := r.Risks[h]; level > worst[h] {
				worst[h] = level
			}
			if r.Risks[h] == RiskHigh {
				highCount[h]++
			}
		}
	}
	return worst, highCount
}
