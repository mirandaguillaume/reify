package classifier

import (
	"strings"
)

type Facet string

const (
	FacetContext       Facet = "context"
	FacetStrategy      Facet = "strategy"
	FacetGuardrails    Facet = "guardrails"
	FacetObservability Facet = "observability"
	FacetSecurity      Facet = "security"
)

var AllFacets = []Facet{
	FacetContext,
	FacetStrategy,
	FacetGuardrails,
	FacetObservability,
	FacetSecurity,
}

// Item is a single classified instruction extracted from an agent file.
type Item struct {
	Text    string
	Facet   Facet
	Section string
}

// Result holds the full classification of an agent file.
type Result struct {
	Format string
	Items  []Item
}

// ByFacet returns items grouped by facet, preserving AllFacets order.
func (r Result) ByFacet() map[Facet][]Item {
	m := make(map[Facet][]Item, len(AllFacets))
	for _, f := range AllFacets {
		m[f] = nil
	}
	for _, item := range r.Items {
		m[item.Facet] = append(m[item.Facet], item)
	}
	return m
}

// Classify parses content and classifies each instruction into a Reify facet.
// format is a hint about the source format ("claude", "copilot", "reify", "").
func Classify(content, format string) Result {
	lines := strings.Split(content, "\n")
	result := Result{Format: format}

	currentSection := ""
	sectionFacet := FacetContext // default for unrecognized sections

	for _, raw := range lines {
		line := strings.TrimSpace(raw)
		if line == "" {
			continue
		}

		// Detect section headers (## or #).
		if strings.HasPrefix(line, "#") {
			heading := strings.TrimLeft(line, "# ")
			currentSection = heading
			sectionFacet = sectionFacetFor(heading)
			continue
		}

		// Skip non-instruction lines (code blocks, horizontal rules).
		if strings.HasPrefix(line, "```") || strings.HasPrefix(line, "---") || strings.HasPrefix(line, "===") {
			continue
		}

		// Extract bullet or numbered list items; skip bare prose lines
		// that are section descriptions rather than actionable instructions.
		text := extractInstruction(line)
		if text == "" {
			continue
		}

		facet := classifyLine(text, sectionFacet)
		result.Items = append(result.Items, Item{
			Text:    text,
			Facet:   facet,
			Section: currentSection,
		})
	}

	return result
}

// sectionFacetFor maps a section heading to its default facet.
func sectionFacetFor(heading string) Facet {
	h := strings.ToLower(heading)

	// Guardrails
	if containsAny(h, "guardrail", "rule", "constraint", "prohibition", "restriction", "must not", "forbidden") {
		return FacetGuardrails
	}
	// Security
	if containsAny(h, "security", "permission", "access", "credential", "secret", "auth") {
		return FacetSecurity
	}
	// Observability
	if containsAny(h, "observ", "metric", "log", "monitor", "trace", "telemetry") {
		return FacetObservability
	}
	// Strategy
	if containsAny(h, "command", "workflow", "tool", "strategy", "approach", "dev", "usage", "run", "build", "test", "deploy", "install", "setup") {
		return FacetStrategy
	}
	// Context
	if containsAny(h, "context", "about", "overview", "stack", "architecture", "tech", "project", "description", "background", "environment", "identity", "persona", "role") {
		return FacetContext
	}

	return FacetContext
}

// classifyLine overrides the section facet with line-level signals.
func classifyLine(text string, sectionFacet Facet) Facet {
	t := strings.ToLower(text)

	// Strong guardrail signals override section default.
	if containsAny(t, "never ", "don't ", "do not ", "must not", "avoid ", "prohibited", "forbidden", "not allowed", "disallow") {
		return FacetGuardrails
	}

	// Strong security signals.
	if containsAny(t, "api key", "secret", "credential", "token", "password", "permission", "access control", "filesystem", "network access", "allowlist") {
		return FacetSecurity
	}

	// Strong observability signals.
	if containsAny(t, "log ", "metric", "trace ", "monitor", "observe", "measure", "telemetry", "report ") {
		return FacetObservability
	}

	return sectionFacet
}

// extractInstruction returns the instruction text from a bullet or numbered
// list line, or empty string if the line is not an instruction.
func extractInstruction(line string) string {
	// Bullet list markers.
	for _, prefix := range []string{"- ", "* ", "+ "} {
		if strings.HasPrefix(line, prefix) {
			return strings.TrimSpace(line[len(prefix):])
		}
	}
	// Numbered list: "1. ", "2. ", etc.
	if len(line) > 2 && line[0] >= '0' && line[0] <= '9' {
		if idx := strings.Index(line, ". "); idx > 0 && idx <= 3 {
			return strings.TrimSpace(line[idx+2:])
		}
	}
	// Inline code or command lines (e.g. `go test ./...`).
	if strings.HasPrefix(line, "`") && strings.HasSuffix(line, "`") {
		return line
	}
	return ""
}

func containsAny(s string, subs ...string) bool {
	for _, sub := range subs {
		if strings.Contains(s, sub) {
			return true
		}
	}
	return false
}
