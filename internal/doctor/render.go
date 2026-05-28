// Package doctor provides the doctor CLI command and output rendering.
package doctor

import (
	"fmt"
	"os"
	"sort"
	"strings"

	"github.com/fatih/color"
	"github.com/mirandaguillaume/reify/internal/doctor/llmutil"
	"github.com/mirandaguillaume/reify/internal/doctor/registry"
)

// IsTTY returns true if stdout is a terminal.
func IsTTY() bool {
	fi, err := os.Stdout.Stat()
	if err != nil {
		return false
	}
	return fi.Mode()&os.ModeCharDevice != 0
}

// RenderFindings prints findings to stdout, adapting format for TTY vs pipe.
// If reg is non-nil, citation details are resolved and displayed.
func RenderFindings(findings []llmutil.Finding, filePath string, tty bool, reg *registry.Registry) {
	if len(findings) == 0 {
		if tty {
			color.Green("✓ No issues found in %s", filePath)
		} else {
			fmt.Printf("No issues found in %s\n", filePath)
		}
		return
	}

	if tty {
		renderTTY(findings, filePath, reg)
	} else {
		renderPlain(findings, filePath, reg)
	}
}

func renderTTY(findings []llmutil.Finding, filePath string, reg *registry.Registry) {
	bold := color.New(color.Bold)
	bold.Printf("\n%d recommendations for %s:\n\n", len(findings), filePath)

	for _, f := range findings {
		badge := confidenceBadgeTTY(effectiveSeverity(f))
		fmt.Printf("%s %s\n", badge, color.New(color.Bold).Sprint(f.Issue))

		if f.CurrentState != "" {
			fmt.Printf("  Now: %s\n", f.CurrentState)
		}
		if f.SuggestedImprovement != "" {
			fmt.Printf("  Fix: %s\n", color.CyanString(f.SuggestedImprovement))
		}
		if cit := resolveCitation(f, reg); cit != "" {
			fmt.Printf("  Ref: %s\n", color.WhiteString(cit))
		}
		fmt.Println()
	}
}

func renderPlain(findings []llmutil.Finding, filePath string, reg *registry.Registry) {
	fmt.Printf("%d recommendations for %s:\n", len(findings), filePath)

	for _, f := range findings {
		badge := confidenceBadgePlain(effectiveSeverity(f))
		fmt.Printf("%s %s\n", badge, f.Issue)

		if f.CurrentState != "" {
			fmt.Printf("  Now: %s\n", f.CurrentState)
		}
		if f.SuggestedImprovement != "" {
			fmt.Printf("  Fix: %s\n", f.SuggestedImprovement)
		}
		if cit := resolveCitation(f, reg); cit != "" {
			fmt.Printf("  Ref: %s\n", cit)
		}
	}
}

// resolveCitation looks up a finding's citation_id in the registry and returns
// a formatted citation string. Returns "" if no match.
func resolveCitation(f llmutil.Finding, reg *registry.Registry) string {
	if reg == nil {
		return ""
	}
	id := f.CitationID
	if id == "" {
		id = f.Category
	}
	rec, ok := reg.Get(id)
	if !ok {
		return ""
	}
	return fmt.Sprintf("%s — %s (%s)", rec.Citation, rec.Paper, rec.URL)
}

// effectiveSeverity returns Severity if set (static checks), else Confidence (LLM findings).
func effectiveSeverity(f llmutil.Finding) string {
	if f.Severity != "" {
		return f.Severity
	}
	return f.Confidence
}

func confidenceBadgeTTY(confidence string) string {
	switch strings.ToLower(confidence) {
	case "high":
		return color.RedString("[HIGH]")
	case "moderate":
		return color.YellowString("[MOD] ")
	case "low":
		return color.BlueString("[LOW] ")
	default:
		return fmt.Sprintf("[%s]", strings.ToUpper(confidence))
	}
}

func confidenceBadgePlain(confidence string) string {
	switch strings.ToLower(confidence) {
	case "high":
		return "HIGH:"
	case "moderate":
		return "MOD: "
	case "low":
		return "LOW: "
	default:
		return strings.ToUpper(confidence) + ":"
	}
}

// StructuralResult holds the result of static checks for rendering.
type StructuralResult struct {
	Passed  int
	Total   int
	Missing []string // categories of missing sections
}

// ComputeStructural counts pass/fail from static findings.
// Includes section-presence checks (Missing) and secret scanning (any security finding = fail).
//
// Secret detection delegates to isSecretFinding (gate.go) so this routine and
// gate.Evaluate stay in sync. Story 4-0 AC #3 — single source of truth for
// secret classification.
func ComputeStructural(findings []llmutil.Finding, totalSections int) StructuralResult {
	missing := make(map[string]bool)
	secretsFound := 0
	for _, f := range findings {
		if strings.Contains(f.Issue, "Missing") {
			missing[f.Category] = true
		}
		if isSecretFinding(f) {
			secretsFound++
		}
	}

	// Total = sections + 1 (secrets clean check)
	total := totalSections + 1
	passed := totalSections - len(missing)
	if secretsFound == 0 {
		passed++ // secrets check passes
	}

	result := StructuralResult{
		Total:  total,
		Passed: passed,
	}
	for cat := range missing {
		result.Missing = append(result.Missing, cat)
	}
	if secretsFound > 0 {
		result.Missing = append(result.Missing, fmt.Sprintf("secrets_clean (%d secrets found)", secretsFound))
	}
	sort.Strings(result.Missing)
	return result
}

