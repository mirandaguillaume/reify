package analysis

import (
	"path/filepath"
	"strconv"
	"strings"
)

// LineKind indicates whether a diff line was added, removed, or is context.
type LineKind int

const (
	LineContext LineKind = iota
	LineAdded
	LineRemoved
)

// DiffLine represents a single line within a diff hunk.
type DiffLine struct {
	Kind    LineKind
	Content string
	NewLine int // line number in new file (0 if removed-only)
	OldLine int // line number in old file (0 if added-only)
}

// DiffHunk represents a contiguous block of changes in a file.
type DiffHunk struct {
	OldStart, OldCount int
	NewStart, NewCount int
	Header             string // the full @@ line
	Lines              []DiffLine
}

// DiffFile represents all changes in a single file.
type DiffFile struct {
	Path     string
	Language string
	Hunks    []DiffHunk
}

// langMap maps file extensions to language names.
var langMap = map[string]string{
	".go":    "go",
	".ts":    "typescript",
	".tsx":   "typescript",
	".js":    "javascript",
	".jsx":   "javascript",
	".py":    "python",
	".rs":    "rust",
	".java":  "java",
	".rb":    "ruby",
	".cs":    "csharp",
	".cpp":   "cpp",
	".c":     "c",
	".swift": "swift",
	".kt":    "kotlin",
	".php":   "php",
	".scala": "scala",
	".ex":    "elixir",
	".exs":   "elixir",
}

// DetectLanguage returns the language name for a file path, or "" if unknown.
func DetectLanguage(path string) string {
	return langMap[filepath.Ext(path)]
}

// ParseDiff parses a unified diff string into structured DiffFiles.
func ParseDiff(diff string) []DiffFile {
	lines := strings.Split(diff, "\n")
	var files []DiffFile
	var current *DiffFile
	var currentHunk *DiffHunk

	for i := 0; i < len(lines); i++ {
		line := lines[i]

		// New file header: "diff --git a/path b/path"
		if strings.HasPrefix(line, "diff --git ") {
			if current != nil {
				if currentHunk != nil {
					current.Hunks = append(current.Hunks, *currentHunk)
					currentHunk = nil
				}
				files = append(files, *current)
			}
			path := extractPath(line)
			current = &DiffFile{
				Path:     path,
				Language: DetectLanguage(path),
			}
			currentHunk = nil
			continue
		}

		// Skip binary files
		if strings.HasPrefix(line, "Binary files") {
			if current != nil {
				current = nil
				currentHunk = nil
			}
			continue
		}

		// Skip index, ---, +++ headers (but extract path from --- if needed)
		if strings.HasPrefix(line, "index ") ||
			strings.HasPrefix(line, "--- ") ||
			strings.HasPrefix(line, "+++ ") ||
			strings.HasPrefix(line, "new file mode") ||
			strings.HasPrefix(line, "deleted file mode") ||
			strings.HasPrefix(line, "old mode") ||
			strings.HasPrefix(line, "new mode") ||
			strings.HasPrefix(line, "similarity index") ||
			strings.HasPrefix(line, "rename from") ||
			strings.HasPrefix(line, "rename to") {
			continue
		}

		// Hunk header: "@@ -old,count +new,count @@"
		if strings.HasPrefix(line, "@@") && current != nil {
			if currentHunk != nil {
				current.Hunks = append(current.Hunks, *currentHunk)
			}
			h := parseHunkHeader(line)
			currentHunk = &h
			continue
		}

		// Diff content lines
		if currentHunk != nil && current != nil {
			if strings.HasPrefix(line, "+") {
				currentHunk.Lines = append(currentHunk.Lines, DiffLine{
					Kind:    LineAdded,
					Content: line[1:],
				})
			} else if strings.HasPrefix(line, "-") {
				currentHunk.Lines = append(currentHunk.Lines, DiffLine{
					Kind:    LineRemoved,
					Content: line[1:],
				})
			} else if strings.HasPrefix(line, " ") {
				currentHunk.Lines = append(currentHunk.Lines, DiffLine{
					Kind:    LineContext,
					Content: line[1:],
				})
			} else if line == `\ No newline at end of file` {
				// skip
			}
		}
	}

	// Flush last file/hunk
	if current != nil {
		if currentHunk != nil {
			current.Hunks = append(current.Hunks, *currentHunk)
		}
		files = append(files, *current)
	}

	// Assign line numbers to each hunk's lines
	for fi := range files {
		for hi := range files[fi].Hunks {
			assignLineNumbers(&files[fi].Hunks[hi])
		}
	}

	return files
}

// extractPath extracts the file path from "diff --git a/path b/path".
func extractPath(line string) string {
	// Format: "diff --git a/path b/path"
	parts := strings.SplitN(line, " b/", 2)
	if len(parts) == 2 {
		return parts[1]
	}
	// Fallback: try to extract from "a/path"
	line = strings.TrimPrefix(line, "diff --git ")
	parts = strings.SplitN(line, " ", 2)
	if len(parts) > 0 {
		return strings.TrimPrefix(parts[0], "a/")
	}
	return ""
}

// parseHunkHeader parses "@@ -old,count +new,count @@ optional context".
func parseHunkHeader(line string) DiffHunk {
	h := DiffHunk{Header: line}

	// Find the range info between @@ markers
	start := strings.Index(line, "@@")
	end := strings.Index(line[start+2:], "@@")
	if start < 0 || end < 0 {
		return h
	}
	rangeStr := strings.TrimSpace(line[start+2 : start+2+end])

	parts := strings.Fields(rangeStr)
	for _, p := range parts {
		if strings.HasPrefix(p, "-") {
			nums := strings.SplitN(p[1:], ",", 2)
			h.OldStart, _ = strconv.Atoi(nums[0])
			if len(nums) > 1 {
				h.OldCount, _ = strconv.Atoi(nums[1])
			} else {
				h.OldCount = 1
			}
		} else if strings.HasPrefix(p, "+") {
			nums := strings.SplitN(p[1:], ",", 2)
			h.NewStart, _ = strconv.Atoi(nums[0])
			if len(nums) > 1 {
				h.NewCount, _ = strconv.Atoi(nums[1])
			} else {
				h.NewCount = 1
			}
		}
	}

	return h
}

// assignLineNumbers fills in NewLine and OldLine for each DiffLine in a hunk.
func assignLineNumbers(h *DiffHunk) {
	oldLine := h.OldStart
	newLine := h.NewStart

	for i := range h.Lines {
		switch h.Lines[i].Kind {
		case LineContext:
			h.Lines[i].OldLine = oldLine
			h.Lines[i].NewLine = newLine
			oldLine++
			newLine++
		case LineAdded:
			h.Lines[i].NewLine = newLine
			newLine++
		case LineRemoved:
			h.Lines[i].OldLine = oldLine
			oldLine++
		}
	}
}
