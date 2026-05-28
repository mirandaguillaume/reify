package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/fatih/color"
	"github.com/mirandaguillaume/reify/internal/analyzer"
	yamlloader "github.com/mirandaguillaume/reify/internal/yaml"
	"github.com/mirandaguillaume/reify/pkg/model"
	"github.com/spf13/cobra"
)

// ScoreReport holds the score results for skills and agents.
type ScoreReport struct {
	Skills []analyzer.SkillScore
	Agents []analyzer.AgentScore
}

// RunScore calculates design quality scores for all skills and agents.
func RunScore(skillsDir string, agentsDir string) ScoreReport {
	var skills []model.SkillBehavior
	var report ScoreReport

	if entries, err := os.ReadDir(skillsDir); err == nil {
		for _, entry := range entries {
			if !strings.HasSuffix(entry.Name(), ".skill.yaml") {
				continue
			}
			content, err := os.ReadFile(filepath.Join(skillsDir, entry.Name()))
			if err != nil {
				continue
			}
			skill, err := yamlloader.ParseSkillYAML(string(content))
			if err == nil {
				skills = append(skills, skill)
				report.Skills = append(report.Skills, analyzer.ScoreSkill(skill))
			}
		}
	}

	if agentsDir != "" {
		skillMap := make(map[string]model.SkillBehavior)
		for _, s := range skills {
			skillMap[s.Skill] = s
		}
		if entries, err := os.ReadDir(agentsDir); err == nil {
			for _, entry := range entries {
				if !strings.HasSuffix(entry.Name(), ".agent.yaml") {
					continue
				}
				content, err := os.ReadFile(filepath.Join(agentsDir, entry.Name()))
				if err != nil {
					continue
				}
				agent, err := yamlloader.ParseAgentYAML(string(content))
				if err == nil {
					resolved := make([]model.SkillBehavior, 0)
					for _, name := range agent.Skills {
						if s, ok := skillMap[name]; ok {
							resolved = append(resolved, s)
						}
					}
					report.Agents = append(report.Agents, analyzer.ScoreAgent(agent, resolved))
				}
			}
		}
	}

	return report
}

func scoreColor(score int) func(string, ...interface{}) string {
	if score >= 80 {
		return color.GreenString
	}
	if score >= 60 {
		return color.YellowString
	}
	return color.RedString
}

func bar(score, maxVal int) string {
	width := 20
	filled := score * width / maxVal
	empty := width - filled
	return color.GreenString(strings.Repeat("\u2588", filled)) + color.HiBlackString(strings.Repeat("\u2591", empty))
}

// PrintScoreReport prints the score report to stdout with colored output.
func PrintScoreReport(report ScoreReport) {
	bold := color.New(color.Bold)

	if len(report.Skills) > 0 {
		fmt.Println()
		bold.Println("  Skills")
		fmt.Println()
		for _, s := range report.Skills {
			c := scoreColor(s.Total)
			fmt.Printf("  %s  %s  %s\n", c("%d", s.Total), bar(s.Total, 100), s.Skill)
			fmt.Println(color.HiBlackString("       context:%d strategy:%d guardrails:%d observability:%d security:%d when_to_use:%d anti_patterns:%d examples:%d",
				s.Breakdown.Context, s.Breakdown.Strategy, s.Breakdown.Guardrails, s.Breakdown.Observability, s.Breakdown.Security,
				s.Breakdown.WhenToUse, s.Breakdown.AntiPatterns, s.Breakdown.Examples))
		}
	}

	if len(report.Agents) > 0 {
		fmt.Println()
		bold.Println("  Agents")
		fmt.Println()
		for _, a := range report.Agents {
			c := scoreColor(a.Total)
			fmt.Printf("  %s  %s  %s\n", c("%d", a.Total), bar(a.Total, 100), a.Agent)
			fmt.Println(color.HiBlackString("       description:%d composition:%d dataFlow:%d orchestration:%d",
				a.Breakdown.Description, a.Breakdown.Composition, a.Breakdown.DataFlow, a.Breakdown.Orchestration))
		}
	}

	if len(report.Skills) == 0 && len(report.Agents) == 0 {
		fmt.Println(color.YellowString("No skills or agents found to score."))
	}
	fmt.Println()
}

func init() {
	var agentsDir string
	scoreCmd := &cobra.Command{
		Use:   "score [path]",
		Short: "Score design quality of skills and agents",
		Args:  cobra.MaximumNArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			path := "skills"
			if len(args) > 0 {
				path = args[0]
			}
			report := RunScore(path, agentsDir)
			PrintScoreReport(report)
		},
	}
	scoreCmd.Flags().StringVarP(&agentsDir, "agents", "a", "agents", "agents directory")
	rootCmd.AddCommand(scoreCmd)
}
