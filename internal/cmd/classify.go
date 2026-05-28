package cmd

import (
	"fmt"
	"os"
	"strings"

	"github.com/fatih/color"
	"github.com/mirandaguillaume/reify/internal/classifier"
	"github.com/mirandaguillaume/reify/internal/discovery"
	"github.com/mirandaguillaume/reify/internal/doctor/parser"
	"github.com/mirandaguillaume/reify/internal/llm"
	"github.com/spf13/cobra"
)

func init() {
	var jsonOut, verbose bool
	var providerFlag string
	modelFlag := "claude-haiku-4-5-20251001" // smallest Anthropic model — sufficient for label classification

	classifyCmd := &cobra.Command{
		Use:   "classify <file|directory>",
		Short: "Classify instructions in an agent file by Reify facet",
		Long: `Classify reads any agent definition file (CLAUDE.md, copilot-instructions.md,
Reify YAML, etc.) and maps each instruction to one of the five Reify facets:
context, strategy, guardrails, observability, security.

Accepts a single file, an agent directory (.claude/, .github/agents/), or a
repo root — all recognized agent files are classified.

An LLM is required (Anthropic, OpenRouter, or Ollama) — keyword-based static
classification was removed because subjective prose cannot be classified
reliably from surface lexical patterns.`,
		Args: cobra.ExactArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			runClassify(args[0], providerFlag, modelFlag, jsonOut, verbose)
		},
	}

	classifyCmd.Flags().BoolVar(&jsonOut, "json", false, "output as JSON (always per-file array)")
	classifyCmd.Flags().BoolVar(&verbose, "verbose", false, "in directory mode, print full per-file detail (default: summary table)")
	classifyCmd.Flags().StringVar(&providerFlag, "provider", "", "LLM provider (ollama, openrouter, anthropic)")
	classifyCmd.Flags().StringVar(&modelFlag, "model", modelFlag, "LLM model name (default: claude-haiku-4-5)")

	rootCmd.AddCommand(classifyCmd)
}

// classifyFileResult bundles the inputs and outputs for one file.
type classifyFileResult struct {
	Path   string
	Format string
	Result classifier.Result
	Err    error
}

func runClassify(path, providerFlag, modelFlag string, jsonOut, verbose bool) {
	files, err := discovery.Resolve(path)
	if err != nil {
		fmt.Println(color.RedString("Error: %v", err))
		os.Exit(1)
	}
	if len(files) == 0 {
		if jsonOut {
			fmt.Println("[]")
			return
		}
		fmt.Println(color.YellowString("No agent files found in %s.", path))
		return
	}

	provider, providerName, perr := selectProvider(providerFlag, modelFlag, false)
	if perr != nil {
		fmt.Println(color.RedString("Error: %v", perr))
		fmt.Println(color.YellowString("classify requires an LLM provider. Set ANTHROPIC_API_KEY, OPENROUTER_API_KEY, or start an Ollama instance."))
		os.Exit(1)
	}
	if !jsonOut {
		fmt.Printf("Using provider: %s\n\n", color.CyanString(providerName))
	}

	results := make([]classifyFileResult, 0, len(files))
	for _, f := range files {
		results = append(results, classifyOne(f, provider))
	}

	if jsonOut {
		printClassifyJSONMulti(results)
		return
	}

	if len(results) == 1 {
		r := results[0]
		if r.Err != nil {
			fmt.Println(color.RedString("Error reading %s: %v", r.Path, r.Err))
			os.Exit(1)
		}
		printClassifyTable(r.Path, r.Result)
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
			printClassifyTable(r.Path, r.Result)
		}
		return
	}
	printClassifySummary(results)
}

// classifyOne reads, detects, and classifies one file. Errors are captured on
// the result rather than aborting — callers iterating over many files want to
// continue past a single bad input.
func classifyOne(filePath string, provider llm.Provider) classifyFileResult {
	content, err := os.ReadFile(filePath)
	if err != nil {
		return classifyFileResult{Path: filePath, Err: err}
	}
	format := detectFormat(filePath, string(content))

	result, classifyErr := classifier.ClassifyLLM(string(content), format, provider)
	if classifyErr != nil {
		return classifyFileResult{Path: filePath, Format: format, Err: classifyErr}
	}
	return classifyFileResult{Path: filePath, Format: format, Result: result}
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

// printClassifyJSONMulti emits a JSON array of one entry per resolved file.
// Single-file invocations produce an array of length one.
func printClassifyJSONMulti(results []classifyFileResult) {
	fmt.Println("[")
	for i, r := range results {
		printClassifyJSONEntry(r, "  ")
		if i < len(results)-1 {
			fmt.Println("  ,")
		}
	}
	fmt.Println("]")
}

func printClassifyJSONEntry(r classifyFileResult, indent string) {
	fmt.Printf("%s{\n", indent)
	fmt.Printf("%s  \"file\": %q,\n", indent, r.Path)
	fmt.Printf("%s  \"format\": %q,\n", indent, r.Format)
	if r.Err != nil {
		fmt.Printf("%s  \"error\": %q\n", indent, r.Err.Error())
		fmt.Printf("%s}\n", indent)
		return
	}
	byFacet := r.Result.ByFacet()
	fmt.Printf("%s  \"facets\": {\n", indent)
	for i, facet := range classifier.AllFacets {
		items := byFacet[facet]
		comma := ","
		if i == len(classifier.AllFacets)-1 {
			comma = ""
		}
		fmt.Printf("%s    %q: [\n", indent, facet)
		for j, item := range items {
			itemComma := ","
			if j == len(items)-1 {
				itemComma = ""
			}
			fmt.Printf("%s      {\"text\": %q, \"section\": %q}%s\n", indent, item.Text, item.Section, itemComma)
		}
		fmt.Printf("%s    ]%s\n", indent, comma)
	}
	fmt.Printf("%s  }\n", indent)
	fmt.Printf("%s}\n", indent)
}

// printClassifySummary prints a compact per-file table when the input is a
// directory holding multiple agent files. The verbose flag switches the
// caller to per-file detail instead.
func printClassifySummary(results []classifyFileResult) {
	bold := color.New(color.Bold)
	bold.Printf("Classified %d files\n\n", len(results))

	header := fmt.Sprintf("  %-50s %-10s %s", "FILE", "FORMAT", "FACETS (ctx/str/grd/obs/sec)")
	fmt.Println(color.New(color.Faint).Sprint(header))

	for _, r := range results {
		if r.Err != nil {
			fmt.Printf("  %-50s %-10s %s\n", trimPath(r.Path, 50), "-", color.RedString("error: %v", r.Err))
			continue
		}
		byFacet := r.Result.ByFacet()
		counts := fmt.Sprintf("%d/%d/%d/%d/%d",
			len(byFacet[classifier.FacetContext]),
			len(byFacet[classifier.FacetStrategy]),
			len(byFacet[classifier.FacetGuardrails]),
			len(byFacet[classifier.FacetObservability]),
			len(byFacet[classifier.FacetSecurity]),
		)
		fmt.Printf("  %-50s %-10s %s\n", trimPath(r.Path, 50), r.Format, counts)
	}
	fmt.Println()
	fmt.Println(color.New(color.Faint).Sprint("Use --verbose for per-file detail."))
}

// trimPath shortens a path for display, keeping the rightmost segment intact.
func trimPath(p string, max int) string {
	if len(p) <= max {
		return p
	}
	return "..." + p[len(p)-max+3:]
}
