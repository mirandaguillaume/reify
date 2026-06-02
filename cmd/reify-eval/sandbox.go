package main

import (
	"fmt"
	"strings"
)

// buildPrompt composes the agent prompt: the task, plus the formulation's
// rule text when present. For the `absent` control (rule == ""), no rule
// section is injected — the agent sees only the task.
func buildPrompt(f *Fixture, rule string) string {
	var b strings.Builder
	b.WriteString("Task:\n")
	b.WriteString(f.Task)
	b.WriteString("\n")
	if strings.TrimSpace(rule) != "" {
		b.WriteString("\nRules:\n")
		fmt.Fprintf(&b, "- %s\n", rule)
	}
	return b.String()
}
