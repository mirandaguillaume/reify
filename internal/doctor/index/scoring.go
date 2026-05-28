package index

import (
	"encoding/json"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/mirandaguillaume/reify/internal/doctor/llmutil"
	"github.com/mirandaguillaume/reify/internal/doctor/scaffold"
	"github.com/mirandaguillaume/reify/internal/doctor/static"
)

// FileScore holds the score for a single resolved file, filtered to its relevant categories.
type FileScore struct {
	Path       string
	Title      string
	Passed     int
	Total      int
	Missing    []string // categories this file should cover but doesn't
	Categories []string // categories this file is responsible for
}

// AggregateScore holds the union-based aggregate across all files.
type AggregateScore struct {
	Covered     int      // unique categories covered by at least one file
	Total       int      // total categories (15)
	Uncovered   []string // categories not covered by any file
	PerFile     []FileScore
}

// ScoreIndex analyzes each resolved file on its relevant categories and computes an aggregate.
func ScoreIndex(resolved []ResolvedFile, mode string) *AggregateScore {
	allCategories := allSectionCategories()
	coveredSet := make(map[string]bool)

	var perFile []FileScore

	for _, rf := range resolved {
		relevantCats := categoriesForFile(rf.Path)
		if len(relevantCats) == 0 {
			continue
		}

		fs := FileScore{
			Path:       rf.Path,
			Title:      rf.Title,
			Categories: relevantCats,
			Total:      len(relevantCats),
		}

		if rf.Missing {
			// All categories uncovered
			fs.Missing = relevantCats
			perFile = append(perFile, fs)
			continue
		}

		if rf.Analysis == nil {
			fs.Missing = relevantCats
			perFile = append(perFile, fs)
			continue
		}

		// Run static checks on this file
		findings := static.RunChecks(rf.Analysis, mode)

		// Check which of this file's categories are covered
		missingSet := findMissingCategories(findings, relevantCats)

		for _, cat := range relevantCats {
			if !missingSet[cat] {
				fs.Passed++
				coveredSet[cat] = true
			} else {
				fs.Missing = append(fs.Missing, cat)
			}
		}

		perFile = append(perFile, fs)
	}

	// Compute uncovered
	var uncovered []string
	for _, cat := range allCategories {
		if !coveredSet[cat] {
			uncovered = append(uncovered, cat)
		}
	}

	return &AggregateScore{
		Covered:   len(coveredSet),
		Total:     len(allCategories),
		Uncovered: uncovered,
		PerFile:   perFile,
	}
}

// findMissingCategories returns a set of categories that are missing based on findings.
func findMissingCategories(findings []llmutil.Finding, relevant []string) map[string]bool {
	missing := make(map[string]bool)
	for _, f := range findings {
		if strings.Contains(f.Issue, "Missing") {
			missing[f.Category] = true
		}
	}
	// Only return relevant categories
	result := make(map[string]bool)
	for _, cat := range relevant {
		if missing[cat] {
			result[cat] = true
		}
	}
	return result
}

// categoriesForFile returns the verifiable categories a specialized file is responsible for.
// Uses filepath.Base for exact matching (not substring).
func categoriesForFile(path string) []string {
	base := filepath.Base(path)
	verifiable := make(map[string]bool)
	for _, c := range verifiableCategories {
		verifiable[c] = true
	}
	for _, sf := range scaffold.DefaultFiles {
		if base == sf.Name {
			// Filter to only categories that static checks can verify
			var cats []string
			for _, c := range sf.Categories {
				if verifiable[c] {
					cats = append(cats, c)
				}
			}
			return cats
		}
	}
	return nil
}

// allSectionCategories returns the 15 section categories that static presence checks can verify.
// Only includes categories present in sectionMapping (presence.go), not scaffold-only categories.
var verifiableCategories = []string{
	"security", "guardrails", "testing", "examples", "error_handling",
	"build_commands", "architecture_hints", "identity", "output_format",
	"decision_authority", "workflow_triggers", "dependency_declaration",
	"memory_management", "goals", "constraints",
}

func allSectionCategories() []string {
	return verifiableCategories
}

// AggregateToJSON converts the aggregate score to JSON.
func AggregateToJSON(agg *AggregateScore, indexPath string) ([]byte, error) {
	type jsonFileScore struct {
		Path    string   `json:"path"`
		Title   string   `json:"title"`
		Passed  int      `json:"passed"`
		Total   int      `json:"total"`
		Missing []string `json:"missing"`
	}
	type jsonAgg struct {
		File      string          `json:"file"`
		Covered   int             `json:"covered"`
		Total     int             `json:"total"`
		Pct       int             `json:"percentage"`
		Uncovered []string        `json:"uncovered"`
		PerFile   []jsonFileScore `json:"per_file"`
	}
	pct := 0
	if agg.Total > 0 {
		pct = agg.Covered * 100 / agg.Total
	}
	out := jsonAgg{
		File: indexPath, Covered: agg.Covered, Total: agg.Total, Pct: pct,
		Uncovered: agg.Uncovered, PerFile: make([]jsonFileScore, 0, len(agg.PerFile)),
	}
	if out.Uncovered == nil {
		out.Uncovered = []string{}
	}
	for _, fs := range agg.PerFile {
		m := fs.Missing
		if m == nil {
			m = []string{}
		}
		out.PerFile = append(out.PerFile, jsonFileScore{
			Path: fs.Path, Title: fs.Title, Passed: fs.Passed, Total: fs.Total, Missing: m,
		})
	}
	return json.MarshalIndent(out, "", "  ")
}

// FormatAggregate formats the aggregate score for terminal display.
func FormatAggregate(agg *AggregateScore) string {
	var b strings.Builder
	pct := 0
	if agg.Total > 0 {
		pct = agg.Covered * 100 / agg.Total
	}

	b.WriteString(fmt.Sprintf("\nAggregate: %d/%d categories covered (%d%%)\n", agg.Covered, agg.Total, pct))

	for _, fs := range agg.PerFile {
		status := "✅"
		if fs.Passed < fs.Total {
			status = fmt.Sprintf("%d/%d", fs.Passed, fs.Total)
		}
		b.WriteString(fmt.Sprintf("  %s %s (%s)\n", status, fs.Path, fs.Title))
		for _, m := range fs.Missing {
			b.WriteString(fmt.Sprintf("    ❌ %s\n", m))
		}
	}

	if len(agg.Uncovered) > 0 {
		b.WriteString("\n  Still uncovered:\n")
		for _, u := range agg.Uncovered {
			b.WriteString(fmt.Sprintf("    ❌ %s\n", u))
		}
	}

	return b.String()
}
