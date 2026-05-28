package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/fatih/color"
	"github.com/mirandaguillaume/reify/internal/linter"
	yamlloader "github.com/mirandaguillaume/reify/internal/yaml"
	"github.com/spf13/cobra"
)

// LintCommandResult holds the aggregated results of linting a directory.
type LintCommandResult struct {
	TotalFiles  int
	TotalIssues int
	Errors      int
	Warnings    int
	Results     map[string][]linter.LintResult
}

// LintDirectory scans a directory for .skill.yaml files and lints each one.
func LintDirectory(skillsDir string) LintCommandResult {
	result := LintCommandResult{Results: make(map[string][]linter.LintResult)}

	entries, err := os.ReadDir(skillsDir)
	if err != nil {
		return result
	}

	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".skill.yaml") {
			continue
		}
		result.TotalFiles++

		content, err := os.ReadFile(filepath.Join(skillsDir, entry.Name()))
		if err != nil {
			continue
		}

		skill, err := yamlloader.ParseSkillYAML(string(content))
		if err != nil {
			result.Results[entry.Name()] = []linter.LintResult{{
				Rule:     "valid-schema",
				Severity: linter.SeverityError,
				Message:  fmt.Sprintf("Invalid skill file: %v", err),
				Facet:    "schema",
			}}
			result.Errors++
			result.TotalIssues++
			continue
		}

		issues := linter.LintSkill(skill)
		if len(issues) > 0 {
			result.Results[entry.Name()] = issues
			for _, issue := range issues {
				result.TotalIssues++
				if issue.Severity == linter.SeverityError {
					result.Errors++
				}
				if issue.Severity == linter.SeverityWarning {
					result.Warnings++
				}
			}
		}
	}

	return result
}

// PrintLintResults prints lint results to stdout with colored output.
func PrintLintResults(result LintCommandResult) {
	if result.TotalFiles == 0 {
		fmt.Println(color.YellowString("No skill files found."))
		return
	}

	for file, issues := range result.Results {
		fmt.Printf("\n%s\n", color.New(color.Bold).Sprint(file))
		for _, issue := range issues {
			icon := color.YellowString("!")
			if issue.Severity == linter.SeverityError {
				icon = color.RedString("x")
			}
			fmt.Printf("  %s [%s] %s\n", icon, issue.Facet, issue.Message)
		}
	}

	fmt.Printf("\nScanned %d skills: %d errors, %d warnings\n", result.TotalFiles, result.Errors, result.Warnings)

	if result.Errors > 0 {
		fmt.Println(color.RedString("\nLint failed."))
	} else if result.Warnings > 0 {
		fmt.Println(color.YellowString("\nLint passed with warnings."))
	} else {
		fmt.Println(color.GreenString("\nLint passed."))
	}
}

func init() {
	lintCmd := &cobra.Command{
		Use:   "lint [path]",
		Short: "Lint skill files for best practices",
		Args:  cobra.MaximumNArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			path := "skills"
			if len(args) > 0 {
				path = args[0]
			}
			result := LintDirectory(path)
			PrintLintResults(result)
			if result.Errors > 0 {
				os.Exit(1)
			}
		},
	}
	rootCmd.AddCommand(lintCmd)
}
