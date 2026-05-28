// Command reify-calibrate is the calibration battery for the LLM facet
// classifier. It is a *separate* binary from `reify` because the calibration
// workflow (sample, judge, score) is internal tooling: it runs against
// existing classify.log artifacts, requires a stronger judge model than
// production traffic, and produces gold-label JSONL files that are not
// useful to general Reify consumers.
//
// Kept in the same module so it can reuse internal/llm, internal/classifier,
// etc. Not registered on the public `reify` rootCmd.
package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"math/rand"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"

	"github.com/fatih/color"
	"github.com/mirandaguillaume/reify/internal/classifier"
	"github.com/mirandaguillaume/reify/internal/llm"
	"github.com/spf13/cobra"
)

func main() {
	root := &cobra.Command{
		Use:   "reify-calibrate",
		Short: "Build and score a calibration battery for the LLM facet classifier",
		Long: `reify-calibrate provides tooling to measure how well the LLM classifier
agrees with human labels per facet.

Subcommands:
  sample   Stratified-sample N items per facet from existing classify.log files
  judge    Have a stronger LLM assign a second opinion (LLM-as-Judge)
  score    Compute precision/recall/F1 and a confusion matrix vs gold labels (TODO)

See docs/calibration/rubric.md for the canonical facet definitions.`,
	}
	root.AddCommand(sampleCmd())
	root.AddCommand(judgeCmd())
	if err := root.Execute(); err != nil {
		os.Exit(1)
	}
}

// calibrateItem is the JSONL row format used across sample/judge/score.
type calibrateItem struct {
	ID         string `json:"id"`
	Text       string `json:"text"`
	Section    string `json:"section,omitempty"`
	SourceFile string `json:"source_file"`
	SourceRepo string `json:"source_repo"`
	LLMLabel   string `json:"llm_label"`
	GoldLabel  string `json:"gold_label,omitempty"`
	JudgeLabel string `json:"judge_label,omitempty"`
	Notes      string `json:"notes,omitempty"`
}

// ---------- sample ----------

func sampleCmd() *cobra.Command {
	var sourceDir string
	var perFacet int
	var output string
	var seed int64

	cmd := &cobra.Command{
		Use:   "sample",
		Short: "Stratified-sample N items per facet from existing classify.log files",
		Long: `sample walks a directory tree looking for classify.log files (the JSON
array output of 'reify classify --json'), flattens every classified item
across the corpus, and writes a stratified sample of N items per facet to
a JSONL output file.

The resulting JSONL has empty gold_label fields ready for a human annotator
to fill in. The id field is stable across runs with the same seed so a
partially-labelled corpus can be re-sampled without losing existing labels
(merge by id).`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runSample(sourceDir, perFacet, output, seed)
		},
	}

	cmd.Flags().StringVar(&sourceDir, "source", "", "directory tree containing classify.log files (required)")
	cmd.Flags().IntVar(&perFacet, "n", 40, "items to sample per facet")
	cmd.Flags().StringVarP(&output, "output", "o", "calibration-corpus.jsonl", "output JSONL path")
	cmd.Flags().Int64Var(&seed, "seed", 42, "random seed for reproducible sampling")
	_ = cmd.MarkFlagRequired("source")
	return cmd
}

