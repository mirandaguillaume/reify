package cmd

import (
	"fmt"
	"os"
	"strings"

	"github.com/fatih/color"
	"github.com/mirandaguillaume/reify/internal/checker"
	"github.com/mirandaguillaume/reify/internal/classifier"
	"github.com/mirandaguillaume/reify/internal/discovery"
	"github.com/mirandaguillaume/reify/internal/llm"
	"github.com/spf13/cobra"
)

func init() {
	var providerFlag, modelFlag string
	var all, verbose bool
	var targets []string

	checkCmd := &cobra.Command{
		Use:   "check <file|directory>",
		Short: "Assess instruction following compliance risk per harness",
		Long: `Analyzes each instruction in an agent file and flags compliance risks
per AI coding harness (Claude Code, Copilot, Cursor).

Accepts a single file, an agent directory, or a repo root — all recognized
agent files are checked. An LLM is required for instruction extraction and
classification.

Risk levels are derived from documented factors — no invented percentages:
  - Negative framing: IFEval benchmark (Zhou et al. 2023)
  - Middle position:  Liu et al. 2023 "Lost in the Middle"
  - Semantic constraint: not statically verifiable
  - Harness weakness: community-reported empirical observation (labeled)`,
		Args: cobra.ExactArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			runCheck(args[0], providerFlag, modelFlag, targets, all, verbose)
		},
	}

	checkCmd.Flags().StringVar(&providerFlag, "provider", "", "LLM provider for classification")
	checkCmd.Flags().StringVar(&modelFlag, "model", "claude-haiku-4-5-20251001", "LLM model name")
	checkCmd.Flags().BoolVar(&all, "all", false, "show all instructions including low-risk ones")
	checkCmd.Flags().BoolVar(&verbose, "verbose", false, "in directory mode, print full per-file detail (default: summary table)")
	checkCmd.Flags().StringSliceVar(&targets, "targets", checker.Harnesses, "harnesses to check against")

	rootCmd.AddCommand(checkCmd)
}

type checkFileResult struct {
	Path   string
	Format string
	Check  checker.CheckResult
	Err    error
	Empty  bool
}

func runCheck(path, providerFlag, modelFlag string, targets []string, all, verbose bool) {
	files, err := discovery.Resolve(path)
	if err != nil {
		fmt.Println(color.RedString("Error: %v", err))
		os.Exit(1)
	}
	if len(files) == 0 {
		fmt.Println(color.YellowString("No agent files found in %s.", path))
		return
	}

	provider, _, provErr := selectProvider(providerFlag, modelFlag, false)
	if provErr != nil {
		fmt.Println(color.RedString("Error: %v", provErr))
		fmt.Println(color.YellowString("check requires an LLM provider. Set ANTHROPIC_API_KEY, OPENROUTER_API_KEY, or start an Ollama instance."))
		os.Exit(1)
	}

	results := make([]checkFileResult, 0, len(files))
	for _, f := range files {
		results = append(results, checkOne(f, provider, targets))
	}

	if len(results) == 1 {
		r := results[0]
		if r.Err != nil {
			fmt.Println(color.RedString("Error: %v", r.Err))
			os.Exit(1)
		}
		if r.Empty {
			fmt.Println(color.YellowString("No instructions found."))
			return
		}
		printCheckResult(r.Path, r.Check, targets, all)
		return
	}

	if verbose {
		for i, r := range results {
			if i > 0 {
				fmt.Println()
			}
			if r.Err != nil {
				fmt.Println(color.RedString("Error reading %s: %v", r.Path, r.Err))
				continue
			}
			if r.Empty {
				fmt.Printf("%s: no instructions found\n", r.Path)
				continue
			}
			printCheckResult(r.Path, r.Check, targets, all)
		}
		return
	}
	printCheckSummary(results, targets)
}

func checkOne(filePath string, provider llm.Provider, targets []string) checkFileResult {
	content, err := os.ReadFile(filePath)
	if err != nil {
		return checkFileResult{Path: filePath, Err: err}
	}
	format := detectFormat(filePath, string(content))

	cl, classifyErr := classifier.ClassifyLLM(string(content), format, provider)
	if classifyErr != nil {
		return checkFileResult{Path: filePath, Format: format, Err: classifyErr}
	}

	if len(cl.Items) == 0 {
		return checkFileResult{Path: filePath, Format: format, Empty: true}
	}

	result := checker.Check(string(content), format, targets, cl)
	return checkFileResult{Path: filePath, Format: format, Check: result}
}

