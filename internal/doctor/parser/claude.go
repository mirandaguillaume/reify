package parser

import (
	"bytes"
	"fmt"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

type claudeParser struct{}

func init() {
	Register("claude", func() FormatParser { return &claudeParser{} })
}

func (p *claudeParser) Format() string { return "claude" }

// Detect returns true if the file looks like a Claude Code agent or skill file.
func (p *claudeParser) Detect(path string, content []byte) bool {
	base := filepath.Base(path)
	dir := filepath.Dir(path)

	// Path-based detection
	if strings.Contains(dir, ".claude/agents") || strings.Contains(dir, ".claude"+string(filepath.Separator)+"agents") {
		return true
	}
	if strings.Contains(dir, ".claude/skills") || strings.Contains(dir, ".claude"+string(filepath.Separator)+"skills") {
		return true
	}
	if base == "CLAUDE.md" {
		return true
	}

	// Frontmatter-based detection: look for Claude-typical fields.
	// extractFrontmatter uses a tolerant fallback parser for frontmatter
	// with unquoted colons, so no text-based fallback is needed.
	fm, _, err := extractFrontmatter(content)
	if err == nil && fm != nil {
		if _, ok := fm["name"]; ok {
			for _, field := range []string{"tools", "model", "description", "color"} {
				if _, has := fm[field]; has {
					return true
				}
			}
		}
	}

	return false
}

// Parse extracts structure from a Claude Code agent file.
func (p *claudeParser) Parse(content []byte) (*AgentAnalysis, error) {
	// Normalize CRLF → LF for consistent parsing across platforms
	normalized := bytes.ReplaceAll(content, []byte("\r\n"), []byte("\n"))
	fm, body, err := extractFrontmatter(normalized)
	if err != nil {
		// No frontmatter — treat entire content as body
		fm = make(map[string]interface{})
		body = content
	}

	sections := parseSections(body)
	tools := extractTools(fm)

	return &AgentAnalysis{
		Format:      "claude",
		Frontmatter: fm,
		Sections:    sections,
		Tools:       tools,
		RawContent:  content,
	}, nil
}

// Validate checks that a rewritten file preserves the original structure.
func (p *claudeParser) Validate(original, rewritten []byte) error {
	// Check frontmatter is still parseable
	origFM, _, _ := extractFrontmatter(original)
	newFM, _, err := extractFrontmatter(rewritten)
	if err != nil && origFM != nil {
		return fmt.Errorf("rewritten file has broken frontmatter: %w", err)
	}

	// Check all original frontmatter fields are preserved (key + value)
	for key, origVal := range origFM {
		newVal, ok := newFM[key]
		if !ok {
			return fmt.Errorf("rewritten file lost frontmatter field: %s", key)
		}
		if fmt.Sprintf("%v", origVal) != fmt.Sprintf("%v", newVal) {
			return fmt.Errorf("rewritten file changed frontmatter field %s: %v → %v", key, origVal, newVal)
		}
	}

	// Check sections are preserved
	origSections := parseSections(bodyAfterFrontmatter(original))
	newSections := parseSections(bodyAfterFrontmatter(rewritten))

	origHeaders := make(map[string]bool)
	for _, s := range origSections {
		if s.Header != "" {
			origHeaders[s.Header] = true
		}
	}
	for header := range origHeaders {
		found := false
		for _, s := range newSections {
			if s.Header == header {
				found = true
				break
			}
		}
		if !found {
			return fmt.Errorf("rewritten file lost section: %s", header)
		}
	}

	return nil
}

// extractFrontmatter splits YAML frontmatter from the markdown body.
// It first tries yaml.v3 for valid YAML frontmatter. On failure, it falls
// back to a tolerant first-level key extractor that handles unquoted colons
// in values (common in real Claude agent files).
func extractFrontmatter(content []byte) (map[string]interface{}, []byte, error) {
	s := string(content)
	if !strings.HasPrefix(strings.TrimSpace(s), "---") {
		return nil, content, fmt.Errorf("no frontmatter delimiter")
	}

	// Find the opening ---
	trimmed := strings.TrimSpace(s)
	rest := trimmed[3:] // skip first ---

	// Find closing ---
	idx := strings.Index(rest, "\n---")
	if idx < 0 {
		return nil, content, fmt.Errorf("no closing frontmatter delimiter")
	}

	yamlStr := rest[:idx]
	afterClosing := rest[idx+4:]

	// Try yaml.v3 first (handles lists, nested structures correctly)
	var fm map[string]interface{}
	if err := yaml.Unmarshal([]byte(yamlStr), &fm); err == nil {
		if fm == nil {
			fm = make(map[string]interface{})
		}
		return fm, []byte(afterClosing), nil
	}

	// Fallback: tolerant first-level key extractor for frontmatter with
	// unquoted colons in values (e.g. description: ... user: "test" ...)
	fm = extractFirstLevelKeys(yamlStr)
	if fm == nil {
		fm = make(map[string]interface{})
	}

	return fm, []byte(afterClosing), nil
}

// extractFirstLevelKeys parses frontmatter line-by-line, treating each
// unindented "key: value" as a top-level entry. Indented lines following a key
// are treated as continuation (list items or multi-line values).
// This handles values containing unquoted colons that break yaml.v3.
func extractFirstLevelKeys(yamlStr string) map[string]interface{} {
	fm := make(map[string]interface{})
	lines := strings.Split(yamlStr, "\n")

	var currentKey string
	var currentVal strings.Builder
	var listItems []interface{}

	flush := func() {
		if currentKey == "" {
			return
		}
		if listItems != nil {
			fm[currentKey] = listItems
		} else {
			fm[currentKey] = strings.TrimSpace(currentVal.String())
		}
		currentKey = ""
		currentVal.Reset()
		listItems = nil
	}

	for _, line := range lines {
		trimmedLine := strings.TrimSpace(line)
		if trimmedLine == "" {
			continue
		}

		// Indented line = continuation of current value (list item or multi-line)
		isIndented := len(line) > 0 && (line[0] == ' ' || line[0] == '\t')
		if isIndented && currentKey != "" {
			if strings.HasPrefix(trimmedLine, "- ") {
				if listItems == nil {
					listItems = []interface{}{}
				}
				listItems = append(listItems, strings.TrimSpace(trimmedLine[2:]))
			}
			continue
		}

		// Top-level line — check for "key: value" pattern
		colonIdx := strings.Index(line, ":")
		if colonIdx > 0 {
			candidate := line[:colonIdx]
			if isValidYAMLKey(candidate) {
				flush()
				currentKey = candidate
				currentVal.WriteString(line[colonIdx+1:])
				continue
			}
		}

		// Unrecognized line — skip
	}

	flush()
	return fm
}

// isValidYAMLKey checks if a string is a valid frontmatter key
// (alphanumeric, underscores, hyphens, no spaces, starts at column 0).
func isValidYAMLKey(s string) bool {
	if s == "" {
		return false
	}
	for _, r := range s {
		if !((r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '_' || r == '-') {
			return false
		}
	}
	return true
}

// bodyAfterFrontmatter returns just the body after frontmatter, for validation.
func bodyAfterFrontmatter(content []byte) []byte {
	_, body, _ := extractFrontmatter(content)
	return body
}

// parseSections splits markdown content into sections by headers.
func parseSections(body []byte) []Section {
	var sections []Section
	var current *Section
	inCodeFence := false

	lines := strings.Split(string(body), "\n")
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)

		// Track code fences to avoid treating comments as headers
		if strings.HasPrefix(trimmed, "```") {
			inCodeFence = !inCodeFence
		}

		if !inCodeFence && strings.HasPrefix(trimmed, "#") {
			// Count header level
			level := 0
			for _, c := range trimmed {
				if c == '#' {
					level++
				} else {
					break
				}
			}
			header := strings.TrimSpace(trimmed[level:])

			if current != nil {
				sections = append(sections, *current)
			}
			current = &Section{
				Header:  header,
				Content: "",
				Level:   level,
			}
		} else if current != nil {
			if current.Content != "" {
				current.Content += "\n"
			}
			current.Content += line
		} else {
			// Content before any header — create a preamble section
			if trimmed != "" {
				if current == nil {
					current = &Section{Header: "", Content: "", Level: 0}
				}
				if current.Content != "" {
					current.Content += "\n"
				}
				current.Content += line
			}
		}
	}

	if current != nil {
		sections = append(sections, *current)
	}

	return sections
}

// extractTools pulls tool names from frontmatter.
func extractTools(fm map[string]interface{}) []string {
	val, ok := fm["tools"]
	if !ok {
		return nil
	}

	switch v := val.(type) {
	case string:
		// "Read, Grep, Glob" or "Read Grep Glob"
		parts := strings.FieldsFunc(v, func(r rune) bool {
			return r == ',' || r == ' '
		})
		var tools []string
		for _, p := range parts {
			p = strings.TrimSpace(p)
			if p != "" {
				tools = append(tools, p)
			}
		}
		return tools
	case []interface{}:
		var tools []string
		for _, item := range v {
			if s, ok := item.(string); ok {
				tools = append(tools, s)
			}
		}
		return tools
	}

	return nil
}
