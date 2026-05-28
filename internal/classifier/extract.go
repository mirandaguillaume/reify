package classifier

import "strings"

// extractItems splits agent-file content into syntactic instruction units
// (bullets, numbered list entries, inline-code commands). It does NOT assign
// facets — that's the LLM's job in ClassifyLLM. Extraction is deterministic
// text parsing; classification of subjective prose is not.
func extractItems(content string) []Item {
	lines := strings.Split(content, "\n")
	var items []Item
	currentSection := ""

	for _, raw := range lines {
		line := strings.TrimSpace(raw)
		if line == "" {
			continue
		}

		if strings.HasPrefix(line, "#") {
			currentSection = strings.TrimLeft(line, "# ")
			continue
		}
		if strings.HasPrefix(line, "```") || strings.HasPrefix(line, "---") || strings.HasPrefix(line, "===") {
			continue
		}

		text := extractInstruction(line)
		if text == "" {
			continue
		}
		items = append(items, Item{Text: text, Section: currentSection})
	}

	return items
}

// extractInstruction returns the instruction text from a bullet, numbered
// list, or inline-code line — or empty string if the line is not one of those.
func extractInstruction(line string) string {
	for _, prefix := range []string{"- ", "* ", "+ "} {
		if strings.HasPrefix(line, prefix) {
			return strings.TrimSpace(line[len(prefix):])
		}
	}
	if len(line) > 2 && line[0] >= '0' && line[0] <= '9' {
		if idx := strings.Index(line, ". "); idx > 0 && idx <= 3 {
			return strings.TrimSpace(line[idx+2:])
		}
	}
	if strings.HasPrefix(line, "`") && strings.HasSuffix(line, "`") {
		return line
	}
	return ""
}