func printCheckSummary(results []checkFileResult, targets []string) {
	bold := color.New(color.Bold)
	bold.Printf("Checked %d files\n\n", len(results))

	header := fmt.Sprintf("  %-50s %-10s %s", "FILE", "FORMAT", strings.Join(targets, " "))
	fmt.Println(color.New(color.Faint).Sprint(header))

	for _, r := range results {
		path := trimPath(r.Path, 50)
		if r.Err != nil {
			fmt.Printf("  %-50s %-10s %s\n", path, "-", color.RedString("error"))
			continue
		}
		if r.Empty {
			fmt.Printf("  %-50s %-10s %s\n", path, r.Format, color.New(color.Faint).Sprint("(no instructions)"))
			continue
		}
		risks := make([]string, 0, len(targets))
		for _, h := range targets {
			risks = append(risks, fmt.Sprintf("%s %s", riskIcon(r.Check.Overall[h]), riskLabel(r.Check.Overall[h])))
		}
		fmt.Printf("  %-50s %-10s %s\n", path, r.Format, strings.Join(risks, "  "))
	}
	fmt.Println()
	fmt.Println(color.New(color.Faint).Sprint("Use --verbose for per-file detail."))
}

func printCheckResult(filePath string, result checker.CheckResult, targets []string, all bool) {
	bold := color.New(color.Bold)
	bold.Printf("Compliance risk: %s\n", filePath)
	fmt.Println()

	byFacet := groupByFacet(result.Instructions)
	facetLabels := map[classifier.Facet]string{
		classifier.FacetContext:       "Context",
		classifier.FacetStrategy:      "Strategy",
		classifier.FacetGuardrails:    "Guardrails",
		classifier.FacetObservability: "Observability",
		classifier.FacetSecurity:      "Security",
	}

	for _, facet := range classifier.AllFacets {
		items := byFacet[facet]
		if len(items) == 0 {
			continue
		}

		shown := 0
		var skipped int
		for _, ir := range items {
			if !all && maxRisk(ir.Risks, targets) == checker.RiskLow && len(ir.Suggestions) == 0 {
				skipped++
				continue
			}
			shown++
		}

		fmt.Printf("  %s (%d)\n", bold.Sprint(facetLabels[facet]), len(items))

		for _, ir := range items {
			if !all && maxRisk(ir.Risks, targets) == checker.RiskLow && len(ir.Suggestions) == 0 {
				continue
			}
			printInstruction(ir, targets)
		}

		_ = shown
		if skipped > 0 {
			fmt.Printf("    %s\n", color.New(color.Faint).Sprintf("… %d low-risk instructions hidden (--all to show)", skipped))
		}
		fmt.Println()
	}

	// Summary
	bold.Println("  Summary")
	for _, h := range targets {
		level := result.Overall[h]
		high := result.HighRiskCount[h]
		icon := riskIcon(level)
		label := riskLabel(level)
		if high > 0 {
			fmt.Printf("    %-12s %s %s  (%d high-risk instruction(s))\n", h, icon, label, high)
		} else {
			fmt.Printf("    %-12s %s %s\n", h, icon, label)
		}
	}
	fmt.Println()
	fmt.Println(color.New(color.Faint).Sprint("  Risk levels based on documented factors only. Run 'reify bench' to measure empirically."))
}

func printInstruction(ir checker.InstructionResult, targets []string) {
	text := ir.Text
	if len(text) > 72 {
		text = text[:69] + "..."
	}
	fmt.Printf("    · %s\n", text)

	// Show per-harness risk only when they differ, otherwise show once.
	allSame := true
	firstLevel := ir.Risks[targets[0]]
	for _, h := range targets[1:] {
		if ir.Risks[h] != firstLevel {
			allSame = false
			break
		}
	}

	if allSame {
		level := firstLevel
		factors := ir.Factors[targets[0]]
		fmt.Printf("        all harnesses  %s %s\n", riskIcon(level), riskLabel(level))
		if level > checker.RiskLow {
			fmt.Printf("        factors: %s\n", strings.Join(factors.ActiveFactors(), " · "))
		}
	} else {
		for _, h := range targets {
			level := ir.Risks[h]
			factors := ir.Factors[h]
			fmt.Printf("        %-12s %s %s", h, riskIcon(level), riskLabel(level))
			if level > checker.RiskLow {
				fmt.Printf("  [%s]", strings.Join(factors.ActiveFactors(), ", "))
			}
			fmt.Println()
		}
	}

	for _, s := range ir.Suggestions {
		fmt.Printf("        %s %s\n", color.YellowString("↑"), color.YellowString(s))
	}
}

func riskIcon(r checker.RiskLevel) string {
	switch r {
	case checker.RiskLow:
		return color.GreenString("✓")
	case checker.RiskMedium:
		return color.YellowString("⚠")
	default:
		return color.RedString("✗")
	}
}

func riskLabel(r checker.RiskLevel) string {
	switch r {
	case checker.RiskLow:
		return color.GreenString("low risk")
	case checker.RiskMedium:
		return color.YellowString("medium risk")
	default:
		return color.RedString("HIGH RISK")
	}
}

func maxRisk(risks map[string]checker.RiskLevel, targets []string) checker.RiskLevel {
	var max checker.RiskLevel
	for _, h := range targets {
		if r := risks[h]; r > max {
			max = r
		}
	}
	return max
}

func groupByFacet(items []checker.InstructionResult) map[classifier.Facet][]checker.InstructionResult {
	m := make(map[classifier.Facet][]checker.InstructionResult)
	for _, item := range items {
		m[item.Facet] = append(m[item.Facet], item)
	}
	return m
}
