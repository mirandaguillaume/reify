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
	"math"
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
// Multi-label per rubric v1.1: every label field is a set of facets, not
// a single facet. UnmarshalJSON accepts the v1 single-string form for
// backward compatibility and promotes it to a singleton list.
type calibrateItem struct {
	ID          string   `json:"id"`
	Text        string   `json:"text"`
	Section     string   `json:"section,omitempty"`
	SourceFile  string   `json:"source_file"`
	SourceRepo  string   `json:"source_repo"`
	LLMLabels   []string `json:"llm_labels,omitempty"`
	GoldLabels  []string `json:"gold_labels,omitempty"`
	JudgeLabels []string `json:"judge_labels,omitempty"`
	Notes       string   `json:"notes,omitempty"`
}

// UnmarshalJSON accepts both v1 (singular string fields) and v1.1
// (plural list fields) for label columns. Items written before the
// multi-label refactor stay readable; new writes always use the
// plural form.
func (it *calibrateItem) UnmarshalJSON(data []byte) error {
	type raw struct {
		ID          string   `json:"id"`
		Text        string   `json:"text"`
		Section     string   `json:"section,omitempty"`
		SourceFile  string   `json:"source_file"`
		SourceRepo  string   `json:"source_repo"`
		LLMLabel    string   `json:"llm_label,omitempty"`
		LLMLabels   []string `json:"llm_labels,omitempty"`
		GoldLabel   string   `json:"gold_label,omitempty"`
		GoldLabels  []string `json:"gold_labels,omitempty"`
		JudgeLabel  string   `json:"judge_label,omitempty"`
		JudgeLabels []string `json:"judge_labels,omitempty"`
		Notes       string   `json:"notes,omitempty"`
	}
	var r raw
	if err := json.Unmarshal(data, &r); err != nil {
		return err
	}
	it.ID = r.ID
	it.Text = r.Text
	it.Section = r.Section
	it.SourceFile = r.SourceFile
	it.SourceRepo = r.SourceRepo
	it.Notes = r.Notes
	it.LLMLabels = chooseLabels(r.LLMLabels, r.LLMLabel)
	it.GoldLabels = chooseLabels(r.GoldLabels, r.GoldLabel)
	it.JudgeLabels = chooseLabels(r.JudgeLabels, r.JudgeLabel)
	return nil
}

