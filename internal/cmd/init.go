package cmd

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/fatih/color"
	"github.com/mirandaguillaume/reify/internal/templates"
	"github.com/spf13/cobra"
)

// InitResult holds the result of an init operation.
type InitResult struct {
	AlreadyInitialized bool
	Path               string
}

// InitProject initializes a Reify project in the given directory.
func InitProject(targetDir string) (InitResult, error) {
	configPath := filepath.Join(targetDir, "reify.yaml")

	if _, err := os.Stat(configPath); err == nil {
		return InitResult{AlreadyInitialized: true, Path: targetDir}, nil
	}

	if err := os.MkdirAll(filepath.Join(targetDir, "skills"), 0755); err != nil {
		return InitResult{}, fmt.Errorf("failed to create skills directory: %w", err)
	}
	if err := os.MkdirAll(filepath.Join(targetDir, "agents"), 0755); err != nil {
		return InitResult{}, fmt.Errorf("failed to create agents directory: %w", err)
	}

	config := "# Reify Project Configuration\nversion: \"0.1.0\"\nskills_dir: skills\nagents_dir: agents\n"
	if err := os.WriteFile(configPath, []byte(config), 0644); err != nil {
		return InitResult{}, fmt.Errorf("failed to write reify.yaml: %w", err)
	}

	tmpl, err := templates.SkillTemplate()
	if err == nil {
		if err := os.WriteFile(filepath.Join(targetDir, "skills", "example.skill.yaml"), tmpl, 0644); err != nil {
			return InitResult{}, fmt.Errorf("failed to write example skill: %w", err)
		}
	}

	return InitResult{AlreadyInitialized: false, Path: targetDir}, nil
}

func init() {
	initCmd := &cobra.Command{
		Use:   "init [path]",
		Short: "Initialize a Reify project in the current directory",
		Args:  cobra.MaximumNArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			path := "."
			if len(args) > 0 {
				path = args[0]
			}
			result, err := InitProject(path)
			if err != nil {
				fmt.Println(color.RedString("Init failed: %v", err))
				os.Exit(1)
			}
			if result.AlreadyInitialized {
				fmt.Println(color.YellowString("Reify project already initialized."))
			} else {
				fmt.Println(color.GreenString("Reify project initialized at"), result.Path)
				fmt.Println("  Created: reify.yaml, skills/, agents/")
			}
		},
	}
	rootCmd.AddCommand(initCmd)
}
