package claude

import "strings"

var toolMap = map[string]string{
	"read_file":  "Read",
	"read":       "Read",
	"write_file": "Write",
	"write":      "Write",
	"edit_file":  "Edit",
	"edit":       "Edit",
	"grep":       "Grep",
	"search":     "Glob",
	"find":       "Glob",
	"glob":       "Glob",
	"bash":       "Bash",
	"shell":      "Bash",
	"exec":       "Bash",
	"terminal":   "Bash",
	"web_fetch":  "WebFetch",
	"http":       "WebFetch",
	"fetch":      "WebFetch",
	"web_search": "WebSearch",
	"todo":       "TodoWrite",
	"task":       "Task",
	"delegate":   "Task",
}

var canonicalOrder = []string{
	"Glob", "Grep", "Read", "Write", "Edit",
	"Bash", "WebFetch", "WebSearch", "TodoWrite", "Task",
}

// MapToolsToClaude maps generic tool names to Claude Code tool names.
func MapToolsToClaude(tools []string) []string {
	seen := map[string]bool{}
	var result []string
	for _, t := range tools {
		if mapped, ok := toolMap[strings.ToLower(t)]; ok && !seen[mapped] {
			result = append(result, mapped)
			seen[mapped] = true
		}
	}
	return result
}

// InferToolsFromSecurity infers Claude tools from security facet settings.
func InferToolsFromSecurity(filesystem string, network string) []string {
	var tools []string

	if filesystem == "read-only" || filesystem == "read-write" || filesystem == "full" {
		tools = append(tools, "Read", "Glob", "Grep")
	}
	if filesystem == "read-write" || filesystem == "full" {
		tools = append(tools, "Write", "Edit")
	}
	if filesystem == "full" {
		tools = append(tools, "Bash")
	}
	if network == "allowlist" || network == "full" {
		tools = append(tools, "WebFetch")
	}
	if network == "full" {
		tools = append(tools, "WebSearch")
	}
	return tools
}

// MergeToolLists merges multiple tool lists, deduplicating and ordering canonically.
func MergeToolLists(lists ...[]string) []string {
	seen := map[string]bool{}
	for _, list := range lists {
		for _, t := range list {
			seen[t] = true
		}
	}
	var ordered []string
	for _, t := range canonicalOrder {
		if seen[t] {
			ordered = append(ordered, t)
			delete(seen, t)
		}
	}
	// Any extra tools not in canonical order
	for t := range seen {
		ordered = append(ordered, t)
	}
	return ordered
}
