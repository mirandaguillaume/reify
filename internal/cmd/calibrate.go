package cmd

import (
	"encoding/json"
	"fmt"
	"math/rand"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/fatih/color"
	"github.com/mirandaguillaume/reify/internal/classifier"
	"github.com/spf13/cobra"
)

// calibrateCmd groups subcommands for building the LLM classifier calibration
// battery: sample (stratified corpus), judge (LLM-as-Judge labelling),
// score (precision/recall/F1 vs gold).
func init() {
	calibrateCmd := &cobra.Command{
		Use:   "calibrate",
		Short: "Build and score a calibration battery for the LLM facet classifier",
		Long: `calibrate provides tooling to measure how well the LLM classifier agrees
with human labels per facet.

Subcommands:
  sample   Stratified-sample N items per facet from existing classify.log files
  judge    Have a stronger LLM assign a second opinion (LLM-as-Judge)
  score    Compute precision/recall/F1 and a confusion matrix vs gold labels

See docs/calibration/rubric.md for the canonical facet definitions.`,
	}

	calibrateCmd.AddCommand(calibrateSampleCmd())
	rootCmd.AddCommand(calibrateCmd)
}

// calibrateItem is the JSONL row format used across sample/judge/score.
// Empty fields are omitted on write to keep the unlabelled corpus tidy.
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

func calibrateSampleCmd() *cobra.Command {
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
			return runCalibrateSample(sourceDir, perFacet, output, seed)
		},
	}

	cmd.Flags().StringVar(&sourceDir, "source", "", "directory tree containing classify.log files (required)")
	cmd.Flags().IntVar(&perFacet, "n", 40, "items to sample per facet")
	cmd.Flags().StringVarP(&output, "output", "o", "calibration-corpus.jsonl", "output JSONL path")
	cmd.Flags().Int64Var(&seed, "seed", 42, "random seed for reproducible sampling")
	_ = cmd.MarkFlagRequired("source")
	return cmd
}

func runCalibrateSample(sourceDir string, perFacet int, output string, seed int64) error {
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
		// Fisher–Yates partial shuffle on the indices for deterministic
		// stratified random sampling.
		idx := rng.Perm(len(pool))[:take]
		sort.Ints(idx)
		for _, i := range idx {
			sampled = append(sampled, pool[i])
		}
		fmt.Printf("  %-14s %4d sampled from %d available\n", facet, take, len(pool))
	}

	// Stable ids so re-sampling with the same seed doesn't lose human labels.
	for i := range sampled {
		sampled[i].ID = fmt.Sprintf("%s-%04d", sampled[i].LLMLabel, i+1)
	}

	if err := writeJSONL(output, sampled); err != nil {
		return fmt.Errorf("write %s: %w", output, err)
	}
	fmt.Printf("\nWrote %d items to %s\n", len(sampled), output)
	fmt.Println(color.New(color.Faint).Sprint("Next: fill gold_label in each row, then `reify calibrate judge` / `reify calibrate score`."))
	return nil
}

// findClassifyLogs walks the source tree returning every file literally
// named classify.log. Restricted to that name to avoid scooping up unrelated
// logs.
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

// loadClassifyLog reads a single classify.log (JSON array of per-file
// classifications) and flattens it into calibrateItems with one row per
// classified instruction.
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

	// source_repo is inferred from the path layout: <root>/<repo>/.../classify.log
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

// inferRepo extracts a short repo identifier from a classify.log path.
// Convention from the dogfood script: <root>/logs/<repo>/classify.log.
// Falls back to the path segment immediately after the source root.
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
