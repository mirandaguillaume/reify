package enricher

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/mirandaguillaume/reify/internal/scanner"
)

// RenderIndex produces a compact pipe-delimited codebase index (~1-2KB).
func RenderIndex(ctx *scanner.CodebaseContext) string {
	var b strings.Builder
	b.WriteString("# Codebase Index\n")
	b.WriteString("|root: ./\n")
	b.WriteString("|IMPORTANT: Read relevant files before making assumptions\n")

	for _, entry := range ctx.Structure {
		b.WriteString("|")
		b.WriteString(entry.Path)
		b.WriteString(":{")
		b.WriteString(strings.Join(entry.Files, ","))
		b.WriteString("}\n")
	}
	return b.String()
}

// maxDirectDeps caps the number of direct dependencies shown in stack.md.
const maxDirectDeps = 15

// RenderStack produces a compact stack summary.
// Shows languages + direct deps only (dev deps excluded to save space).
func RenderStack(ctx *scanner.CodebaseContext) string {
	var b strings.Builder
	b.WriteString("# Stack\n")

	for _, lang := range ctx.Stack.Languages {
		if lang.Percentage < 1 {
			continue // skip languages with <1% share
		}
		b.WriteString(fmt.Sprintf("|%s (%.0f%% of source, %d files)\n", lang.Name, lang.Percentage, lang.FileCount))
	}

	directCount := 0
	for _, dep := range ctx.Stack.Deps {
		if dep.Kind == "runtime" || dep.Kind == "dev" {
			continue
		}
		if directCount >= maxDirectDeps {
			b.WriteString("|...\n")
			break
		}
		b.WriteString(fmt.Sprintf("|%s %s\n", dep.Name, dep.Version))
		directCount++
	}

	return b.String()
}

// symbolGroups groups symbols by directory, preserving insertion order.
func symbolGroups(symbols []scanner.SymbolEntry) (map[string][]scanner.SymbolEntry, []string) {
	groups := map[string][]scanner.SymbolEntry{}
	var order []string
	for _, sym := range symbols {
		dir := filepath.Dir(sym.File)
		if _, seen := groups[dir]; !seen {
			order = append(order, dir)
		}
		groups[dir] = append(groups[dir], sym)
	}
	return groups, order
}

// RenderSymbolsIndex produces a compact summary: package → key types (structs, interfaces) only.
// This is the "map" an agent reads first to understand the architecture.
func RenderSymbolsIndex(ctx *scanner.CodebaseContext) string {
	if len(ctx.Symbols) == 0 {
		return ""
	}

	groups, order := symbolGroups(ctx.Symbols)

	var b strings.Builder
	b.WriteString("# Key Symbols\n")
	b.WriteString("|IMPORTANT: Read `symbols/<pkg>.md` for full details of a package\n")

	for _, dir := range order {
		syms := groups[dir]
		// Index only shows structs, interfaces, and top-level funcs (not methods/consts/vars).
		var keyNames []string
		for _, s := range syms {
			switch s.Kind {
			case "struct", "interface":
				keyNames = append(keyNames, fmt.Sprintf("%s(%s)", s.Name, s.Kind))
			case "func":
				keyNames = append(keyNames, fmt.Sprintf("%s(func)", s.Name))
			}
		}
		if len(keyNames) == 0 {
			continue
		}
		b.WriteString("|")
		b.WriteString(dir)
		b.WriteString(":")
		b.WriteString(strings.Join(keyNames, ","))
		b.WriteString("\n")
	}

	return b.String()
}

// RenderSymbolShard produces the full symbol list for one package directory.
func RenderSymbolShard(dir string, symbols []scanner.SymbolEntry) string {
	var b strings.Builder
	b.WriteString(fmt.Sprintf("# %s\n", dir))

	// Group by kind for readability.
	kindOrder := []string{"interface", "struct", "type", "func", "method", "const", "var"}
	byKind := map[string][]string{}
	for _, s := range symbols {
		byKind[s.Kind] = append(byKind[s.Kind], s.Name)
	}

	for _, kind := range kindOrder {
		names := byKind[kind]
		if len(names) == 0 {
			continue
		}
		b.WriteString(fmt.Sprintf("|%s: %s\n", kind, strings.Join(names, ", ")))
	}

	return b.String()
}

// SymbolShardFilename converts a directory path to a flat filename.
// "pkg/model" → "pkg-model.md", "." → "root.md"
func SymbolShardFilename(dir string) string {
	if dir == "." {
		return "root.md"
	}
	return strings.ReplaceAll(dir, "/", "-") + ".md"
}

