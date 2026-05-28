package importer

import (
	"strings"

	"gopkg.in/yaml.v3"
)

// AgentFrontmatter holds parsed YAML frontmatter from an agent file.
type AgentFrontmatter struct {
	Name        string      `yaml:"name"`
	Description string      `yaml:"description"`
	Tools       []string    `yaml:"-"`
	RawTools    interface{} `yaml:"tools"`
	Model       string      `yaml:"model"`
}

// ExtractFrontmatter parses YAML frontmatter delimited by "---" markers.
// It returns the parsed frontmatter, the remaining body (without the
// frontmatter block), and any error encountered during parsing.
// If no frontmatter is found, it returns an empty AgentFrontmatter and the
// full content as the body.
func ExtractFrontmatter(content string) (AgentFrontmatter, string, error) {
	lines := strings.SplitAfter(content, "\n")

	// Find the opening "---" marker.
	start := -1
	for i, line := range lines {
		if strings.TrimSpace(line) == "---" {
			start = i
			break
		}
	}
	if start == -1 {
		return AgentFrontmatter{}, content, nil
	}

	// Find the closing "---" marker.
	end := -1
	for i := start + 1; i < len(lines); i++ {
		if strings.TrimSpace(lines[i]) == "---" {
			end = i
			break
		}
	}
	if end == -1 {
		return AgentFrontmatter{}, content, nil
	}

	// Extract the YAML block between the markers.
	yamlBlock := strings.Join(lines[start+1:end], "")
	body := strings.Join(lines[end+1:], "")

	var fm AgentFrontmatter
	if err := yaml.Unmarshal([]byte(yamlBlock), &fm); err != nil {
		return AgentFrontmatter{}, content, err
	}

	fm.Tools = normalizeTools(fm.RawTools)
	return fm, body, nil
}

// normalizeTools converts the raw tools value (which may be a YAML list or a
// comma-separated string) into a deduplicated string slice.
func normalizeTools(raw interface{}) []string {
	if raw == nil {
		return nil
	}

	var tools []string
	switch v := raw.(type) {
	case []interface{}:
		for _, item := range v {
			if s, ok := item.(string); ok {
				tools = append(tools, strings.TrimSpace(s))
			}
		}
	case string:
		for _, part := range strings.Split(v, ",") {
			t := strings.TrimSpace(part)
			if t != "" {
				tools = append(tools, t)
			}
		}
	}
	return tools
}
