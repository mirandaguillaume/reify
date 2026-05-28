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
  score    Compute precision/recall/F1, a confusion matrix and Cohen's kappa

See docs/calibration/rubric.md for the canonical facet definitions.`,
	}
	root.AddCommand(sampleCmd())
	root.AddCommand(judgeCmd())
	root.AddCommand(scoreCmd())
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

// ---------- score ----------

func scoreCmd() *cobra.Command {
	var input string
	var format string

	cmd := &cobra.Command{
		Use:   "score",
		Short: "Compute precision/recall/F1, confusion matrix and Cohen's kappa vs gold labels",
		Long: `score reads a labelled calibration corpus (JSONL) and reports calibration
metrics for every available prediction column against gold_label.

Items without gold_label are skipped. Items with invalid (non-facet) labels
are flagged as errors. The metrics computed per (prediction, gold) pair:

  - precision, recall, F1 per facet
  - macro-F1 (unweighted mean across facets)
  - accuracy (overall agreement)
  - confusion matrix (rows = gold, columns = predicted)
  - Cohen's kappa with the standard Landis & Koch (1977) interpretation

When both llm_label and judge_label are present, also reports their pairwise
agreement (judge ↔ llm) which is informative even on items lacking gold.

Use --format json for a machine-readable dump suitable for a CI gate.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runScore(input, format)
		},
	}

	cmd.Flags().StringVarP(&input, "input", "i", "", "input JSONL corpus with gold_label filled (required)")
	cmd.Flags().StringVarP(&format, "format", "f", "text", "output format: text or json")
	_ = cmd.MarkFlagRequired("input")
	return cmd
}

func runScore(input, format string) error {
	items, err := readJSONL(input)
	if err != nil {
		return fmt.Errorf("read %s: %w", input, err)
	}

	var golded []calibrateItem
	var invalid []string
	for _, it := range items {
		if it.GoldLabel == "" {
			continue
		}
		if !isValidFacet(it.GoldLabel) {
			invalid = append(invalid, fmt.Sprintf("%s: gold_label=%q", it.ID, it.GoldLabel))
			continue
		}
		golded = append(golded, it)
	}

	report := &scoreReport{
		Input:         input,
		TotalItems:    len(items),
		ScoredItems:   len(golded),
		InvalidGolds:  invalid,
		LabelsPresent: map[string]int{},
	}
	for _, it := range items {
		if it.LLMLabel != "" {
			report.LabelsPresent["llm"]++
		}
		if it.JudgeLabel != "" {
			report.LabelsPresent["judge"]++
		}
	}

	// Per-comparison metrics: any label column that's populated and has a
	// gold counterpart on the same item.
	if any := anyLabel(golded, func(it calibrateItem) string { return it.LLMLabel }); any {
		report.LLMvsGold = computeComparison(golded, func(it calibrateItem) string { return it.LLMLabel })
	}
	if any := anyLabel(golded, func(it calibrateItem) string { return it.JudgeLabel }); any {
		report.JudgeVsGold = computeComparison(golded, func(it calibrateItem) string { return it.JudgeLabel })
	}

	// Judge vs LLM agreement does not require gold_label.
	if anyLabel(items, func(it calibrateItem) string { return it.LLMLabel }) &&
		anyLabel(items, func(it calibrateItem) string { return it.JudgeLabel }) {
		pairs := pairedItems(items, func(it calibrateItem) string { return it.LLMLabel },
			func(it calibrateItem) string { return it.JudgeLabel })
		report.JudgeVsLLM = &interAnnotator{
			N:          len(pairs),
			Agreement:  agreementRate(pairs),
			CohensKapp: cohenKappa(pairs),
		}
	}

	if format == "json" {
		out, err := json.MarshalIndent(report, "", "  ")
		if err != nil {
			return err
		}
		fmt.Println(string(out))
		return nil
	}
	renderScoreText(report)
	return nil
}

// scoreReport is the top-level result for --format json. Empty fields are
// omitted so a corpus without judge labels doesn't produce noisy nulls.
type scoreReport struct {
	Input         string         `json:"input"`
	TotalItems    int            `json:"total_items"`
	ScoredItems   int            `json:"scored_items"`
	LabelsPresent map[string]int `json:"labels_present"`
	InvalidGolds  []string       `json:"invalid_golds,omitempty"`

	LLMvsGold   *comparison     `json:"llm_vs_gold,omitempty"`
	JudgeVsGold *comparison     `json:"judge_vs_gold,omitempty"`
	JudgeVsLLM  *interAnnotator `json:"judge_vs_llm,omitempty"`
}