// RenderPointer produces 2-3 lines appended to SKILL.md in index mode.
func RenderPointer(ctx *scanner.CodebaseContext, contextDir string) string {
	var b strings.Builder
	b.WriteString("\n## Codebase Context\n\n")
	b.WriteString(fmt.Sprintf("Read `%s/index.md` for project structure, then read relevant source files.\n", contextDir))
	additional := fmt.Sprintf("`%s/stack.md`", contextDir)
	if len(ctx.Symbols) > 0 {
		additional += fmt.Sprintf(" | `%s/symbols.md` (index) → `%s/symbols/<pkg>.md` (details)", contextDir, contextDir)
	}
	b.WriteString(fmt.Sprintf("Additional context: %s\n", additional))
	return b.String()
}

// RenderInline produces sections inlined directly into SKILL.md (full mode).
func RenderInline(ctx *scanner.CodebaseContext) string {
	var b strings.Builder

	b.WriteString("\n## Codebase Context\n\n")

	// Structure
	b.WriteString("### Project Structure\n\n")
	b.WriteString("```\n")
	for _, entry := range ctx.Structure {
		b.WriteString(entry.Path + "/\n")
		for _, f := range entry.Files {
			b.WriteString("  " + f + "\n")
		}
	}
	b.WriteString("```\n\n")

	// Stack
	b.WriteString("### Stack\n\n")
	for _, lang := range ctx.Stack.Languages {
		if lang.Percentage < 1 {
			continue
		}
		b.WriteString(fmt.Sprintf("- **%s** — %.0f%% of source (%d files)\n", lang.Name, lang.Percentage, lang.FileCount))
	}
	b.WriteString("\n")

	// Only show direct deps (skip runtime and dev).
	var directDeps []scanner.DepInfo
	for _, dep := range ctx.Stack.Deps {
		if dep.Kind != "runtime" && dep.Kind != "dev" {
			directDeps = append(directDeps, dep)
		}
	}
	if len(directDeps) > 0 {
		b.WriteString("### Dependencies\n\n")
		for i, dep := range directDeps {
			if i >= maxDirectDeps {
				b.WriteString("- ...\n")
				break
			}
			b.WriteString(fmt.Sprintf("- %s %s\n", dep.Name, dep.Version))
		}
		b.WriteString("\n")
	}

	if len(ctx.Commands) > 0 {
		b.WriteString("### Commands\n\n")
		for _, cmd := range ctx.Commands {
			b.WriteString(fmt.Sprintf("- **%s**: `%s` (from %s)\n", cmd.Name, cmd.Command, cmd.Source))
		}
		b.WriteString("\n")
	}

	if len(ctx.Symbols) > 0 {
		b.WriteString("### Key Symbols\n\n")
		groups, order := symbolGroups(ctx.Symbols)
		for _, dir := range order {
			// Inline mode: show structs, interfaces, funcs (skip methods/consts/vars).
			var items []string
			for _, sym := range groups[dir] {
				switch sym.Kind {
				case "struct", "interface", "func":
					items = append(items, fmt.Sprintf("`%s` (%s)", sym.Name, sym.Kind))
				}
			}
			if len(items) > 0 {
				b.WriteString(fmt.Sprintf("**%s**: %s\n\n", dir, strings.Join(items, ", ")))
			}
		}
	}

	return b.String()
}

// WriteContextFiles writes index.md and stack.md to the given directory.
func WriteContextFiles(ctx *scanner.CodebaseContext, contextDir string) error {
	if err := os.MkdirAll(contextDir, 0755); err != nil {
		return fmt.Errorf("creating context dir: %w", err)
	}

	indexContent := RenderIndex(ctx)
	if err := os.WriteFile(filepath.Join(contextDir, "index.md"), []byte(indexContent), 0644); err != nil {
		return fmt.Errorf("writing index.md: %w", err)
	}

	stackContent := RenderStack(ctx)
	if err := os.WriteFile(filepath.Join(contextDir, "stack.md"), []byte(stackContent), 0644); err != nil {
		return fmt.Errorf("writing stack.md: %w", err)
	}

	if len(ctx.Symbols) > 0 {
		// Write symbols index.
		indexContent := RenderSymbolsIndex(ctx)
		if err := os.WriteFile(filepath.Join(contextDir, "symbols.md"), []byte(indexContent), 0644); err != nil {
			return fmt.Errorf("writing symbols.md: %w", err)
		}

		// Write per-package shards.
		symbolsDir := filepath.Join(contextDir, "symbols")
		if err := os.MkdirAll(symbolsDir, 0755); err != nil {
			return fmt.Errorf("creating symbols dir: %w", err)
		}
		groups, order := symbolGroups(ctx.Symbols)
		for _, dir := range order {
			shard := RenderSymbolShard(dir, groups[dir])
			filename := SymbolShardFilename(dir)
			if err := os.WriteFile(filepath.Join(symbolsDir, filename), []byte(shard), 0644); err != nil {
				return fmt.Errorf("writing symbols/%s: %w", filename, err)
			}
		}
	}

	return nil
}
