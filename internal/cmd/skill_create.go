package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"

	"github.com/fatih/color"
	"github.com/mirandaguillaume/reify/pkg/model"
	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"
)

var validName = regexp.MustCompile(`^[a-z0-9][a-z0-9_-]*$`)

// CreateSkillOptions holds optional parameters for skill creation.
type CreateSkillOptions struct {
	Tools  []string
	Memory string
}

// CreateSkillResult holds the outcome of a skill creation.
type CreateSkillResult struct {
	Success bool
	Path    string
	Error   string
}

// CreateSkill creates a new skill YAML file in the given project directory.
func CreateSkill(projectDir, name string, opts CreateSkillOptions) CreateSkillResult {
	if !validName.MatchString(name) {
		return CreateSkillResult{Error: fmt.Sprintf("Invalid skill name %q. Use lowercase letters, numbers, hyphens, and underscores only.", name)}
	}

	skillPath := filepath.Join(projectDir, "skills", name+".skill.yaml")
	if _, err := os.Stat(skillPath); err == nil {
		return CreateSkillResult{Error: fmt.Sprintf("Skill %q already exists at %s", name, skillPath)}
	}

	memory := model.MemoryType("short-term")
	if opts.Memory != "" {
		memory = model.MemoryType(opts.Memory)
	}

	tools := opts.Tools
	if tools == nil {
		tools = []string{}
	}

	skill := model.SkillBehavior{
		Skill:   name,
		Version: "0.1.0",
		Context: model.ContextFacet{
			Consumes: []string{},
			Produces: []string{},
			Memory:   memory,
		},
		Strategy: model.StrategyFacet{
			Tools:    tools,
			Approach: "sequential",
			Steps:    []string{},
		},
		Guardrails: []model.GuardrailRule{},
		Observability: model.ObservabilityFacet{
			TraceLevel: model.TraceLevelMinimal,
			Metrics:    []string{},
		},
		Security: model.SecurityFacet{
			Filesystem: model.AccessNone,
			Network:    model.NetworkNone,
			Secrets:    []string{},
		},
		Negotiation: model.NegotiationFacet{
			FileConflicts: model.NegotiationYield,
			Priority:      0,
		},
	}

	data, err := yaml.Marshal(skill)
	if err != nil {
		return CreateSkillResult{Error: fmt.Sprintf("Failed to marshal skill: %v", err)}
	}

	os.MkdirAll(filepath.Dir(skillPath), 0755)
	if err := os.WriteFile(skillPath, data, 0644); err != nil {
		return CreateSkillResult{Error: fmt.Sprintf("Failed to write file: %v", err)}
	}

	return CreateSkillResult{Success: true, Path: skillPath}
}

func init() {
	var tools []string
	var memory string

	skillCmd := &cobra.Command{
		Use:   "skill",
		Short: "Manage skills",
	}

	createCmd := &cobra.Command{
		Use:   "create <name>",
		Short: "Create a new skill",
		Args:  cobra.ExactArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			result := CreateSkill(".", args[0], CreateSkillOptions{Tools: tools, Memory: memory})
			if result.Success {
				fmt.Println(color.GreenString("Skill %q created at %s", args[0], result.Path))
			} else {
				fmt.Println(color.RedString(result.Error))
			}
		},
	}

	createCmd.Flags().StringSliceVarP(&tools, "tools", "t", nil, "tools the skill can use")
	createCmd.Flags().StringVarP(&memory, "memory", "m", "short-term", "memory type")

	skillCmd.AddCommand(createCmd)
	rootCmd.AddCommand(skillCmd)
}
