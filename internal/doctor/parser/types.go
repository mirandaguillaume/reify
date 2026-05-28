// Package parser provides format detection and parsing for agent definition files.
package parser

// AgentAnalysis holds the parsed structure of an agent definition file.
type AgentAnalysis struct {
	Format      string                 // "claude", "copilot", "reify", etc.
	Frontmatter map[string]interface{} // Parsed YAML frontmatter fields
	Sections    []Section              // Markdown sections split by headers
	Tools       []string               // Tool names extracted from frontmatter or body
	RawContent  []byte                 // Original file content
	Warnings    []string               // Parse-time warnings (e.g., description exceeds limit)
}

// Section represents a markdown section delimited by a header.
type Section struct {
	Header  string // Full header text (e.g., "## Rules")
	Content string // Content between this header and the next
	Level   int    // Header level: 1 for #, 2 for ##, 3 for ###, etc.
}
