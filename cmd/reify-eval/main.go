// Command reify-eval measures whether a rule's formulation changes an
// agent's obedience, by running the real harness against trapped git
// fixtures in a disposable Docker sandbox. See
// docs/superpowers/specs/2026-06-02-eval-harness-design.md.
package main

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

func main() {
	root := &cobra.Command{
		Use:   "reify-eval",
		Short: "Fidelity eval harness: measure rule-formulation effectiveness",
	}
	root.AddCommand(runCmd())
	if err := root.Execute(); err != nil {
		os.Exit(1)
	}
}

func runCmd() *cobra.Command {
	var fixtureDir, model, effort string
	var targetWidth float64
	var nMax int
	var fillerTokens int
	var positions []float64
	cmd := &cobra.Command{
		Use:   "run",
		Short: "Run the eval matrix for a fixture (one couple, all formulations)",
		RunE: func(cmd *cobra.Command, args []string) error {
			f, err := loadFixture(fixtureDir)
			if err != nil {
				return err
			}
			// LITM sweep: with filler, run the whole formulation matrix once
			// per rule position; without filler, a single pass (position
			// irrelevant). Each pass is labelled with its position.
			sweep := []float64{0}
			if fillerTokens > 0 {
				sweep = positions
			}
			for _, pos := range sweep {
				litm := litmParams{FillerTokens: fillerTokens, Position: pos}
				var results []FormulationResult
				for name, rule := range f.Formulations {
					rule := rule
					attempt := func() (*AttemptResult, error) {
						return runAttempt(f, rule, model, effort, litm)
					}
					label := name
					if fillerTokens > 0 {
						label = fmt.Sprintf("%s@%s", name, litmPositionName(pos))
						fmt.Fprintf(os.Stderr, "running cell: %s (filler=%d tok, pos=%s)\n", name, fillerTokens, litmPositionName(pos))
					} else {
						fmt.Fprintf(os.Stderr, "running cell: %s\n", name)
					}
					results = append(results, FormulationResult{Name: label, Cell: runCell(attempt, targetWidth, nMax)})
				}
				header := f.Intention
				if fillerTokens > 0 {
					header = fmt.Sprintf("%s  filler=%dtok  position=%s", f.Intention, fillerTokens, litmPositionName(pos))
				}
				fmt.Print(renderMatrix(model, effort, header, results))
				fmt.Println()
			}
			return nil
		},
	}
	cmd.Flags().StringVarP(&fixtureDir, "fixture", "f", "", "fixture directory (required)")
	cmd.Flags().StringVar(&model, "model", "haiku", "model alias")
	cmd.Flags().StringVar(&effort, "effort", "medium", "effort level (low|medium|high|xhigh|max)")
	cmd.Flags().Float64Var(&targetWidth, "ci-width", 0.15, "target Wilson half-width to stop a cell")
	cmd.Flags().IntVar(&nMax, "n-max", 30, "max attempts per cell")
	cmd.Flags().IntVar(&fillerTokens, "filler-tokens", 0, "lost-in-the-middle: approx filler tokens around the rule (0 = no filler)")
	cmd.Flags().Float64SliceVar(&positions, "positions", []float64{0.0, 0.5, 1.0}, "lost-in-the-middle: rule positions to sweep (0=head,0.5=mid,1=tail)")
	_ = cmd.MarkFlagRequired("fixture")
	return cmd
}
