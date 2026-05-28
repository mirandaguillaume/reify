package static

import (
	"sort"
	"strings"

	"github.com/mirandaguillaume/reify/internal/doctor/llmutil"
	"github.com/mirandaguillaume/reify/internal/doctor/parser"
)

func init() {
	RegisterCheck(&presenceCheck{})
}

// sectionMapping maps registry category IDs to keywords that indicate
// the section is present (case-insensitive partial match on headings).
var sectionMapping = map[string][]string{
	"security":               {"security", "permissions", "access control", "sandbox"},
	"guardrails":             {"guardrails", "constraints", "limits", "boundaries", "restrictions"},
	"testing":                {"testing", "tests", "test plan", "validation", "verification"},
	"examples":               {"example", "sample", "demo", "few-shot"},
	"error_handling":         {"error", "failure", "fallback", "recovery", "exception"},
	"build_commands":         {"build", "run", "commands", "scripts", "setup", "install"},
	"architecture_hints":     {"architecture", "structure", "design", "patterns", "layers"},
	"identity":               {"identity", "persona", "role", "you are", "your role"},
	"output_format":          {"output", "format", "response format", "schema"},
	"decision_authority":     {"decision", "authority", "autonomy", "escalat", "approval"},
	"workflow_triggers":      {"trigger", "when to use", "invocation", "activation", "invoke"},
	"dependency_declaration": {"input", "output", "consumes", "produces", "dependencies", "i/o"},
	"memory_management":      {"memory", "context", "session", "state", "persistence"},
	"goals":                  {"goal", "objective", "purpose", "mission"},
	"constraints":            {"constraint", "prohibition", "must not", "never", "forbidden"},
}

type presenceCheck struct{}

func (p *presenceCheck) ID() string              { return "section-presence" }
func (p *presenceCheck) Tags() []string          { return []string{"default"} }
func (p *presenceCheck) Category() string        { return "structure" }
func (p *presenceCheck) DefaultSeverity() string { return "moderate" }

func (p *presenceCheck) Run(analysis *parser.AgentAnalysis) []llmutil.Finding {
	if analysis == nil {
		return nil
	}

	// Collect all heading text + body content for keyword matching
	headings := make([]string, 0, len(analysis.Sections))
	for _, s := range analysis.Sections {
		headings = append(headings, strings.ToLower(s.Header))
	}

	// Also check raw content for non-header matches (e.g., "You are a..." in body)
	content := strings.ToLower(string(analysis.RawContent))

	// Sort categories for deterministic output order
	categories := make([]string, 0, len(sectionMapping))
	for cat := range sectionMapping {
		categories = append(categories, cat)
	}
	sort.Strings(categories)

	var findings []llmutil.Finding
	for _, category := range categories {
		keywords := sectionMapping[category]
		if sectionPresent(headings, content, keywords) {
			continue
		}
		findings = append(findings, llmutil.Finding{
			Category:             category,
			Issue:                "Missing " + category + " section",
			Confidence:           "moderate",
			CitationID:           category,
			CurrentState:         "No heading or content matches keywords: " + strings.Join(keywords[:min(3, len(keywords))], ", "),
			SuggestedImprovement: "Add a section addressing " + category,
		})
	}

	return findings
}

// bodyOnlyKeywords are multi-word phrases safe to search in body content.
// Single common words like "run", "error", "output" are heading-only to avoid false positives.
var bodyOnlyKeywords = map[string]bool{
	"you are": true, "your role": true, "access control": true,
	"test plan": true, "few-shot": true, "response format": true,
	"when to use": true, "must not": true, "i/o": true,
}

func sectionPresent(headings []string, content string, keywords []string) bool {
	// Check headings (strongest signal — all keywords allowed)
	for _, h := range headings {
		for _, kw := range keywords {
			if strings.Contains(h, kw) {
				return true
			}
		}
	}
	// Check body content only for multi-word phrases (low false-positive risk)
	for _, kw := range keywords {
		if bodyOnlyKeywords[kw] && strings.Contains(content, kw) {
			return true
		}
	}
	return false
}

