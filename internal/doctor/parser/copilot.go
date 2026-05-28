package parser

import (
	"bytes"
	"fmt"
	"path/filepath"
	"strings"
)

type copilotParser struct{}

func init() {
	Register("copilot", func() FormatParser { return &copilotParser{} })
}

func (p *copilotParser) Format() string { return "copilot" }

// Detect returns true if the file looks like a GitHub Copilot agent or skill file.
func (p *copilotParser) Detect(path string, content []byte) bool {
	base := filepath.Base(path)
	dir := filepath.Dir(path)

	// Path-based detection with extension checks
	if strings.Contains(dir, ".github/agents") || strings.Contains(dir, ".github"+string(filepath.Separator)+"agents") {
		return strings.HasSuffix(base, ".md") // Only markdown files
	}
	if strings.Contains(dir, ".github/skills") || strings.Contains(dir, ".github"+string(filepath.Separator)+"skills") {
		return base == "SKILL.md" // Only SKILL.md in skills directories
	}
	if base == "copilot-instructions.md" && strings.Contains(dir, ".github") {
		return true
	}

	// Frontmatter-based detection: Copilot agents typically have tools as JSON array
	fm, _, err := extractFrontmatter(content)
	if err == nil && fm != nil {
		// Copilot agents commonly have description + tools, often without name
		if _, hasDesc := fm["description"]; hasDesc {
			if _, hasTools := fm["tools"]; hasTools {
				// Check if tools looks like JSON array (Copilot style)
				if toolsStr, ok := fm["tools"].(string); ok && strings.HasPrefix(strings.TrimSpace(toolsStr), "[") {
					return true
				}
				// YAML list of tools with Copilot-specific names
				if toolsList, ok := fm["tools"].([]interface{}); ok && len(toolsList) > 0 {
					for _, t := range toolsList {
						if s, ok := t.(string); ok {
							// Copilot-specific tool names
							if s == "execute" || s == "search" || s == "read" || s == "edit" || s == "todo" || s == "web" {
								return true
							}
							// VS Code specific tools
							if strings.HasPrefix(s, "vscode/") || strings.HasPrefix(s, "execute/") || strings.HasPrefix(s, "read/") || strings.HasPrefix(s, "search/") || strings.HasPrefix(s, "web/") {
								return true
							}
						}
					}
				}
			}
			// Has target field (Copilot-specific)
			if _, hasTarget := fm["target"]; hasTarget {
				return true
			}
		}
	}

	return false
}

// Parse extracts structure from a GitHub Copilot agent file.
func (p *copilotParser) Parse(content []byte) (*AgentAnalysis, error) {
	// Normalize CRLF
	normalized := bytes.ReplaceAll(content, []byte("\r\n"), []byte("\n"))

	fm, body, err := extractFrontmatter(normalized)
	if err != nil {
		fm = make(map[string]interface{})
		body = normalized
	}

	sections := parseSections(body)
	tools := extractTools(fm)

	var warnings []string
	// Check Copilot-specific: description has 1024-char limit
	if desc, ok := fm["description"].(string); ok && len(desc) > 1024 {
		warnings = append(warnings, fmt.Sprintf("description exceeds Copilot's 1024-char limit (%d chars)", len(desc)))
	}

	return &AgentAnalysis{
		Format:      "copilot",
		Frontmatter: fm,
		Sections:    sections,
		Tools:       tools,
		RawContent:  content,
		Warnings:    warnings,
	}, nil
}

// Validate checks that a rewritten file preserves the original structure.
func (p *copilotParser) Validate(original, rewritten []byte) error {
	// Reuse Claude's validation logic — same frontmatter + section checks
	c := &claudeParser{}
	return c.Validate(original, rewritten)
}
