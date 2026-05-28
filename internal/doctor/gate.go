package doctor

import (
	"fmt"
	"strings"

	"github.com/mirandaguillaume/reify/internal/doctor/llmutil"
)

// GateCondition defines a single quality gate condition.
type GateCondition struct {
	Metric    string  `yaml:"metric" json:"metric"`
	Operator  string  `yaml:"operator" json:"operator"`
	Threshold float64 `yaml:"threshold" json:"threshold"`
	Blocking  bool    `yaml:"blocking" json:"blocking"`
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
// Note: secret-scanning and structural-presence conditions were dropped along
// with the static check infrastructure; the gate is now LLM-finding-centric.
func DefaultGate() *QualityGate {
	return &QualityGate{
		Conditions: []GateCondition{
			{Metric: "high_findings", Operator: "<=", Threshold: 5, Blocking: false},
		},
	}
}

// Evaluate checks all conditions against finding-derived metrics.
// Gate passes iff ALL blocking conditions pass.
//
// An empty Conditions slice produces a warning rather than silently passing —
// "all checks satisfied" must not imply "no checks ran".
func (g *QualityGate) Evaluate(findings []llmutil.Finding) GateResult {
	metrics := computeMetrics(findings)
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

func computeMetrics(findings []llmutil.Finding) map[string]float64 {
	highFindings := 0
	for _, f := range findings {
		sev := f.Severity
		if sev == "" {
			sev = f.Confidence
		}
		if strings.ToLower(sev) == "high" {
			highFindings++
		}
	}

	return map[string]float64{
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
		return false
	}
}
