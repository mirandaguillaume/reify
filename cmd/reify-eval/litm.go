package main

import (
	"fmt"
	"strings"
)

// Lost-in-the-middle (LITM) prompt assembly. The experiment holds the
// fixture and formulation constant and varies WHERE the rule sits inside a
// long, plausible context (a fake CLAUDE.md). It measures whether a rule
// buried mid-context is obeyed less than one at the head or tail.
//
// Position is a fraction in [0,1]: 0.0 = rule before all filler (head),
// 0.5 = middle, 1.0 = rule after all filler, just before the task (tail).

// fillerSection is one plausible CLAUDE.md block. Kept generic and
// non-conflicting with any test intention so the only "live" instruction
// is the rule under test.
var fillerSections = []string{
	"## Project context\nThis is a Go service. The module layout follows standard cmd/ and internal/ separation. Prefer small, focused packages.",
	"## Build & test\nUse `go build ./...` to compile and `go test ./...` to run the suite. CI runs vet and the race detector.",
	"## Conventions\nName things for what they do, not how. Keep functions short. Errors are wrapped with %w and context. Avoid global state.",
	"## Formatting\nAll code is gofmt'd. Imports are grouped stdlib / third-party / local. Comments are full sentences.",
	"## Architecture\nThe domain types live in pkg/ and are imported by internal/. The dependency direction is strictly one-way.",
	"## Logging\nStructured logging only. No fmt.Println in library code. Log at the boundary, not in the core.",
	"## Reviews\nEvery change is reviewed. Keep diffs small and focused. Explain non-obvious decisions in the commit message.",
	"## Testing style\nTable-driven tests with testify. Black-box where it clarifies the contract. One assertion concern per test.",
	"## Error handling\nReturn errors, don't panic in libraries. Validate inputs at the edge. Fail fast with clear messages.",
	"## Dependencies\nThe project values a small dependency tree. Review transitive deps. Pin versions in go.mod.",
}

// buildFiller returns approximately targetTokens worth of plausible
// CLAUDE.md text by repeating the section catalogue. Token count is
// approximated at ~4 chars/token (the project's standard estimate).
func buildFiller(targetTokens int) string {
	if targetTokens <= 0 {
		return ""
	}
	targetChars := targetTokens * 4
	var b strings.Builder
	i := 0
	for b.Len() < targetChars {
		b.WriteString(fillerSections[i%len(fillerSections)])
		b.WriteString("\n\n")
		i++
	}
	return b.String()
}

// buildPromptLITM assembles the agent prompt with the rule spliced into
// filler at a fractional depth. The task always comes last (closest to
// the user turn). With rule == "" (absent control) no rule is injected
// but the filler is still present, so the absent baseline shares the same
// context length as the rule conditions.
func buildPromptLITM(f *Fixture, rule string, fillerTokens int, position float64) string {
	filler := buildFiller(fillerTokens)
	blocks := strings.Split(strings.TrimSpace(filler), "\n\n")
	if len(blocks) == 1 && blocks[0] == "" {
		blocks = nil
	}

	ruleBlock := ""
	if strings.TrimSpace(rule) != "" {
		ruleBlock = "## Rules\n- " + rule
	}

	// Splice the rule block at the fractional position among the filler
	// blocks. position 0 -> before all; 1 -> after all.
	var b strings.Builder
	b.WriteString("# Project guidelines\n\n")
	if ruleBlock != "" {
		idx := int(float64(len(blocks)) * position)
		if idx < 0 {
			idx = 0
		}
		if idx > len(blocks) {
			idx = len(blocks)
		}
		for i := 0; i < idx; i++ {
			b.WriteString(blocks[i])
			b.WriteString("\n\n")
		}
		b.WriteString(ruleBlock)
		b.WriteString("\n\n")
		for i := idx; i < len(blocks); i++ {
			b.WriteString(blocks[i])
			b.WriteString("\n\n")
		}
	} else {
		for _, bl := range blocks {
			b.WriteString(bl)
			b.WriteString("\n\n")
		}
	}

	b.WriteString("---\n\nTask:\n")
	b.WriteString(f.Task)
	b.WriteString("\n")
	return b.String()
}

// litmPositionName maps a fractional position to a short label for output.
func litmPositionName(position float64) string {
	switch {
	case position <= 0.0:
		return "head"
	case position >= 1.0:
		return "tail"
	default:
		return fmt.Sprintf("mid-%.0f%%", position*100)
	}
}
