package analysis

// Analyze is the top-level entry point: takes a raw unified diff string,
// parses it, extracts AST context for each file, scans for patterns,
// and returns a structured markdown string suitable for LLM consumption.
//
// The sourceFiles map provides full source code for files in the diff
// (key = file path, value = file content). If nil, AST analysis is skipped
// and the result is pattern-only.
func Analyze(diff string, sourceFiles map[string][]byte) (string, error) {
	files := ParseDiff(diff)
	if len(files) == 0 {
		return "", nil
	}

	// Parse AST for each file that has source code available.
	asts := make(map[string]*ASTContext)
	for i, f := range files {
		var code []byte
		if sourceFiles != nil {
			code = sourceFiles[f.Path]
		}

		// If no full source provided, reconstruct from diff (added lines only).
		if code == nil {
			code = reconstructFromDiff(f)
		}

		if len(code) == 0 || f.Language == "" {
			continue
		}

		ast, err := ParseAST(code, f.Language)
		if err != nil {
			continue // skip files with parse errors
		}
		if ast == nil {
			continue // unsupported language
		}

		ast.File = f.Path
		OverlayHunks(ast, files[i].Hunks)
		asts[f.Path] = ast
	}

	// Scan for suspicious patterns.
	patterns := ScanPatterns(files, asts)
	patterns = DeduplicateHits(patterns)

	// Render structured output.
	return Render(files, asts, patterns), nil
}

// reconstructFromDiff builds pseudo-source from added lines in a diff.
// This is a best-effort approach when full source is not available.
func reconstructFromDiff(f DiffFile) []byte {
	var lines []string
	for _, h := range f.Hunks {
		for _, l := range h.Lines {
			if l.Kind == LineAdded || l.Kind == LineContext {
				lines = append(lines, l.Content)
			}
		}
	}
	if len(lines) == 0 {
		return nil
	}
	return []byte(joinLines(lines))
}

func joinLines(lines []string) string {
	result := ""
	for i, l := range lines {
		result += l
		if i < len(lines)-1 {
			result += "\n"
		}
	}
	return result
}
