package cmd

import (
	"fmt"
	"os"
	"strings"

	"github.com/fatih/color"
	"github.com/mirandaguillaume/reify/internal/checker"
	"github.com/mirandaguillaume/reify/internal/classifier"
	"github.com/spf13/cobra"
)

func init() {
	var providerFlag, modelFlag string
	var quick, all bool
	var targets []string

	checkCmd := &cobra.Command{
		Use:   "check <file>",
		Short: "Assess instruction following compliance risk per harness",
		Long: `Analyzes each instruction in an agent file and flags compliance risks
per AI coding harness (Claude Code, Copilot, Cursor).

Risk levels are derived from documented factors — no invented percentages:
  - Negative framing: IFEval benchmark (Zhou et al. 2023)
  - Middle position:  Liu et al. 2023 "Lost in the Middle"
  - Semantic constraint: not statically verifiable
  - Harness weakness: community-reported empirical observation (labeled)

Use --bench to validate empirically.`,
		Args: cobra.ExactArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			runCheck(args[0], providerFlag, modelFlag, targets, quick, all)
		},
	}

	checkCmd.Flags().StringVar(&providerFlag, "provider", "", "LLM provider for classification")
	checkCmd.Flags().StringVar(&modelFlag, "model", "claude-haiku-4-5-20251001", "LLM model name")
	checkCmd.Flags().BoolVar(&quick, "quick", false, "static classification only — no LLM")
	checkCmd.Flags().BoolVar(&all, "all", false, "show all instructions including low-risk ones")
	checkCmd.Flags().StringSliceVar(&targets, "targets", checker.Harnesses, "harnesses to check against")

	rootCmd.AddCommand(checkCmd)
}

func runCheck(filePath, providerFlag, modelFlag string, targets []string, quick, all bool) {
	content, err := os.ReadFile(filePath)
	if err != nil {
		fmt.Println(color.RedString("Error: %v", err))
		os.Exit(1)
	}

	format := detectFormat(filePath, string(content))

	var cl classifier.Result
	if quick {
		cl = classifier.Classify(string(content), format)
	} else {
		provider, _, provErr := selectProvider(providerFlag, modelFlag, false)
		if provErr != nil {
			fmt.Println(color.YellowString("No LLM provider — falling back to static classification"))
			cl = classifier.Classify(string(content), format)
		} else {
			cl, _ = classifier.ClassifyLLM(string(content), format, provider)
		}
	}

	if len(cl.Items) == 0 {
		fmt.Println(color.YellowString("No instructions found."))
		return
	}

	result := checker.Check(string(content), format, targets, cl)
	printCheckResult(filePath, result, targets, all)
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
