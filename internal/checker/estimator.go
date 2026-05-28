package checker

import (
	"strings"

	"github.com/mirandaguillaume/reify/internal/classifier"
)

// RiskLevel is a qualitative compliance risk for one instruction on one harness.
type RiskLevel int

const (
	RiskLow    RiskLevel = iota // likely followed
	RiskMedium                  // one concern
	RiskHigh                    // multiple concerns, likely ignored
)

func (r RiskLevel) String() string {
	switch r {
	case RiskLow:
		return "low"
	case RiskMedium:
		return "medium"
	case RiskHigh:
		return "high"
	default:
		return "unknown"
	}
}

// RiskFactors lists the evidence-backed reasons behind a risk level.
// Each factor maps to a published finding or documented observation.
type RiskFactors struct {
	NegativeFraming    bool // IFEval (Zhou et al. 2023): "do not X" harder than "always Y"
	MiddlePosition     bool // Liu et al. 2023 "Lost in the Middle" + Veseli et al. 2025
	SemanticConstraint bool // Not statically verifiable → harder to enforce automatically
	HarnessWeakness    bool // Observed weaker point for this specific harness (empirical, not peer-reviewed)
}

// ActiveFactors returns the list of active risk factor descriptions.
func (f RiskFactors) ActiveFactors() []string {
	var out []string
	if f.NegativeFraming {
		out = append(out, "negative framing (IFEval, Zhou 2023)")
	}
	if f.MiddlePosition {
		out = append(out, "middle of file (Lost in the Middle, Liu 2023)")
	}
	if f.SemanticConstraint {
		out = append(out, "semantic — not statically verifiable")
	}
	if f.HarnessWeakness {
		out = append(out, "empirically weaker on this harness (community observation)")
	}
	return out
}

// Count returns the number of active risk factors.
func (f RiskFactors) Count() int {
	n := 0
	if f.NegativeFraming {
		n++
	}
	if f.MiddlePosition {
		n++
	}
	if f.SemanticConstraint {
		n++
	}
	if f.HarnessWeakness {
		n++
	}
	return n
}

// AssessRisk derives a qualitative risk level from the instruction properties.
func AssessRisk(ir InstructionResult, harness string) (RiskLevel, RiskFactors) {
	factors := RiskFactors{
		NegativeFraming:    ir.IsNegative,
		MiddlePosition:     ir.Position > 0.25 && ir.Position < 0.75,
		SemanticConstraint: !ir.IsVerifiable && ir.Facet != classifier.FacetContext,
		HarnessWeakness:    harnessWeakness(ir, harness),
	}

	switch factors.Count() {
	case 0:
		return RiskLow, factors
	case 1:
		return RiskMedium, factors
	default:
		return RiskHigh, factors
	}
}

// harnessWeakness returns true when this instruction type is an observed
// weak point for the harness. Labeled as empirical community observation,
// not peer-reviewed.
func harnessWeakness(ir InstructionResult, harness string) bool {
	switch harness {
	case "copilot":
		// Copilot (GPT-based by default) reported by community as less reliable
		// on negative constraints and instructions buried in long files.
		return ir.IsNegative || (ir.Position > 0.5 && ir.Facet == classifier.FacetGuardrails)
	case "cursor":
		// Cursor behavior depends on configured model (Claude or GPT).
		// Negative constraints flagged as medium concern for non-Claude configs.
		return ir.IsNegative
	default:
		return false
	}
}

// isNegativeInstruction detects prohibition framing.
func isNegativeInstruction(text string) bool {
	t := strings.ToLower(strings.TrimSpace(text))
	return strings.HasPrefix(t, "never ") ||
		strings.HasPrefix(t, "don't ") ||
		strings.HasPrefix(t, "do not ") ||
		strings.HasPrefix(t, "avoid ") ||
		strings.HasPrefix(t, "must not") ||
		strings.Contains(t, " never ") ||
		strings.Contains(t, " do not ")
}

// isVerifiableInstruction detects statically checkable instructions.
func isVerifiableInstruction(text string, facet classifier.Facet) bool {
	t := strings.ToLower(text)
	if facet == classifier.FacetSecurity {
		return true
	}
	verifiableKeywords := []string{
		"tab", "space", "indent", "quote", "semicolon", "comma",
		"header", "copyright", "import", "export", "type ", "interface",
		"test file", "test suite", ".test.", "_test.", "suffix", "prefix",
	}
	for _, kw := range verifiableKeywords {
		if strings.Contains(t, kw) {
			return true
		}
	}
	return false
}

// buildSuggestions generates improvement suggestions for risky instructions.
func buildSuggestions(ir InstructionResult, targets []string) []string {
	var suggestions []string

	if ir.IsNegative {
		for _, h := range targets {
			level, _ := AssessRisk(ir, h)
			if level >= RiskHigh {
				suggestions = append(suggestions,
					`Reframe as positive — "always do X" instead of "never do Y" (IFEval shows better compliance)`)
				break
			}
		}
	}

	if ir.Position > 0.50 && ir.Facet == classifier.FacetGuardrails {
		suggestions = append(suggestions,
			"Move to top 25% of file — critical instructions in the middle are less followed (Liu 2023)")
	}

	return dedupe(suggestions)
}

func dedupe(ss []string) []string {
	seen := make(map[string]bool)
	var out []string
	for _, s := range ss {
		if !seen[s] {
			seen[s] = true
			out = append(out, s)
		}
	}
	return out
}
