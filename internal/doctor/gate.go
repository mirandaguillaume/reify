package doctor

import (
	"fmt"
	"strings"

	"github.com/mirandaguillaume/reify/internal/doctor/llmutil"
)

// GateCondition defines a single quality gate condition.
type GateCondition struct {
	Metric   string  `yaml:"metric" json:"metric"`
	Operator string  `yaml:"operator" json:"operator"`
	Threshold float64 `yaml:"threshold" json:"threshold"`
	Blocking bool    `yaml:"blocking" json:"blocking"`
}

// QualityGate holds conditions that must be satisfied for the gate to pass.
type QualityGate struct {
	Conditions []GateCondition
}

// GateResult holds the outcome of a quality gate evaluation.
type GateResult struct {
	Pass     bool     `json:"pass"`
	Failures []string `json:"failures,omitempty"`
	Warnings []string `json:"warnings,omitempty"`
}

// DefaultGate returns the built-in quality gate conditions.
func DefaultGate() *QualityGate {
	return &QualityGate{
		Conditions: []GateCondition{
			{Metric: "secrets_found", Operator: "==", Threshold: 0, Blocking: true},
			{Metric: "structural_pct", Operator: ">=", Threshold: 50, Blocking: true},
			{Metric: "high_findings", Operator: "<=", Threshold: 5, Blocking: false},
		},
	}
}

// isSecretFinding returns true if the finding represents a detected secret.
// Centralized here so gate evaluation (gate.go) and structural scoring
// (render.go ComputeStructural) stay in sync. Heuristic: security category +
// Issue text contains "detected". Story 4-0 AC #3.
func isSecretFinding(f llmutil.Finding) bool {
	return strings.EqualFold(f.Category, "security") && strings.Contains(strings.ToLower(f.Issue), "detected")
}

// Evaluate checks all conditions against the report metrics.
// Gate passes iff ALL blocking conditions pass.
//
// An empty Conditions slice does NOT silently pass — it produces a warning
// so the user knows the gate configuration is empty. A silent pass on an
// empty gate is dangerous because it implies "all checks satisfied" when in
// reality no checks ran. Story 4-0 AC #4.
func (g *QualityGate) Evaluate(structural StructuralResult, findings []llmutil.Finding) GateResult {
	metrics := computeMetrics(structural, findings)
	result := GateResult{Pass: true}

	if len(g.Conditions) == 0 {
		result.Warnings = append(result.Warnings, "quality gate has no conditions — nothing was evaluated")
		return result
	}

	for _, cond := range g.Conditions {
		val, ok := metrics[cond.Metric]
		if !ok {
			msg := fmt.Sprintf("unknown metric %q — condition skipped", cond.Metric)
			result.Warnings = append(result.Warnings, msg)
			continue
		}

		passed := evalCondition(val, cond.Operator, cond.Threshold)
		if !passed {
			msg := fmt.Sprintf("%s %s %.0f (actual: %.0f)", cond.Metric, cond.Operator, cond.Threshold, val)
			if cond.Blocking {
				result.Pass = false
				result.Failures = append(result.Failures, msg)
			} else {
				result.Warnings = append(result.Warnings, msg)
			}
		}
	}

	return result
}

func computeMetrics(structural StructuralResult, findings []llmutil.Finding) map[string]float64 {
	secretsFound := 0
	highFindings := 0
	for _, f := range findings {
		if isSecretFinding(f) {
			secretsFound++
		}
		sev := f.Severity
		if sev == "" {
			sev = f.Confidence
		}
		if strings.ToLower(sev) == "high" {
			highFindings++
		}
	}

	pct := float64(0)
	if structural.Total > 0 {
		pct = float64(structural.Passed) * 100 / float64(structural.Total)
	}

	return map[string]float64{
		"secrets_found":  float64(secretsFound),
		"structural_pct": pct,
		"high_findings":  float64(highFindings),
		"total_findings": float64(len(findings)),
	}
}

func evalCondition(value float64, operator string, threshold float64) bool {
	switch operator {
	case "==":
		return value == threshold
	case "!=":
		return value != threshold
	case ">=":
		return value >= threshold
	case "<=":
		return value <= threshold
	case ">":
		return value > threshold
	case "<":
		return value < threshold
	default:
		return false // unknown operator — fail-closed for safety
	}
}
