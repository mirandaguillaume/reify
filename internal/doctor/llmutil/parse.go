package llmutil

import (
	"fmt"
	"strings"

	"gopkg.in/yaml.v3"
)

// Finding represents a single analysis finding from the LLM or static checks.
type Finding struct {
	Category             string `yaml:"category"`
	Issue                string `yaml:"issue"`
	Confidence           string `yaml:"confidence"`
	Severity             string `yaml:"severity,omitempty"`
	CitationID           string `yaml:"citation_id,omitempty"`
	CurrentState         string `yaml:"current_state"`
	SuggestedImprovement string `yaml:"suggested_improvement"`
}

// validCategories lists the accepted finding categories.
var validCategories = map[string]bool{
	"guardrails":            true,
	"security":              true,
	"ordering":              true,
	"decomposition":         true,
	"context":               true,
	"redundancy":            true,
	"tool_usage":            true,
	"specificity":           true,
	"testing":               true,
	"examples":              true,
	"error_handling":        true,
	"scope":                 true,
	"memory_management":     true,
	"output_format":         true,
	"decision_authority":    true,
	"build_commands":        true,
	"architecture_hints":    true,
	"prompt_injection":      true,
	"idempotency":           true,
	"context_tiering":       true,
	"output_constraints":    true,
	"workflow_triggers":     true,
	"dependency_declaration": true,
	"version_drift":         true,
	"identity":              true,
	"goals":                 true,
	"constraints":           true,
	"structure":             true,
	"consistency":           true,
	"readability":           true,
	"agent_smell":           true,
}

// validConfidences lists the accepted confidence levels.
var validConfidences = map[string]bool{
	"high":     true,
	"moderate": true,
	"low":      true,
}

// findingsWrapper is the expected YAML output schema from the LLM.
type findingsWrapper struct {
	Findings []Finding `yaml:"findings"`
}

// ParseFindings parses YAML output into a list of validated findings.
// Returns an error if the YAML is invalid or uses an unexpected schema.
func ParseFindings(yamlStr string) ([]Finding, error) {
	if strings.TrimSpace(yamlStr) == "" {
		return nil, fmt.Errorf("empty YAML input")
	}

	// Try direct parse first
	var wrapper findingsWrapper
	err := yaml.Unmarshal([]byte(yamlStr), &wrapper)

	// If parse fails or no findings key, try to find "findings:" in the text
	// Local models often prepend conversational text before YAML
	if err != nil || wrapper.Findings == nil {
		if idx := strings.Index(yamlStr, "findings:"); idx > 0 {
			trimmed := yamlStr[idx:]
			var retry findingsWrapper
			if retryErr := yaml.Unmarshal([]byte(trimmed), &retry); retryErr == nil && retry.Findings != nil {
				wrapper = retry
				err = nil
			}
		}
	}

	if err != nil {
		return nil, fmt.Errorf("parse findings YAML: %w", err)
	}

	// Check for wrong key — valid YAML but no "findings" key
	if wrapper.Findings == nil {
		var raw map[string]interface{}
		if yamlErr := yaml.Unmarshal([]byte(yamlStr), &raw); yamlErr == nil && len(raw) > 0 {
			for key := range raw {
				if key != "findings" {
					return nil, fmt.Errorf("LLM used unexpected key %q instead of 'findings'", key)
				}
			}
		}
		return nil, fmt.Errorf("no 'findings' key in YAML output")
	}

	// Validate and filter findings
	var valid []Finding
	for _, f := range wrapper.Findings {
		if f.Issue == "" {
			continue // Skip empty findings
		}
		f.Category = normalizeCategory(f.Category)
		f.Confidence = normalizeConfidence(f.Confidence)
		valid = append(valid, f)
	}

	return valid, nil
}

func normalizeCategory(cat string) string {
	lower := strings.ToLower(strings.TrimSpace(cat))
	if validCategories[lower] {
		return lower
	}
	// Try without trailing 's' (e.g., "guardrails" → "guardrail" won't match, but "contexts" → "context" would)
	if strings.HasSuffix(lower, "s") {
		trimmed := lower[:len(lower)-1]
		if validCategories[trimmed] {
			return trimmed
		}
	}
	return lower // Return lowercased even if not recognized
}

func normalizeConfidence(conf string) string {
	lower := strings.ToLower(strings.TrimSpace(conf))
	if validConfidences[lower] {
		return lower
	}
	return lower
}
