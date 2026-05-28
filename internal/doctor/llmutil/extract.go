// Package llmutil provides utilities for parsing structured output from LLM responses.
package llmutil

import (
	"fmt"
	"regexp"
	"strings"
)

// yamlFenceRe matches markdown code fences wrapping YAML content.
var yamlFenceRe = regexp.MustCompile("(?s)```(?:ya?ml)?\\s*\\n(.*?)\\n```")

// ExtractYAML strips markdown code fences from LLM output and returns clean YAML.
// Handles: ```yaml, ```yml, ```, and no fences. Extracts ALL fenced blocks if multiple.
func ExtractYAML(output string) (string, error) {
	if strings.TrimSpace(output) == "" {
		return "", fmt.Errorf("empty LLM response")
	}

	matches := yamlFenceRe.FindAllStringSubmatch(output, -1)
	if len(matches) > 0 {
		var parts []string
		for _, m := range matches {
			if len(m) > 1 {
				parts = append(parts, strings.TrimSpace(m[1]))
			}
		}
		return strings.Join(parts, "\n"), nil
	}

	// No fences — return trimmed output as-is
	return strings.TrimSpace(output), nil
}
