package analysis

import (
	"fmt"
	"strings"
)

// Render combines parsed diffs, AST contexts, and pattern hits into
// a structured markdown string for LLM consumption.
func Render(files []DiffFile, asts map[string]*ASTContext, patterns []PatternHit) string {
	var sb strings.Builder

	// Group patterns by file for fast lookup.
	patternsByFile := make(map[string][]PatternHit)
	for _, p := range patterns {
		patternsByFile[p.File] = append(patternsByFile[p.File], p)
	}

	for _, f := range files {
		langLabel := f.Language
		if langLabel == "" {
			langLabel = "unknown"
		}
		sb.WriteString(fmt.Sprintf("## File: %s (%s)\n\n", f.Path, langLabel))

		ast := asts[f.Path]

		// Changed functions section
		if ast != nil {
			var changedSyms []ASTSymbol
			for _, sym := range ast.Symbols {
				if sym.Changed {
					changedSyms = append(changedSyms, sym)
				}
			}
			if len(changedSyms) > 0 {
				sb.WriteString("### Changed Symbols\n")
				for _, sym := range changedSyms {
					sb.WriteString(fmt.Sprintf("- `%s %s` (lines %d-%d) — MODIFIED\n",
						sym.Kind, sym.Name, sym.StartLine, sym.EndLine))
				}
				sb.WriteString("\n")

				// AST context: show body of changed symbols (truncated)
				sb.WriteString("### AST Context\n")
				for _, sym := range changedSyms {
					body := sym.Body
					lines := strings.Split(body, "\n")
					if len(lines) > 30 {
						body = strings.Join(lines[:15], "\n") +
							"\n    // ... (" + fmt.Sprintf("%d", len(lines)-30) + " lines omitted)\n" +
							strings.Join(lines[len(lines)-15:], "\n")
					}
					sb.WriteString("```\n")
					sb.WriteString(body)
					sb.WriteString("\n```\n\n")
				}
			}
		}

		// Flagged patterns section
		filePatterns := patternsByFile[f.Path]
		if len(filePatterns) > 0 {
			sb.WriteString("### Flagged Patterns\n")
			for _, p := range filePatterns {
				sb.WriteString(fmt.Sprintf("- [%s] line %d: `%s` (severity: %s)\n",
					p.Category, p.Line, p.Snippet, p.Severity))
				if p.Rule != "" {
					sb.WriteString(fmt.Sprintf("  Rule: %s\n", p.Rule))
				}
			}
			sb.WriteString("\n")
		}

		// If no AST and no patterns, show raw diff summary
		if ast == nil && len(filePatterns) == 0 {
			added, removed := countChanges(f)
			sb.WriteString(fmt.Sprintf("_No AST available. Diff: +%d/-%d lines._\n\n", added, removed))
		}
	}

	return sb.String()
}

// countChanges counts added and removed lines in a DiffFile.
func countChanges(f DiffFile) (added, removed int) {
	for _, h := range f.Hunks {
		for _, l := range h.Lines {
			switch l.Kind {
			case LineAdded:
				added++
			case LineRemoved:
				removed++
			}
		}
	}
	return
}