// RenderStructural prints the structural score before LLM analysis.
func RenderStructural(result StructuralResult, filePath string, tty bool) {
	pct := 0
	if result.Total > 0 {
		pct = result.Passed * 100 / result.Total
	}

	if tty {
		bold := color.New(color.Bold)
		bold.Printf("\nStructural: %d/%d passed (%d%%)\n", result.Passed, result.Total, pct)
		if len(result.Missing) > 0 {
			for _, m := range result.Missing {
				fmt.Printf("  %s %s\n", color.RedString("x"), m)
			}
		}
	} else {
		fmt.Printf("Structural: %d/%d passed (%d%%)\n", result.Passed, result.Total, pct)
		if len(result.Missing) > 0 {
			for _, m := range result.Missing {
				fmt.Printf("  MISSING: %s\n", m)
			}
		}
	}
}

// RenderAntiPatterns prints agent smell findings in a separate section.
func RenderAntiPatterns(findings []llmutil.Finding, tty bool) {
	var smells []llmutil.Finding
	for _, f := range findings {
		if f.Category == "agent_smell" {
			smells = append(smells, f)
		}
	}
	if len(smells) == 0 {
		return
	}

	if tty {
		bold := color.New(color.Bold)
		bold.Printf("\nAnti-patterns detected:\n\n")
	} else {
		fmt.Printf("\nAnti-patterns detected:\n")
	}
	for _, f := range smells {
		if tty {
			fmt.Printf("  %s %s\n", color.YellowString("!"), color.New(color.Bold).Sprint(f.Issue))
			fmt.Printf("    %s\n", color.CyanString(f.SuggestedImprovement))
		} else {
			fmt.Printf("  ! %s\n", f.Issue)
			fmt.Printf("    %s\n", f.SuggestedImprovement)
		}
		fmt.Println()
	}
}

// RenderConsistency prints cross-file consistency findings in a separate section.
func RenderConsistency(findings []llmutil.Finding, tty bool, reg *registry.Registry) {
	if len(findings) == 0 {
		return
	}
	if tty {
		bold := color.New(color.Bold)
		bold.Printf("\n── Cross-File Consistency ──\n\n")
	} else {
		fmt.Printf("\n-- Cross-File Consistency --\n")
	}
	for _, f := range findings {
		sev := effectiveSeverity(f)
		badge := confidenceBadgePlain(sev)
		if tty {
			badge = confidenceBadgeTTY(sev)
		}
		fmt.Printf("%s %s\n", badge, f.Issue)
		if f.CurrentState != "" {
			fmt.Printf("  Evidence: %s\n", f.CurrentState)
		}
		if f.SuggestedImprovement != "" {
			if tty {
				fmt.Printf("  Fix: %s\n", color.CyanString(f.SuggestedImprovement))
			} else {
				fmt.Printf("  Fix: %s\n", f.SuggestedImprovement)
			}
		}
		fmt.Println()
	}
}

// RenderAggregate prints the aggregate summary after individual file results.
func RenderAggregate(report *AggregateReport, tty bool) {
	if tty {
		renderAggregateTTY(report)
	} else {
		renderAggregatePlain(report)
	}
}

func renderAggregateTTY(r *AggregateReport) {
	bold := color.New(color.Bold)
	bold.Printf("\n── Aggregate Summary ──\n\n")

	fmt.Printf("  Files analyzed: %d/%d", r.AnalyzedFiles, r.TotalFiles)
	if r.FailedFiles > 0 {
		fmt.Printf(" (%s failed)", color.RedString("%d", r.FailedFiles))
	}
	fmt.Println()

	fmt.Printf("  Total findings: %d\n", r.TotalFindings)

	if len(r.ByCategory) > 0 {
		fmt.Printf("  By category:    ")
		cats := sortedCategories(r.ByCategory)
		for i, cat := range cats {
			if i > 0 {
				fmt.Printf(", ")
			}
			fmt.Printf("%s=%d", cat, r.ByCategory[cat])
		}
		fmt.Println()
	}

	if len(r.FilesNoGuardrails) > 0 {
		fmt.Printf("  %s %d files with no guardrails findings\n",
			color.YellowString("[WARN]"), len(r.FilesNoGuardrails))
	}
	if len(r.FilesNoSecurity) > 0 {
		fmt.Printf("  %s %d files with no security findings\n",
			color.YellowString("[WARN]"), len(r.FilesNoSecurity))
	}
	fmt.Println()
}

func renderAggregatePlain(r *AggregateReport) {
	fmt.Printf("\n-- Aggregate Summary --\n")
	fmt.Printf("Files analyzed: %d/%d", r.AnalyzedFiles, r.TotalFiles)
	if r.FailedFiles > 0 {
		fmt.Printf(" (%d failed)", r.FailedFiles)
	}
	fmt.Println()

	fmt.Printf("Total findings: %d\n", r.TotalFindings)

	if len(r.ByCategory) > 0 {
		fmt.Printf("By category: ")
		cats := sortedCategories(r.ByCategory)
		for i, cat := range cats {
			if i > 0 {
				fmt.Printf(", ")
			}
			fmt.Printf("%s=%d", cat, r.ByCategory[cat])
		}
		fmt.Println()
	}

	if len(r.FilesNoGuardrails) > 0 {
		fmt.Printf("WARN: %d files with no guardrails findings\n", len(r.FilesNoGuardrails))
	}
	if len(r.FilesNoSecurity) > 0 {
		fmt.Printf("WARN: %d files with no security findings\n", len(r.FilesNoSecurity))
	}
}

func sortedCategories(m map[string]int) []string {
	cats := make([]string, 0, len(m))
	for k := range m {
		cats = append(cats, k)
	}
	sort.Strings(cats)
	return cats
}