type comparison struct {
	N         int                                                 `json:"n"`
	PerFacet  map[classifier.Facet]facetMetrics                   `json:"per_facet"`
	MacroF1   float64                                             `json:"macro_f1"`
	Accuracy  float64                                             `json:"accuracy"`
	Confusion map[classifier.Facet]map[classifier.Facet]int       `json:"confusion"`
	Kappa     float64                                             `json:"cohens_kappa"`
}

type facetMetrics struct {
	Precision float64 `json:"precision"`
	Recall    float64 `json:"recall"`
	F1        float64 `json:"f1"`
	Support   int     `json:"support"`
	TP, FP, FN int    `json:"-"`
}

type interAnnotator struct {
	N          int     `json:"n"`
	Agreement  float64 `json:"agreement"`
	CohensKapp float64 `json:"cohens_kappa"`
}

// pair is a single (predicted, gold) tuple used during metric calculation.
type pair struct{ a, b string }

func anyLabel(items []calibrateItem, get func(calibrateItem) string) bool {
	for _, it := range items {
		if get(it) != "" {
			return true
		}
	}
	return false
}

// pairedItems collects (A, B) tuples where both extractors return a non-empty,
// valid facet for the item. Items missing either label are dropped so the
// resulting metrics speak only to overlap.
func pairedItems(items []calibrateItem, a, b func(calibrateItem) string) []pair {
	var out []pair
	for _, it := range items {
		va, vb := a(it), b(it)
		if va == "" || vb == "" {
			continue
		}
		if !isValidFacet(va) || !isValidFacet(vb) {
			continue
		}
		out = append(out, pair{va, vb})
	}
	return out
}

func computeComparison(golded []calibrateItem, predict func(calibrateItem) string) *comparison {
	pairs := pairedItems(golded, predict, func(it calibrateItem) string { return it.GoldLabel })
	if len(pairs) == 0 {
		return nil
	}

	confusion := make(map[classifier.Facet]map[classifier.Facet]int)
	for _, f := range classifier.AllFacets {
		confusion[f] = make(map[classifier.Facet]int)
		for _, g := range classifier.AllFacets {
			confusion[f][g] = 0
		}
	}

	// pair.a = predicted, pair.b = gold.
	correct := 0
	for _, p := range pairs {
		pred := classifier.Facet(p.a)
		gold := classifier.Facet(p.b)
		confusion[gold][pred]++
		if pred == gold {
			correct++
		}
	}

	perFacet := make(map[classifier.Facet]facetMetrics)
	macroSum := 0.0
	macroCount := 0
	for _, f := range classifier.AllFacets {
		tp := confusion[f][f]
		fn := 0
		for _, g := range classifier.AllFacets {
			if g != f {
				fn += confusion[f][g]
			}
		}
		fp := 0
		for _, g := range classifier.AllFacets {
			if g != f {
				fp += confusion[g][f]
			}
		}
		support := tp + fn

		var precision, recall, f1 float64
		if tp+fp > 0 {
			precision = float64(tp) / float64(tp+fp)
		}
		if tp+fn > 0 {
			recall = float64(tp) / float64(tp+fn)
		}
		if precision+recall > 0 {
			f1 = 2 * precision * recall / (precision + recall)
		}
		perFacet[f] = facetMetrics{
			Precision: precision, Recall: recall, F1: f1, Support: support,
			TP: tp, FP: fp, FN: fn,
		}
		if support > 0 {
			macroSum += f1
			macroCount++
		}
	}

	var macroF1 float64
	if macroCount > 0 {
		macroF1 = macroSum / float64(macroCount)
	}

	return &comparison{
		N:         len(pairs),
		PerFacet:  perFacet,
		MacroF1:   macroF1,
		Accuracy:  float64(correct) / float64(len(pairs)),
		Confusion: confusion,
		Kappa:     cohenKappa(pairs),
	}
}

// cohenKappa returns Cohen's kappa for two annotators on the same N items.
// Treats the labels in pair.a as annotator A and pair.b as annotator B.
// Returns 0 when expected agreement equals observed agreement (κ is
// undefined for p_e=1 which we approximate by saturating to 1.0).
func cohenKappa(pairs []pair) float64 {
	n := len(pairs)
	if n == 0 {
		return 0
	}
	agree := 0
	marginA := map[string]float64{}
	marginB := map[string]float64{}
	for _, p := range pairs {
		if p.a == p.b {
			agree++
		}
		marginA[p.a]++
		marginB[p.b]++
	}
	po := float64(agree) / float64(n)
	pe := 0.0
	N := float64(n)
	for k, v := range marginA {
		pe += (v / N) * (marginB[k] / N)
	}
	if pe >= 1.0 {
		return 1.0
	}
	return (po - pe) / (1.0 - pe)
}