// chooseLabels prefers a populated plural field; falls back to promoting
// the v1 singular field to a singleton list when the plural is empty.
func chooseLabels(plural []string, singular string) []string {
	if len(plural) > 0 {
		return plural
	}
	if singular != "" {
		return []string{singular}
	}
	return nil
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
			// classify.log items are single-label by construction (Reify
			// classify emits each item under a single facet); promote into
			// our multi-label representation.
			if len(it.LLMLabels) == 0 {
				continue
			}
			f := classifier.Facet(it.LLMLabels[0])
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
		first := "x"
		if len(sampled[i].LLMLabels) > 0 {
			first = sampled[i].LLMLabels[0]
		}
		sampled[i].ID = fmt.Sprintf("%s-%04d", first, i+1)
	}

	if err := writeJSONL(output, sampled); err != nil {
		return fmt.Errorf("write %s: %w", output, err)
	}
	fmt.Printf("\nWrote %d items to %s\n", len(sampled), output)
	fmt.Println(color.New(color.Faint).Sprint("Next: fill gold_labels (JSON array) in each row, then `reify-calibrate judge` / `reify-calibrate score`."))
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
					LLMLabels:  []string{facet},
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
		if len(it.JudgeLabels) > 0 && !force {
			continue
		}
		pending = append(pending, i)
	}
	if len(pending) == 0 {
		fmt.Println(color.YellowString("All items already have judge_labels. Use --force to re-judge."))
		return nil
	}

	fmt.Printf("Judging %d / %d items (concurrency=%d)\n\n", len(pending), len(items), concurrency)

	var (
		mu             sync.Mutex
		done           int
		errors         int
		jaccardSum     float64
		jaccardCounted int
	)
	sem := make(chan struct{}, concurrency)
	var wg sync.WaitGroup

	for _, idx := range pending {
		wg.Add(1)
		sem <- struct{}{}
		go func(i int) {
			defer wg.Done()
			defer func() { <-sem }()

			labels, jerr := judgeOne(provider, header, items[i])

			mu.Lock()
			defer mu.Unlock()
			done++
			if jerr != nil {
				errors++
				fmt.Fprintf(os.Stderr, "  [%d/%d] %s: error: %v\n", done, len(pending), items[i].ID, jerr)
				return
			}
			items[i].JudgeLabels = labels
			if len(items[i].LLMLabels) > 0 {
				jaccardSum += jaccard(labels, items[i].LLMLabels)
				jaccardCounted++
			}
			if done%20 == 0 || done == len(pending) {
				rate := jaccardSum * 100 / float64(jaccardCounted)
				fmt.Fprintf(os.Stderr, "  progress %d/%d  errors=%d  mean Jaccard(judge,llm)=%.1f%%\n",
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
		fmt.Println(color.YellowString("Done with %d errors (left judge_labels empty for those).", errors))
	} else {
		fmt.Println(color.GreenString("Done."))
	}
	if jaccardCounted > 0 {
		rate := jaccardSum * 100 / float64(jaccardCounted)
		fmt.Printf("Mean Jaccard(judge_labels, llm_labels): %.1f%% over %d items with both populated\n",
			rate, jaccardCounted)
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
//
// v1.1 (multi-label): the judge is asked to emit every facet that
// applies, space-separated. Rubric v1's tie-breakers were removed because
// they force the judge to drop a true facet when two apply.
func judgePromptHeader() string {
	return `You are a strict annotator applying the Reify facet rubric (v1.1).

Five facets — an item may belong to ONE OR MORE:

- context: background knowledge the agent needs. Stateless facts about
  the project — tech stack, architecture, layout, conventions, the
  agent's own role. NOT how to do anything.
- strategy: how to approach a task. Imperatives, workflows, commands to
  run, decision procedures, style rules phrased as "do X" or "use Y".
- guardrails: things the agent must NOT do — negative-framed prohibitions.
- observability: what to log, monitor, surface, report. Visibility of the
  agent's behaviour and the system's behaviour.
- security: permissions, secrets, access control, network/filesystem
  boundaries, data classification. A rule whose breach is a security
  event (leak, escalation, unauthorized action) belongs here.

Multi-label guidance:

- "Never commit .env files" -> guardrails AND security
- "Log every API call without including PII" -> observability AND security
- "Use bcrypt for password hashing" -> context AND security (often;
  context alone if the line is purely descriptive)

Pick every facet that genuinely applies. Do NOT pick more than necessary
(no facet appears reflexively because of related keywords).

There is no "other" or "general". An item that fits none of the five is
not a valid item — answer with the single most defensible facet rather
than emitting nothing.

Output: facet names that apply, lowercase, space-separated, no
punctuation, no quotes, no explanation.

Examples of valid outputs:
  guardrails
  guardrails security
  observability
  strategy security
  context

---

`
}

func judgeOne(provider llm.Provider, header string, it calibrateItem) ([]string, error) {
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
	b.WriteString("\nYour answer (space-separated facet names):\n")

	resp, err := provider.Complete(b.String())
	if err != nil {
		return nil, err
	}
	labels := parseJudgeAnswer(resp)
	if len(labels) == 0 {
		return nil, fmt.Errorf("no valid facet in judge response (raw: %q)", strings.TrimSpace(resp))
	}
	return labels, nil
}

// parseJudgeAnswer extracts the set of valid facet labels from a judge
// response. Tolerant of common LLM noise: leading "Facet:" prefixes, code
// fences, quotes, comma or newline separators. Returns the labels in their
// first-seen order, deduplicated.
func parseJudgeAnswer(s string) []string {
	s = strings.TrimSpace(s)
	// Strip a leading prefix like "Facet:" or "Labels:".
	lower := strings.ToLower(s)
	for _, prefix := range []string{"facets:", "facet:", "labels:", "label:", "answer:"} {
		if strings.HasPrefix(lower, prefix) {
			s = strings.TrimSpace(s[len(prefix):])
			lower = strings.ToLower(s)
		}
	}
	// Trim code-fence and quote noise around the whole string.
	s = strings.Trim(s, "`'\"")

	// Tokenize on whitespace, commas, semicolons, and slashes.
	tokens := strings.FieldsFunc(strings.ToLower(s), func(r rune) bool {
		switch r {
		case ' ', '\t', '\n', ',', ';', '/', '\r':
			return true
		}
		return false
	})

	seen := map[string]bool{}
	var out []string
	for _, t := range tokens {
		t = strings.Trim(t, "`'\".")
		if t == "" {
			continue
		}
		if !isValidFacet(t) {
			continue // ignore unrecognised tokens rather than failing the whole answer
		}
		if seen[t] {
			continue
		}
		seen[t] = true
		out = append(out, t)
	}
	return out
}

func isValidFacet(s string) bool {
	for _, f := range classifier.AllFacets {
		if string(f) == s {
			return true
		}
	}
	return false
}

// ---------- score (multi-label) ----------

func scoreCmd() *cobra.Command {
	var input string
	var format string

	cmd := &cobra.Command{
		Use:   "score",
		Short: "Compute multi-label per-facet metrics, set-level metrics, and taxonomy diagnostics",
		Long: `score reads a labelled calibration corpus (JSONL) and reports calibration
metrics under the v1.1 multi-label rubric.

For every prediction column that overlaps with gold_labels:
  - per-facet binary precision, recall, F1, support (each facet is a
    separate binary classification problem)
  - macro-F1 (unweighted mean across facets present in the gold)
  - micro-F1 (computed on aggregate TP/FP/FN)
  - exact-match accuracy (predicted set == gold set)
  - mean Jaccard (|G ∩ P| / |G ∪ P|, 1.0 = perfect)
  - mean Hamming loss (fraction of facets wrong per item, 0.0 = perfect)
  - macro Cohen's kappa, treating each facet as an independent
    binary annotation problem

When both llm_labels and judge_labels are present, also reports their
pairwise set-membership agreement on items without gold.

Taxonomy diagnostics (computed on gold_labels alone):
  - cardinality histogram: how many items have 1, 2, 3, ... facets?
  - co-occurrence matrix: count of items where {f_i, f_j} ⊆ gold
  - singleton rate per facet: how often does each facet appear alone?
  - PMI per pair: pointwise mutual information; high values flag
    facet pairs that occur together more than chance would predict

Use --format json for a machine-readable dump.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runScore(input, format)
		},
	}
	cmd.Flags().StringVarP(&input, "input", "i", "", "input JSONL corpus with gold_labels filled (required)")
	cmd.Flags().StringVarP(&format, "format", "f", "text", "output format: text or json")
	_ = cmd.MarkFlagRequired("input")
	return cmd
}

func runScore(input, format string) error {
	items, err := readJSONL(input)
	if err != nil {
		return fmt.Errorf("read %s: %w", input, err)
	}

	report := &scoreReport{
		Input:         input,
		TotalItems:    len(items),
		LabelsPresent: map[string]int{},
	}
	var golded []calibrateItem
	for _, it := range items {
		if len(it.LLMLabels) > 0 {
			report.LabelsPresent["llm"]++
		}
		if len(it.JudgeLabels) > 0 {
			report.LabelsPresent["judge"]++
		}
		if len(it.GoldLabels) == 0 {
			continue
		}
		valid, invalid := splitValid(it.GoldLabels)
		if len(invalid) > 0 {
			report.InvalidGolds = append(report.InvalidGolds,
				fmt.Sprintf("%s: invalid facet(s) %v in gold_labels", it.ID, invalid))
			continue
		}
		it.GoldLabels = valid
		golded = append(golded, it)
	}
	report.ScoredItems = len(golded)

	if len(golded) > 0 {
		if anyLabelSet(golded, func(it calibrateItem) []string { return it.LLMLabels }) {
			report.LLMvsGold = computeMulti(golded, func(it calibrateItem) []string { return it.LLMLabels })
		}
		if anyLabelSet(golded, func(it calibrateItem) []string { return it.JudgeLabels }) {
			report.JudgeVsGold = computeMulti(golded, func(it calibrateItem) []string { return it.JudgeLabels })
		}
		report.Taxonomy = computeTaxonomy(golded)
	}

	// Judge vs LLM agreement does not require gold.
	if anyLabelSet(items, func(it calibrateItem) []string { return it.LLMLabels }) &&
		anyLabelSet(items, func(it calibrateItem) []string { return it.JudgeLabels }) {
		report.JudgeVsLLM = computePairAgreement(items,
			func(it calibrateItem) []string { return it.LLMLabels },
			func(it calibrateItem) []string { return it.JudgeLabels })
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

// scoreReport is the top-level result for --format json.
type scoreReport struct {
	Input         string         `json:"input"`
	TotalItems    int            `json:"total_items"`
	ScoredItems   int            `json:"scored_items"`
	LabelsPresent map[string]int `json:"labels_present"`
	InvalidGolds  []string       `json:"invalid_golds,omitempty"`

	LLMvsGold   *multiComparison `json:"llm_vs_gold,omitempty"`
	JudgeVsGold *multiComparison `json:"judge_vs_gold,omitempty"`
	JudgeVsLLM  *pairAgreement   `json:"judge_vs_llm,omitempty"`

	Taxonomy *taxonomyReport `json:"taxonomy,omitempty"`
}

type multiComparison struct {
	N            int                               `json:"n"`
	PerFacet     map[classifier.Facet]facetMetrics `json:"per_facet"`
	MacroF1      float64                           `json:"macro_f1"`
	MicroF1      float64                           `json:"micro_f1"`
	ExactMatch   float64                           `json:"exact_match"`
	MeanJaccard  float64                           `json:"mean_jaccard"`
	MeanHamming  float64                           `json:"mean_hamming_loss"`
	MacroKappa   float64                           `json:"macro_kappa"`
}

type facetMetrics struct {
	Precision float64 `json:"precision"`
	Recall    float64 `json:"recall"`
	F1        float64 `json:"f1"`
	Support   int     `json:"support"`
	Kappa     float64 `json:"kappa"`
	TP        int     `json:"tp"`
	FP        int     `json:"fp"`
	FN        int     `json:"fn"`
	TN        int     `json:"tn"`
}

type pairAgreement struct {
	N           int     `json:"n"`
	ExactMatch  float64 `json:"exact_match"`
	MeanJaccard float64 `json:"mean_jaccard"`
}

type taxonomyReport struct {
	N             int                                                    `json:"n"`
	Cardinality   map[int]int                                            `json:"cardinality"`
	SingletonRate map[classifier.Facet]float64                           `json:"singleton_rate"`
	Cooccurrence  map[classifier.Facet]map[classifier.Facet]int          `json:"cooccurrence"`
	PMI           map[classifier.Facet]map[classifier.Facet]float64      `json:"pmi"`
}

// ---------- score helpers ----------

func anyLabelSet(items []calibrateItem, get func(calibrateItem) []string) bool {
	for _, it := range items {
		if len(get(it)) > 0 {
			return true
		}
	}
	return false
}

// splitValid partitions a label slice into known and unknown facets.
func splitValid(labels []string) (valid []string, invalid []string) {
	for _, l := range labels {
		if isValidFacet(l) {
			valid = append(valid, l)
		} else {
			invalid = append(invalid, l)
		}
	}
	return valid, invalid
}

// asSet converts a slice to a presence map keyed by facet, dropping
// unknown labels. The boolean is the membership marker.
func asSet(labels []string) map[classifier.Facet]bool {
	out := map[classifier.Facet]bool{}
	for _, l := range labels {
		if isValidFacet(l) {
			out[classifier.Facet(l)] = true
		}
	}
	return out
}

func jaccard(a, b []string) float64 {
	sa := asSet(a)
	sb := asSet(b)
	if len(sa) == 0 && len(sb) == 0 {
		return 1.0
	}
	inter := 0
	for f := range sa {
		if sb[f] {
			inter++
		}
	}
	union := len(sa) + len(sb) - inter
	if union == 0 {
		return 1.0
	}
	return float64(inter) / float64(union)
}

func hammingLoss(pred, gold []string) float64 {
	sp := asSet(pred)
	sg := asSet(gold)
	wrong := 0
	for _, f := range classifier.AllFacets {
		if sp[f] != sg[f] {
			wrong++
		}
	}
	return float64(wrong) / float64(len(classifier.AllFacets))
}

func computeMulti(golded []calibrateItem, predict func(calibrateItem) []string) *multiComparison {
	type itemSets struct{ pred, gold map[classifier.Facet]bool }
	var sets []itemSets
	for _, it := range golded {
		preds := predict(it)
		if len(preds) == 0 {
			continue
		}
		sets = append(sets, itemSets{pred: asSet(preds), gold: asSet(it.GoldLabels)})
	}
	if len(sets) == 0 {
		return nil
	}

	tp := map[classifier.Facet]int{}
	fp := map[classifier.Facet]int{}
	fn := map[classifier.Facet]int{}
	tn := map[classifier.Facet]int{}
	for _, s := range sets {
		for _, f := range classifier.AllFacets {
			gp, gg := s.pred[f], s.gold[f]
			switch {
			case gp && gg:
				tp[f]++
			case gp && !gg:
				fp[f]++
			case !gp && gg:
				fn[f]++
			default:
				tn[f]++
			}
		}
	}

	perFacet := map[classifier.Facet]facetMetrics{}
	var macroF1Sum float64
	var macroKSum float64
	var macroCount int
	totalTP, totalFP, totalFN := 0, 0, 0
	for _, f := range classifier.AllFacets {
		support := tp[f] + fn[f]
		precision := safeDiv(tp[f], tp[f]+fp[f])
		recall := safeDiv(tp[f], tp[f]+fn[f])
		f1 := harmonic(precision, recall)
		k := kappaForBinary(tp[f], fp[f], fn[f], tn[f])

		perFacet[f] = facetMetrics{
			Precision: precision, Recall: recall, F1: f1, Support: support, Kappa: k,
			TP: tp[f], FP: fp[f], FN: fn[f], TN: tn[f],
		}
		totalTP += tp[f]
		totalFP += fp[f]
		totalFN += fn[f]
		if support > 0 {
			macroF1Sum += f1
			macroKSum += k
			macroCount++
		}
	}

	microP := safeDiv(totalTP, totalTP+totalFP)
	microR := safeDiv(totalTP, totalTP+totalFN)
	microF1 := harmonic(microP, microR)

	exactCount := 0
	jaccardSum := 0.0
	hammingSum := 0.0
	for _, s := range sets {
		if setsEqual(s.pred, s.gold) {
			exactCount++
		}
		jaccardSum += jaccardSets(s.pred, s.gold)
		hammingSum += hammingSets(s.pred, s.gold)
	}

	return &multiComparison{
		N:           len(sets),
		PerFacet:    perFacet,
		MacroF1:     ifPositive(macroCount, macroF1Sum/float64(macroCount)),
		MicroF1:     microF1,
		ExactMatch:  float64(exactCount) / float64(len(sets)),
		MeanJaccard: jaccardSum / float64(len(sets)),
		MeanHamming: hammingSum / float64(len(sets)),
		MacroKappa:  ifPositive(macroCount, macroKSum/float64(macroCount)),
	}
}

func computePairAgreement(items []calibrateItem, a, b func(calibrateItem) []string) *pairAgreement {
	var n int
	exact := 0
	jaccardSum := 0.0
	for _, it := range items {
		la, lb := a(it), b(it)
		if len(la) == 0 || len(lb) == 0 {
			continue
		}
		sa, sb := asSet(la), asSet(lb)
		if setsEqual(sa, sb) {
			exact++
		}
		jaccardSum += jaccardSets(sa, sb)
		n++
	}
	if n == 0 {
		return nil
	}
	return &pairAgreement{
		N:           n,
		ExactMatch:  float64(exact) / float64(n),
		MeanJaccard: jaccardSum / float64(n),
	}
}

func computeTaxonomy(golded []calibrateItem) *taxonomyReport {
	n := len(golded)
	if n == 0 {
		return nil
	}
	cardinality := map[int]int{}
	facetCount := map[classifier.Facet]int{}
	singletonCount := map[classifier.Facet]int{}
	cooc := map[classifier.Facet]map[classifier.Facet]int{}
	for _, f := range classifier.AllFacets {
		cooc[f] = map[classifier.Facet]int{}
	}
	for _, it := range golded {
		set := asSet(it.GoldLabels)
		cardinality[len(set)]++
		for f := range set {
			facetCount[f]++
			if len(set) == 1 {
				singletonCount[f]++
			}
		}
		// Co-occurrence (unordered pairs, also self-counts on diagonal).
		for f1 := range set {
			for f2 := range set {
				cooc[f1][f2]++
			}
		}
	}

	singletonRate := map[classifier.Facet]float64{}
	for _, f := range classifier.AllFacets {
		singletonRate[f] = safeDiv(singletonCount[f], facetCount[f])
	}

	pmi := map[classifier.Facet]map[classifier.Facet]float64{}
	N := float64(n)
	for _, f1 := range classifier.AllFacets {
		pmi[f1] = map[classifier.Facet]float64{}
		for _, f2 := range classifier.AllFacets {
			joint := float64(cooc[f1][f2]) / N
			pf1 := float64(facetCount[f1]) / N
			pf2 := float64(facetCount[f2]) / N
			if joint == 0 || pf1 == 0 || pf2 == 0 {
				pmi[f1][f2] = 0
				continue
			}
			pmi[f1][f2] = math.Log2(joint / (pf1 * pf2))
		}
	}

	return &taxonomyReport{
		N:             n,
		Cardinality:   cardinality,
		SingletonRate: singletonRate,
		Cooccurrence:  cooc,
		PMI:           pmi,
	}
}

// ---------- math helpers ----------

func safeDiv(num, den int) float64 {
	if den == 0 {
		return 0
	}
	return float64(num) / float64(den)
}

func harmonic(p, r float64) float64 {
	if p+r == 0 {
		return 0
	}
	return 2 * p * r / (p + r)
}

func ifPositive(count int, val float64) float64 {
	if count == 0 {
		return 0
	}
	return val
}

func setsEqual(a, b map[classifier.Facet]bool) bool {
	if len(a) != len(b) {
		return false
	}
	for k := range a {
		if !b[k] {
			return false
		}
	}
	return true
}

func jaccardSets(a, b map[classifier.Facet]bool) float64 {
	if len(a) == 0 && len(b) == 0 {
		return 1.0
	}
	inter := 0
	for k := range a {
		if b[k] {
			inter++
		}
	}
	union := len(a) + len(b) - inter
	if union == 0 {
		return 1.0
	}
	return float64(inter) / float64(union)
}

func hammingSets(pred, gold map[classifier.Facet]bool) float64 {
	wrong := 0
	for _, f := range classifier.AllFacets {
		if pred[f] != gold[f] {
			wrong++
		}
	}
	return float64(wrong) / float64(len(classifier.AllFacets))
}

// kappaForBinary returns Cohen's kappa for a 2x2 contingency table of a
// single facet membership decision. Saturates to 1.0 when expected
// agreement equals 1 (all items have the same label on both sides).
func kappaForBinary(tp, fp, fn, tn int) float64 {
	n := float64(tp + fp + fn + tn)
	if n == 0 {
		return 0
	}
	po := float64(tp+tn) / n
	pa := float64(tp+fp) / n
	pb := float64(tp+fn) / n
	pe := pa*pb + (1-pa)*(1-pb)
	if pe >= 1.0 {
		return 1.0
	}
	return (po - pe) / (1.0 - pe)
}

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

// ---------- render ----------

func renderScoreText(r *scoreReport) {
	bold := color.New(color.Bold).SprintFunc()
	faint := color.New(color.Faint).SprintFunc()

	fmt.Printf("%s %s\n", bold("Corpus:"), r.Input)
	fmt.Printf("  total items        : %d\n", r.TotalItems)
	fmt.Printf("  with gold_labels   : %d\n", r.ScoredItems)
	if c := r.LabelsPresent["llm"]; c > 0 {
		fmt.Printf("  with llm_labels    : %d\n", c)
	}
	if c := r.LabelsPresent["judge"]; c > 0 {
		fmt.Printf("  with judge_labels  : %d\n", c)
	}
	if len(r.InvalidGolds) > 0 {
		fmt.Println(color.YellowString("  invalid gold rows:"))
		for _, s := range r.InvalidGolds {
			fmt.Printf("    %s\n", s)
		}
	}

	if r.LLMvsGold != nil {
		fmt.Println()
		fmt.Println(bold("=== llm_labels vs gold ==="))
		renderMultiComparison(r.LLMvsGold)
	}
	if r.JudgeVsGold != nil {
		fmt.Println()
		fmt.Println(bold("=== judge_labels vs gold ==="))
		renderMultiComparison(r.JudgeVsGold)
	}
	if r.JudgeVsLLM != nil {
		fmt.Println()
		fmt.Println(bold("=== judge_labels vs llm_labels (no gold required) ==="))
		fmt.Printf("  N            : %d\n", r.JudgeVsLLM.N)
		fmt.Printf("  exact match  : %.3f\n", r.JudgeVsLLM.ExactMatch)
		fmt.Printf("  mean Jaccard : %.3f\n", r.JudgeVsLLM.MeanJaccard)
	}
	if r.Taxonomy != nil {
		fmt.Println()
		fmt.Println(bold("=== Taxonomy diagnostics (from gold_labels) ==="))
		renderTaxonomy(r.Taxonomy)
		_ = faint
	}
}

func renderMultiComparison(c *multiComparison) {
	bold := color.New(color.Bold).SprintFunc()
	faint := color.New(color.Faint).SprintFunc()

	fmt.Printf("  N: %d   exact-match: %.3f   Jaccard: %.3f   Hamming: %.3f\n",
		c.N, c.ExactMatch, c.MeanJaccard, c.MeanHamming)
	fmt.Printf("  macro-F1: %.3f   micro-F1: %.3f   macro-κ: %.3f %s\n\n",
		c.MacroF1, c.MicroF1, c.MacroKappa,
		faint("("+kappaInterpretation(c.MacroKappa)+")"))

	fmt.Println(bold("  Per-facet metrics (each facet = independent binary task):"))
	fmt.Printf("    %-14s %9s %7s %7s %7s %8s\n", "facet", "precision", "recall", "F1", "κ", "support")
	for _, f := range classifier.AllFacets {
		m := c.PerFacet[f]
		fmt.Printf("    %-14s %9.3f %7.3f %7.3f %7.3f %8d\n",
			string(f), m.Precision, m.Recall, m.F1, m.Kappa, m.Support)
	}
}

func renderTaxonomy(t *taxonomyReport) {
	bold := color.New(color.Bold).SprintFunc()

	fmt.Printf("  N items: %d\n\n", t.N)

	// Cardinality histogram.
	fmt.Println(bold("  Cardinality histogram (facets per item):"))
	maxK := 0
	for k := range t.Cardinality {
		if k > maxK {
			maxK = k
		}
	}
	for k := 1; k <= maxK; k++ {
		n := t.Cardinality[k]
		if n == 0 {
			continue
		}
		bar := strings.Repeat("█", n*20/max1(t.N))
		fmt.Printf("    %d facet%s: %4d  %s\n", k, plural(k), n, bar)
	}

	// Singleton rate.
	fmt.Println()
	fmt.Println(bold("  Singleton rate (P(only this facet | facet appears)):"))
	for _, f := range classifier.AllFacets {
		fmt.Printf("    %-14s %.2f\n", string(f), t.SingletonRate[f])
	}

	// Co-occurrence matrix.
	fmt.Println()
	fmt.Println(bold("  Co-occurrence matrix (counts):"))
	fmt.Printf("    %-14s", "")
	for _, f := range classifier.AllFacets {
		fmt.Printf(" %6s", abbreviateFacet(f))
	}
	fmt.Println()
	for _, f1 := range classifier.AllFacets {
		fmt.Printf("    %-14s", string(f1))
		for _, f2 := range classifier.AllFacets {
			fmt.Printf(" %6d", t.Cooccurrence[f1][f2])
		}
		fmt.Println()
	}

	// PMI matrix.
	fmt.Println()
	fmt.Println(bold("  PMI (positive = more co-occurrent than chance; 0 = independent):"))
	fmt.Printf("    %-14s", "")
	for _, f := range classifier.AllFacets {
		fmt.Printf(" %7s", abbreviateFacet(f))
	}
	fmt.Println()
	for _, f1 := range classifier.AllFacets {
		fmt.Printf("    %-14s", string(f1))
		for _, f2 := range classifier.AllFacets {
			fmt.Printf(" %7.2f", t.PMI[f1][f2])
		}
		fmt.Println()
	}
}

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

func plural(n int) string {
	if n == 1 {
		return ""
	}
	return "s"
}

func max1(n int) int {
	if n < 1 {
		return 1
	}
	return n
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