func runSample(sourceDir string, perFacet int, output string, seed int64) error {
	if perFacet <= 0 {
		return fmt.Errorf("--n must be > 0")
	}

	logs, err := findClassifyLogs(sourceDir)
	if err != nil {
		return err
	}
	if len(logs) == 0 {
		return fmt.Errorf("no classify.log files found under %s", sourceDir)
	}

	fmt.Printf("Scanning %d classify.log files...\n", len(logs))

	byFacet := make(map[classifier.Facet][]calibrateItem)
	for _, log := range logs {
		items, err := loadClassifyLog(log, sourceDir)
		if err != nil {
			fmt.Fprintf(os.Stderr, "  warn: skipping %s: %v\n", log, err)
			continue
		}
		for _, it := range items {
			f := classifier.Facet(it.LLMLabel)
			byFacet[f] = append(byFacet[f], it)
		}
	}

	total := 0
	for _, items := range byFacet {
		total += len(items)
	}
	fmt.Printf("Pool: %d items across %d facets\n", total, len(byFacet))

	rng := rand.New(rand.NewSource(seed))
	var sampled []calibrateItem
	for _, facet := range classifier.AllFacets {
		pool := byFacet[facet]
		take := perFacet
		if take > len(pool) {
			fmt.Println(color.YellowString("  warn: facet %s has only %d items, taking all", facet, len(pool)))
			take = len(pool)
		}
		idx := rng.Perm(len(pool))[:take]
		sort.Ints(idx)
		for _, i := range idx {
			sampled = append(sampled, pool[i])
		}
		fmt.Printf("  %-14s %4d sampled from %d available\n", facet, take, len(pool))
	}

	for i := range sampled {
		sampled[i].ID = fmt.Sprintf("%s-%04d", sampled[i].LLMLabel, i+1)
	}

	if err := writeJSONL(output, sampled); err != nil {
		return fmt.Errorf("write %s: %w", output, err)
	}
	fmt.Printf("\nWrote %d items to %s\n", len(sampled), output)
	fmt.Println(color.New(color.Faint).Sprint("Next: fill gold_label in each row, then `reify-calibrate judge` / `reify-calibrate score`."))
	return nil
}

func findClassifyLogs(root string) ([]string, error) {
	var out []string
	err := filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil
		}
		if !info.IsDir() && filepath.Base(path) == "classify.log" {
			out = append(out, path)
		}
		return nil
	})
	return out, err
}

func loadClassifyLog(path, sourceRoot string) ([]calibrateItem, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var entries []struct {
		File   string `json:"file"`
		Format string `json:"format"`
		Facets map[string][]struct {
			Text    string `json:"text"`
			Section string `json:"section"`
		} `json:"facets"`
	}
	if err := json.Unmarshal(raw, &entries); err != nil {
		return nil, fmt.Errorf("not a classify --json array: %w", err)
	}

	repo := inferRepo(path, sourceRoot)

	var items []calibrateItem
	for _, entry := range entries {
		for facet, list := range entry.Facets {
			for _, it := range list {
				items = append(items, calibrateItem{
					Text:       it.Text,
					Section:    it.Section,
					SourceFile: entry.File,
					SourceRepo: repo,
					LLMLabel:   facet,
				})
			}
		}
	}
	return items, nil
}

func inferRepo(logPath, sourceRoot string) string {
	rel, err := filepath.Rel(sourceRoot, logPath)
	if err != nil {
		return ""
	}
	parts := strings.Split(rel, string(filepath.Separator))
	for i, p := range parts {
		if p == "logs" && i+1 < len(parts) {
			return parts[i+1]
		}
	}
	if len(parts) > 0 {
		return parts[0]
	}
	return ""
}

// ---------- judge ----------

