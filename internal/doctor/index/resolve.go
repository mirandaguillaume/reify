// Package index resolves AGENTS.md index files that reference specialized agent files.
// When Doctor encounters an index with markdown links to .agents/*.md files,
// it resolves each reference, analyzes the linked files, and produces an aggregate score.
package index

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/mirandaguillaume/reify/internal/doctor/llmutil"
	"github.com/mirandaguillaume/reify/internal/doctor/parser"
)

// linkRe matches markdown links to .agents/ files: [Title](.agents/file.md) or [Title](path/to/file.md)
var linkRe = regexp.MustCompile(`\[([^\]]+)\]\(([^\)]+\.md)\)`)

// ResolvedFile holds a resolved reference from an index file.
type ResolvedFile struct {
	Path     string                 // relative path from index
	Title    string                 // link title text
	Content  []byte                 // file content (nil if missing)
	Analysis *parser.AgentAnalysis  // parsed analysis (nil if error)
	Error    error                  // parse or read error
	Missing  bool                   // true if file doesn't exist
}

// ResolveIndex parses an index file and resolves all markdown link references.
// basePath is the directory containing the index file.
func ResolveIndex(indexContent []byte, basePath string) []ResolvedFile {
	matches := linkRe.FindAllStringSubmatch(string(indexContent), -1)
	if len(matches) == 0 {
		return nil
	}

	var resolved []ResolvedFile
	seen := make(map[string]bool) // dedup

	for _, m := range matches {
		title := m[1]
		relPath := m[2]

		// Skip external URLs
		if len(relPath) > 4 && (relPath[:4] == "http" || relPath[:2] == "//") {
			continue
		}

		fullPath := filepath.Clean(filepath.Join(basePath, relPath))
		// Prevent path traversal outside base directory
		cleanBase := filepath.Clean(basePath) + string(filepath.Separator)
		if !strings.HasPrefix(fullPath, cleanBase) && fullPath != filepath.Clean(basePath) {
			continue
		}
		if seen[fullPath] {
			continue
		}
		seen[fullPath] = true

		rf := ResolvedFile{
			Path:  relPath,
			Title: title,
		}

		content, err := os.ReadFile(fullPath)
		if err != nil {
			rf.Missing = true
			rf.Error = fmt.Errorf("referenced file not found: %s", relPath)
			resolved = append(resolved, rf)
			continue
		}
		rf.Content = content

		// Try to parse with Claude format (most .agents/ files are markdown)
		p, err := parser.Get("claude")
		if err != nil {
			// Fallback: create a basic analysis from raw content
			rf.Analysis = &parser.AgentAnalysis{
				Format:     "markdown",
				RawContent: content,
				Sections:   parseSectionsSimple(content),
			}
		} else {
			analysis, err := p.Parse(content)
			if err != nil {
				// Fallback to basic parsing
				rf.Analysis = &parser.AgentAnalysis{
					Format:     "markdown",
					RawContent: content,
					Sections:   parseSectionsSimple(content),
				}
			} else {
				rf.Analysis = analysis
			}
		}

		resolved = append(resolved, rf)
	}

	return resolved
}

// IsIndex returns true if the content looks like an index file (has markdown links to .md files).
func IsIndex(content []byte) bool {
	matches := linkRe.FindAllString(string(content), 3)
	return len(matches) >= 2 // at least 2 links to be considered an index
}

// MissingFiles returns HIGH findings for all missing referenced files.
func MissingFiles(resolved []ResolvedFile) []llmutil.Finding {
	var findings []llmutil.Finding
	for _, rf := range resolved {
		if rf.Missing {
			findings = append(findings, llmutil.Finding{
				Category:             "version_drift",
				Issue:                fmt.Sprintf("Referenced file not found: %s", rf.Path),
				Confidence:           "high",
				CitationID:           "version_drift",
				CurrentState:         fmt.Sprintf("Index references %s but file does not exist", rf.Path),
				SuggestedImprovement: fmt.Sprintf("Create %s or remove the reference from the index", rf.Path),
			})
		}
	}
	return findings
}

// parseSectionsSimple extracts markdown sections from raw content.
func parseSectionsSimple(content []byte) []parser.Section {
	var sections []parser.Section
	lines := strings.Split(string(content), "\n")
	var current *parser.Section

	for _, line := range lines {
		if len(line) > 0 && line[0] == '#' {
			level := 0
			for _, c := range line {
				if c == '#' {
					level++
				} else {
					break
				}
			}
			header := strings.TrimSpace(line[level:])
			if current != nil {
				sections = append(sections, *current)
			}
			current = &parser.Section{Header: header, Level: level}
		} else if current != nil {
			current.Content += line + "\n"
		}
	}
	if current != nil {
		sections = append(sections, *current)
	}
	return sections
}
