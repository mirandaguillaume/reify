package copilot

import "strings"

var toolMap = map[string]string{
	"read_file":  "read",
	"read":       "read",
	"write_file": "edit",
	"write":      "edit",
	"edit_file":  "edit",
	"edit":       "edit",
	"grep":       "search",
	"search":     "search",
	"find":       "search",
	"glob":       "search",
	"bash":       "execute",
	"shell":      "execute",
	"exec":       "execute",
	"terminal":   "execute",
	"web_fetch":  "web",
	"http":       "web",
	"fetch":      "web",
	"web_search": "web",
	"todo":       "todo",
	"task":       "agent",
	"delegate":   "agent",
}

var canonicalOrder = []string{"read", "edit", "search", "execute", "web", "agent", "todo"}

// MapToolsToCopilot maps generic tool names to Copilot tool names.
func MapToolsToCopilot(tools []string) []string {
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

// InferCopilotToolsFromSecurity infers Copilot tools from security facet settings.
func InferCopilotToolsFromSecurity(filesystem, network string) []string {
	var tools []string
	if filesystem == "read-only" || filesystem == "read-write" || filesystem == "full" {
		tools = append(tools, "read", "search")
	}
	if filesystem == "read-write" || filesystem == "full" {
		tools = append(tools, "edit")
	}
	if filesystem == "full" {
		tools = append(tools, "execute")
	}
	if network == "allowlist" || network == "full" {
		tools = append(tools, "web")
	}
	return tools
}

// MergeCopilotToolLists merges multiple tool lists, deduplicating and ordering canonically.
func MergeCopilotToolLists(lists ...[]string) []string {
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