func judgeCmd() *cobra.Command {
	var input, output string
	var providerFlag, modelFlag string
	var concurrencyFlag int
	var force bool

	cmd := &cobra.Command{
		Use:   "judge",
		Short: "Have a stronger LLM assign a second-opinion facet label to each item",
		Long: `judge reads a calibration corpus (JSONL produced by 'reify-calibrate sample'),
sends each item to a strong LLM with the rubric inlined, and writes back the
corpus with a populated judge_label field.

The judge is a second annotator, not a referee. Its agreement with the human
gold labels is itself a measure (Cohen's kappa); high agreement on a sample
without gold labels does NOT prove either is correct — it just means both
share whatever bias is present.

To avoid in-family agreement bias, prefer a judge model from a different
family than the model(s) being evaluated. By default the strongest Anthropic
model is used; override with --judge-provider/--judge-model.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runJudge(input, output, providerFlag, modelFlag, concurrencyFlag, force)
		},
	}

	cmd.Flags().StringVarP(&input, "input", "i", "", "input JSONL corpus (required)")
	cmd.Flags().StringVarP(&output, "output", "o", "", "output JSONL path (default: overwrite input)")
	cmd.Flags().StringVar(&providerFlag, "judge-provider", "anthropic", "LLM provider for the judge (anthropic, openrouter, ollama)")
	cmd.Flags().StringVar(&modelFlag, "judge-model", "claude-opus-4-20250514", "judge model name (use the strongest from a different family than the evaluated model)")
	cmd.Flags().IntVar(&concurrencyFlag, "concurrency", 8, "parallel judge requests")
	cmd.Flags().BoolVar(&force, "force", false, "re-judge items that already have a judge_label")
	_ = cmd.MarkFlagRequired("input")
	return cmd
}

func runJudge(input, output, providerFlag, modelFlag string, concurrency int, force bool) error {
	if output == "" {
		output = input
	}
	if concurrency <= 0 {
		concurrency = 1
	}

	items, err := readJSONL(input)
	if err != nil {
		return fmt.Errorf("read %s: %w", input, err)
	}
	if len(items) == 0 {
		return fmt.Errorf("corpus is empty")
	}

	provider, err := selectJudgeProvider(providerFlag, modelFlag)
	if err != nil {
		return fmt.Errorf("judge provider: %w", err)
	}
	fmt.Printf("Judge: %s / %s\n", color.CyanString(providerFlag), modelFlag)

	header := judgePromptHeader()

	var pending []int
	for i, it := range items {
		if it.JudgeLabel != "" && !force {
			continue
		}
		pending = append(pending, i)
	}
	if len(pending) == 0 {
		fmt.Println(color.YellowString("All items already have judge_label. Use --force to re-judge."))
		return nil
	}

	fmt.Printf("Judging %d / %d items (concurrency=%d)\n\n", len(pending), len(items), concurrency)

	var (
		mu        sync.Mutex
		done      int
		errors    int
		agreeWith int
	)
	sem := make(chan struct{}, concurrency)
	var wg sync.WaitGroup

	for _, idx := range pending {
		wg.Add(1)
		sem <- struct{}{}
		go func(i int) {
			defer wg.Done()
			defer func() { <-sem }()

			label, jerr := judgeOne(provider, header, items[i])

			mu.Lock()
			defer mu.Unlock()
			done++
			if jerr != nil {
				errors++
				fmt.Fprintf(os.Stderr, "  [%d/%d] %s: error: %v\n", done, len(pending), items[i].ID, jerr)
				return
			}
			items[i].JudgeLabel = label
			if label == items[i].LLMLabel {
				agreeWith++
			}
			if done%20 == 0 || done == len(pending) {
				rate := float64(agreeWith) * 100 / float64(done-errors)
				fmt.Fprintf(os.Stderr, "  progress %d/%d  errors=%d  judge↔llm agree=%.1f%%\n",
					done, len(pending), errors, rate)
			}
		}(idx)
	}
	wg.Wait()

	if err := writeJSONL(output, items); err != nil {
		return fmt.Errorf("write %s: %w", output, err)
	}

	fmt.Println()
	if errors > 0 {
		fmt.Println(color.YellowString("Done with %d errors (left judge_label empty for those).", errors))
	} else {
		fmt.Println(color.GreenString("Done."))
	}
	if done > errors {
		rate := float64(agreeWith) * 100 / float64(done-errors)
		fmt.Printf("Judge ↔ existing llm_label agreement: %.1f%% (%d / %d)\n", rate, agreeWith, done-errors)
	}
	fmt.Printf("Wrote %s\n", output)
	return nil
}

// selectJudgeProvider is a minimal provider selector for the judge tool.
// Differs from the main reify selectProvider in that it doesn't auto-detect
// Ollama and doesn't honor REIFY_API_KEY catch-alls — the judge should be
// chosen explicitly to control the experiment.
func selectJudgeProvider(name, model string) (llm.Provider, error) {
	var apiKey string
	switch name {
	case "anthropic":
		apiKey = firstNonEmpty(os.Getenv("ANTHROPIC_REIFY_API_KEY"), os.Getenv("ANTHROPIC_API_KEY"))
		if apiKey == "" {
			return nil, fmt.Errorf("set ANTHROPIC_API_KEY (or ANTHROPIC_REIFY_API_KEY)")
		}
	case "openrouter":
		apiKey = firstNonEmpty(os.Getenv("OPENROUTER_REIFY_API_KEY"), os.Getenv("OPENROUTER_API_KEY"))
		if apiKey == "" {
			return nil, fmt.Errorf("set OPENROUTER_API_KEY (or OPENROUTER_REIFY_API_KEY)")
		}
	case "ollama":
		// no key
	default:
		return nil, fmt.Errorf("unknown provider %q (anthropic, openrouter, ollama)", name)
	}
	return llm.GetProviderWithModel(name, apiKey, model)
}

func firstNonEmpty(vals ...string) string {
	for _, v := range vals {
		if v != "" {
			return v
		}
	}
	return ""
}

// judgePromptHeader is the rubric-grounded preamble shared across calls.
// Per-item content is appended at call time. Kept compact so it fits in a
// single Anthropic prompt-cache breakpoint if caching is added later.
func judgePromptHeader() string {
	return `You are a strict annotator applying the Reify facet rubric (v1).

Five facets, exactly one per item:

- context: background knowledge the agent needs. Stateless facts about
  the project — tech stack, architecture, layout, conventions, the
  agent's own role. NOT how to do anything.
- strategy: how to approach a task. Imperatives, workflows, commands to
  run, decision procedures, style rules phrased as "do X" or "use Y".
- guardrails: things the agent must NOT do — negative-framed prohibitions
  whose breach is a code smell or rule violation but NOT a security event.
- observability: what to log, monitor, surface, report. Visibility of the
  agent's behaviour and the system's behaviour.
- security: permissions, secrets, access control, network/filesystem
  boundaries, data classification. A rule whose breach is a security
  event (leak, escalation, unauthorized action) belongs here even if
  phrased as a prohibition.

Tie-breakers, in order:
1. If breaking the rule causes a security event -> security.
2. Negative-framed and not a security event -> guardrails (not strategy).
3. "Log X" -> observability even if context-flavoured.
4. Telling the agent what to do beats telling it what is true -> strategy.

There is no "other" or "general". Force a choice.

Output: exactly one lowercase word — one of:
context, strategy, guardrails, observability, security.

No explanation, no punctuation, no quotes.

---

`
}

func judgeOne(provider llm.Provider, header string, it calibrateItem) (string, error) {
	var b strings.Builder
	b.WriteString(header)
	b.WriteString("Item to classify:\n\n")
	fmt.Fprintf(&b, "Text: %s\n", it.Text)
	if it.Section != "" {
		fmt.Fprintf(&b, "Section: %s\n", it.Section)
	}
	if it.SourceFile != "" {
		fmt.Fprintf(&b, "Source file: %s\n", it.SourceFile)
	}
	if it.SourceRepo != "" {
		fmt.Fprintf(&b, "Source repo: %s\n", it.SourceRepo)
	}
	b.WriteString("\nYour answer (one lowercase word):\n")

	resp, err := provider.Complete(b.String())
	if err != nil {
		return "", err
	}
	label := normalizeJudgeAnswer(resp)
	if !isValidFacet(label) {
		return "", fmt.Errorf("unrecognised judge label %q (raw: %q)", label, strings.TrimSpace(resp))
	}
	return label, nil
}

func normalizeJudgeAnswer(s string) string {
	s = strings.TrimSpace(s)
	for _, prefix := range []string{"facet:", "label:", "answer:"} {
		if strings.HasPrefix(strings.ToLower(s), prefix) {
			s = strings.TrimSpace(s[len(prefix):])
		}
	}
	s = strings.Trim(s, "`'\"")
	if i := strings.IndexAny(s, " \n\t."); i > 0 {
		s = s[:i]
	}
	return strings.ToLower(strings.TrimSpace(s))
}

func isValidFacet(s string) bool {
	for _, f := range classifier.AllFacets {
		if string(f) == s {
			return true
		}
	}
	return false
}

// ---------- shared I/O ----------

func writeJSONL(path string, items []calibrateItem) error {
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()
	enc := json.NewEncoder(f)
	for _, it := range items {
		if err := enc.Encode(it); err != nil {
			return err
		}
	}
	return nil
}

func readJSONL(path string) ([]calibrateItem, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	var items []calibrateItem
	scanner := bufio.NewScanner(f)
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		var it calibrateItem
		if err := json.Unmarshal([]byte(line), &it); err != nil {
			return nil, fmt.Errorf("bad JSONL line: %w", err)
		}
		items = append(items, it)
	}
	return items, scanner.Err()
}
