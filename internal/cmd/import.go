package cmd

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/fatih/color"
	"github.com/mirandaguillaume/reify/internal/importer"
	"github.com/spf13/cobra"
)

func init() {
	var provider, model, outputDir string
	var minScore int
	var yes, dryRun, force bool

	importCmd := &cobra.Command{
		Use:   "import <source>",
		Short: "Import agent definitions as Reify skill specs",
		Args:  cobra.ExactArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			source := args[0]

			runImport(source, provider, model, outputDir, minScore, yes, dryRun, force)
		},
	}

	importCmd.Flags().StringVarP(&provider, "provider", "p", "", "LLM provider name")
	importCmd.Flags().StringVar(&model, "model", "", "LLM model name (provider-specific)")
	importCmd.Flags().StringVarP(&outputDir, "output", "o", ".", "output directory")
	importCmd.Flags().IntVar(&minScore, "min-score", 60, "minimum quality score")
	importCmd.Flags().BoolVar(&yes, "yes", false, "skip confirmation prompt")
	importCmd.Flags().BoolVar(&dryRun, "dry-run", false, "preview without writing files")
	importCmd.Flags().BoolVar(&force, "force", false, "overwrite existing files")

	rootCmd.AddCommand(importCmd)
}

func runImport(source, providerFlag, modelFlag, outputDir string, minScore int, yes, dryRun, force bool) {
	// 1. Resolve provider: flag → Ollama auto-detect → env keys → error
	llmProvider, providerName, err := selectProvider(providerFlag, modelFlag, false)
	if err != nil {
		fmt.Println(color.RedString("Error: %v", err))
		os.Exit(1)
	}
	_ = providerName

	// 4. Run import pipeline
	result := importer.RunImport(importer.ImportOptions{
		Source:   source,
		Provider: llmProvider,
		MinScore: minScore,
		OutputDir: outputDir,
	})

	if result.Error != "" {
		fmt.Println(color.RedString("Import failed: %s", result.Error))
		os.Exit(1)
	}

	// 5. Show preview
	importer.FormatPreview(result, os.Stdout)

	if dryRun {
		return
	}

	// 6. Prompt for confirmation unless --yes
	if !yes {
		skillCount := len(result.Skills)
		agentCount := 0
		if result.Agent != nil {
			agentCount = 1
		}
		contractCount := len(result.Contracts)
		if contractCount > 0 {
			fmt.Printf("Write %d skill(s) + %d agent(s) + %d contract(s)? [y/N] ", skillCount, agentCount, contractCount)
		} else {
			fmt.Printf("Write %d skill(s) + %d agent(s)? [y/N] ", skillCount, agentCount)
		}
		scanner := bufio.NewScanner(os.Stdin)
		scanner.Scan()
		answer := strings.TrimSpace(strings.ToLower(scanner.Text()))
		if answer != "y" && answer != "yes" {
			fmt.Println("Aborted.")
			return
		}
	}

	// 7. If --force, remove existing files before writing
	if force {
		removeExistingFiles(result, outputDir)
	}

	// 8. Write files
	written, err := importer.WriteImportResult(result, outputDir)
	if err != nil {
		fmt.Println(color.RedString("Write failed: %v", err))
		os.Exit(1)
	}

	// 9. Print written files
	for _, path := range written {
		fmt.Println(color.GreenString("  wrote %s", path))
	}
}


// removeExistingFiles removes files that would conflict with WriteImportResult.
func removeExistingFiles(result importer.ImportResult, outputDir string) {
	for _, sr := range result.Skills {
		name := sr.Skill.Skill
		if name == "" {
			name = "unknown"
		}
		path := filepath.Join(outputDir, "skills", name+".skill.yaml")
		os.Remove(path)
	}
	if result.Agent != nil {
		name := result.Agent.Agent.Agent
		if name == "" {
			name = "unknown"
		}
		path := filepath.Join(outputDir, "agents", name+".agent.yaml")
		os.Remove(path)
	}
	for name := range result.Contracts {
		path := filepath.Join(outputDir, "contracts", name+".md")
		os.Remove(path)
	}
}