func agreementRate(pairs []pair) float64 {
	if len(pairs) == 0 {
		return 0
	}
	agree := 0
	for _, p := range pairs {
		if p.a == p.b {
			agree++
		}
	}
	return float64(agree) / float64(len(pairs))
}

// kappaInterpretation maps a kappa value to the Landis & Koch (1977) label
// commonly used in inter-annotator agreement literature.
func kappaInterpretation(k float64) string {
	switch {
	case k < 0:
		return "poor (worse than chance)"
	case k < 0.2:
		return "slight"
	case k < 0.4:
		return "fair"
	case k < 0.6:
		return "moderate"
	case k < 0.8:
		return "substantial"
	default:
		return "almost perfect"
	}
}

func renderScoreText(r *scoreReport) {
	bold := color.New(color.Bold).SprintFunc()
	faint := color.New(color.Faint).SprintFunc()

	fmt.Printf("%s %s\n", bold("Corpus:"), r.Input)
	fmt.Printf("  total items     : %d\n", r.TotalItems)
	fmt.Printf("  with gold_label : %d\n", r.ScoredItems)
	if c, ok := r.LabelsPresent["llm"]; ok {
		fmt.Printf("  with llm_label  : %d\n", c)
	}
	if c, ok := r.LabelsPresent["judge"]; ok {
		fmt.Printf("  with judge_label: %d\n", c)
	}
	if len(r.InvalidGolds) > 0 {
		fmt.Println(color.YellowString("  invalid gold labels:"))
		for _, s := range r.InvalidGolds {
			fmt.Printf("    %s\n", s)
		}
	}

	if r.LLMvsGold != nil {
		fmt.Println()
		fmt.Println(bold("=== llm_label vs gold ==="))
		renderComparison(r.LLMvsGold)
	}
	if r.JudgeVsGold != nil {
		fmt.Println()
		fmt.Println(bold("=== judge_label vs gold ==="))
		renderComparison(r.JudgeVsGold)
	}
	if r.JudgeVsLLM != nil {
		fmt.Println()
		fmt.Println(bold("=== judge_label vs llm_label (no gold required) ==="))
		fmt.Printf("  N            : %d\n", r.JudgeVsLLM.N)
		fmt.Printf("  agreement    : %.3f\n", r.JudgeVsLLM.Agreement)
		fmt.Printf("  Cohen's κ    : %.3f %s\n",
			r.JudgeVsLLM.CohensKapp,
			faint("("+kappaInterpretation(r.JudgeVsLLM.CohensKapp)+")"))
	}
}

func renderComparison(c *comparison) {
	bold := color.New(color.Bold).SprintFunc()
	faint := color.New(color.Faint).SprintFunc()

	fmt.Printf("  N: %d   accuracy: %.3f   macro-F1: %.3f   κ: %.3f %s\n\n",
		c.N, c.Accuracy, c.MacroF1, c.Kappa,
		faint("("+kappaInterpretation(c.Kappa)+")"))

	fmt.Println(bold("  Per-facet metrics:"))
	fmt.Printf("    %-14s %9s %7s %7s %8s\n", "facet", "precision", "recall", "F1", "support")
	for _, f := range classifier.AllFacets {
		m := c.PerFacet[f]
		fmt.Printf("    %-14s %9.3f %7.3f %7.3f %8d\n",
			string(f), m.Precision, m.Recall, m.F1, m.Support)
	}

	fmt.Println()
	fmt.Println(bold("  Confusion matrix (rows = gold, columns = predicted):"))
	fmt.Printf("    %-14s", "")
	for _, f := range classifier.AllFacets {
		fmt.Printf(" %6s", abbreviateFacet(f))
	}
	fmt.Println()
	for _, gold := range classifier.AllFacets {
		fmt.Printf("    %-14s", string(gold))
		for _, pred := range classifier.AllFacets {
			n := c.Confusion[gold][pred]
			if n == 0 {
				fmt.Printf(" %6s", faint("."))
				continue
			}
			if gold == pred {
				fmt.Printf(" %6s", color.GreenString("%d", n))
			} else {
				fmt.Printf(" %6d", n)
			}
		}
		fmt.Println()
	}
}

// abbreviateFacet shortens facet names to keep the confusion matrix grid
// readable on an 80-column terminal.
func abbreviateFacet(f classifier.Facet) string {
	switch f {
	case classifier.FacetContext:
		return "ctx"
	case classifier.FacetStrategy:
		return "str"
	case classifier.FacetGuardrails:
		return "grd"
	case classifier.FacetObservability:
		return "obs"
	case classifier.FacetSecurity:
		return "sec"
	}
	return string(f)
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
