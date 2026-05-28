package cmd

import (
	"fmt"
	"os"
	"strings"

	"github.com/fatih/color"
	"github.com/mirandaguillaume/reify/internal/classifier"
	"github.com/mirandaguillaume/reify/internal/doctor/parser"
	"github.com/spf13/cobra"
)

func init() {
	var jsonOut, quick bool
	var providerFlag string
	modelFlag := "claude-haiku-4-5-20251001" // smallest Anthropic model — sufficient for label classification

	classifyCmd := &cobra.Command{
		Use:   "classify <file>",
		Short: "Classify instructions in an agent file by Reify facet",
		Long: `Classify reads any agent definition file (CLAUDE.md, copilot-instructions.md,
Reify YAML, etc.) and maps each instruction to one of the five Reify facets:
context, strategy, guardrails, observability, security.

Uses an LLM by default for semantic accuracy. Use --quick for instant static analysis.
Useful before 'reify import' to understand what's present and what's missing.`,
		Args: cobra.ExactArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			runClassify(args[0], providerFlag, modelFlag, jsonOut, quick)
		},
	}

	classifyCmd.Flags().BoolVar(&jsonOut, "json", false, "output as JSON")
	classifyCmd.Flags().BoolVar(&quick, "quick", false, "static analysis only — no LLM, instant results")
	classifyCmd.Flags().StringVar(&providerFlag, "provider", "", "LLM provider (ollama, openrouter, anthropic)")
	classifyCmd.Flags().StringVar(&modelFlag, "model", modelFlag, "LLM model name (default: claude-haiku-4-5)")

	rootCmd.AddCommand(classifyCmd)
}

func runClassify(filePath, providerFlag, modelFlag string, jsonOut, quick bool) {
	content, err := os.ReadFile(filePath)
	if err != nil {
		fmt.Println(color.RedString("Error: %v", err))
		os.Exit(1)
	}

	format := detectFormat(filePath, string(content))

	var result classifier.Result

	if quick {
		result = classifier.Classify(string(content), format)
	} else {
		provider, providerName, err := selectProvider(providerFlag, modelFlag, false)
		if err != nil {
			fmt.Println(color.RedString("Error: %v", err))
			fmt.Println(color.YellowString("Tip: use --quick for static analysis without an LLM."))
			os.Exit(1)
		}
		if !jsonOut {
			fmt.Printf("Using provider: %s\n\n", color.CyanString(providerName))
		}
		result, err = classifier.ClassifyLLM(string(content), format, provider)
		if err != nil {
			fmt.Println(color.RedString("Error: %v", err))
			os.Exit(1)
		}
	}

	if jsonOut {
		printClassifyJSON(result)
		return
	}

	printClassifyTable(filePath, result)
}

// detectFormat uses the doctor parser registry to identify the agent format.
func detectFormat(filePath, content string) string {
	p, err := parser.DetectFormat(filePath, []byte(content))
	if err != nil {
		return "unknown"
	}
	return p.Format()
}

func printClassifyTable(filePath string, result classifier.Result) {
	total := len(result.Items)

	bold := color.New(color.Bold)
	bold.Printf("Classifying %s", filePath)
	if result.Format != "" && result.Format != "unknown" {
		fmt.Printf(" (%s format)", result.Format)
	}
	fmt.Println()
	fmt.Println()

	if total == 0 {
		fmt.Println(color.YellowString("No instructions found. The file may be empty or use an unsupported format."))
		return
	}

	byFacet := result.ByFacet()

	facetLabels := map[classifier.Facet]string{
		classifier.FacetContext:       "Context",
		classifier.FacetStrategy:      "Strategy",
		classifier.FacetGuardrails:    "Guardrails",
		classifier.FacetObservability: "Observability",
		classifier.FacetSecurity:      "Security",
	}
	facetColors := map[classifier.Facet]func(...interface{}) string{
		classifier.FacetContext:       color.New(color.FgCyan).SprintFunc(),
		classifier.FacetStrategy:      color.New(color.FgBlue).SprintFunc(),
		classifier.FacetGuardrails:    color.New(color.FgYellow).SprintFunc(),
		classifier.FacetObservability: color.New(color.FgMagenta).SprintFunc(),
		classifier.FacetSecurity:      color.New(color.FgRed).SprintFunc(),
	}

	for _, facet := range classifier.AllFacets {
		items := byFacet[facet]
		label := facetLabels[facet]
		colorFn := facetColors[facet]
		count := len(items)

		if count == 0 {
			fmt.Printf("  %s %s\n",
				color.New(color.FgRed).Sprint("✗"),
				color.New(color.Faint).Sprintf("%s (0 — missing)", label),
			)
			continue
		}

		pct := count * 100 / total
		bar := strings.Repeat("█", pct/5)
		fmt.Printf("  %s  %s %s\n",
			colorFn(fmt.Sprintf("%-14s", label)),
			colorFn(bar),
			color.New(color.Faint).Sprintf("%d%%  (%d)", pct, count),
		)
		for _, item := range items {
			text := item.Text
			if len(text) > 80 {
				text = text[:77] + "..."
			}
			fmt.Printf("    %s %s\n", color.New(color.Faint).Sprint("·"), text)
		}
		fmt.Println()
	}

	// Summary line.
	missing := missingFacets(byFacet)
	if len(missing) > 0 {
		labels := make([]string, len(missing))
		for i, f := range missing {
			labels[i] = facetLabels[f]
		}
		fmt.Printf("%s Missing facets: %s\n",
			color.YellowString("⚠"),
			color.YellowString(strings.Join(labels, ", ")),
		)
		fmt.Println("  Run 'reify doctor' for recommendations, or 'reify import' to convert.")
	} else {
		fmt.Printf("%s All 5 facets covered — ready for 'reify import'\n", color.GreenString("✓"))
	}
}

func missingFacets(byFacet map[classifier.Facet][]classifier.Item) []classifier.Facet {
	var missing []classifier.Facet
	for _, f := range classifier.AllFacets {
		if len(byFacet[f]) == 0 {
			missing = append(missing, f)
		}
	}
	return missing
}

func printClassifyJSON(result classifier.Result) {
	byFacet := result.ByFacet()
	fmt.Println("{")
	fmt.Printf("  \"format\": %q,\n", result.Format)
	fmt.Println("  \"facets\": {")
	for i, facet := range classifier.AllFacets {
		items := byFacet[facet]
		comma := ","
		if i == len(classifier.AllFacets)-1 {
			comma = ""
		}
		fmt.Printf("    %q: [\n", facet)
		for j, item := range items {
			itemComma := ","
			if j == len(items)-1 {
				itemComma = ""
			}
			fmt.Printf("      {\"text\": %q, \"section\": %q}%s\n", item.Text, item.Section, itemComma)
		}
		fmt.Printf("    ]%s\n", comma)
	}
	fmt.Println("  }")
	fmt.Println("}")
}
