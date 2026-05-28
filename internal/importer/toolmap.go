package importer

// reverseClaudeMap maps Claude Code tool names to generic Reify tool names.
var reverseClaudeMap = map[string]string{
	"Read":      "read_file",
	"Write":     "write_file",
	"Edit":      "edit_file",
	"Grep":      "grep",
	"Glob":      "search",
	"Bash":      "bash",
	"WebFetch":  "web_fetch",
	"WebSearch": "web_search",
	"TodoWrite": "todo",
	"Task":      "task",
}

// reverseCopilotMap maps GitHub Copilot tool names to generic Reify tool names.
var reverseCopilotMap = map[string]string{
	"read":    "read_file",
	"edit":    "write_file",
	"search":  "grep",
	"execute": "bash",
	"web":     "web_search",
}

// ReverseMapTools translates framework-specific tool names into generic Reify
// tool names. For FrameworkUnknown it tries both maps. Unmapped tools are
// silently dropped and the result is deduplicated.
func ReverseMapTools(tools []string, framework Framework) []string {
	seen := make(map[string]bool)
	var result []string

	add := func(generic string) {
		if !seen[generic] {
			seen[generic] = true
			result = append(result, generic)
		}
	}

	for _, tool := range tools {
		switch framework {
		case FrameworkClaude:
			if g, ok := reverseClaudeMap[tool]; ok {
				add(g)
			}
		case FrameworkCopilot:
			if g, ok := reverseCopilotMap[tool]; ok {
				add(g)
			}
		default:
			// Try both maps for unknown framework.
			if g, ok := reverseClaudeMap[tool]; ok {
				add(g)
			}
			if g, ok := reverseCopilotMap[tool]; ok {
				add(g)
			}
		}
	}
	return result
}
