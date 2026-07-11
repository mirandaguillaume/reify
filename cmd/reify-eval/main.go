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
	cmd := &cobra.Command{
		Use:   "run",
		Short: "Run the eval matrix for a fixture (one couple, all formulations)",
		RunE: func(cmd *cobra.Command, args []string) error {
			f, err := loadFixture(fixtureDir)
			if err != nil {
				return err
			}
			var results []FormulationResult
			for name, rule := range f.Formulations {
				rule := rule
				attempt := func() (*AttemptResult, error) {
					return runAttempt(f, rule, model, effort)
				}
				fmt.Fprintf(os.Stderr, "running cell: %s\n", name)
				results = append(results, FormulationResult{Name: name, Cell: runCell(attempt, targetWidth, nMax)})
			}
			fmt.Print(renderMatrix(model, effort, f.Intention, results))
			return nil
		},
	}
	cmd.Flags().StringVarP(&fixtureDir, "fixture", "f", "", "fixture directory (required)")
	cmd.Flags().StringVar(&model, "model", "haiku", "model alias")
	cmd.Flags().StringVar(&effort, "effort", "medium", "effort level (low|medium|high|xhigh|max)")
	cmd.Flags().Float64Var(&targetWidth, "ci-width", 0.15, "target Wilson half-width to stop a cell")
	cmd.Flags().IntVar(&nMax, "n-max", 30, "max attempts per cell")
	_ = cmd.MarkFlagRequired("fixture")
	return cmd
}
