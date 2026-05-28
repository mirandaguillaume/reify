package copilot

import (
	"fmt"
	"strings"

	"github.com/mirandaguillaume/reify/internal/generator"
	"github.com/mirandaguillaume/reify/pkg/model"
)

// ResolveCopilotAgentTools collects and merges Copilot tools from all skills.
func ResolveCopilotAgentTools(skills []model.SkillBehavior) []string {
	var allLists [][]string
	for _, skill := range skills {
		allLists = append(allLists, MapToolsToCopilot(skill.Strategy.Tools))
		allLists = append(allLists, InferCopilotToolsFromSecurity(
			string(skill.Security.Filesystem),
			string(skill.Security.Network),
		))
	}
	return MergeCopilotToolLists(allLists...)
}

// GenerateCopilotAgentMd generates a Copilot .agent.md from an AgentComposition.
func GenerateCopilotAgentMd(agent model.AgentComposition, resolvedSkills []model.SkillBehavior, outputDir string) string {
	var lines []string

	// Frontmatter — orchestrator only needs task tool
	lines = append(lines, "---")
	lines = append(lines, "name: "+agent.Agent)
	if agent.Description != "" {
		lines = append(lines, "description: "+agent.Description)
	}
	if len(resolvedSkills) > 0 {
		lines = append(lines, `tools: ["task"]`)
	}
	lines = append(lines, "---")
	lines = append(lines, "")

	if len(agent.Stages) > 0 {
		// Staged pipeline — metadata table + flat steps
		allSkills := agent.AllSkills()
		totalSkills := len(allSkills)
		lines = append(lines, fmt.Sprintf("You are %s. An orchestrator that coordinates %d specialized subagents.",
			generator.ToTitle(agent.Agent), totalSkills))
		lines = append(lines, "")

		// Pipeline table (metadata only)
		lines = append(lines, "## Pipeline")
		lines = append(lines, "| Stage | Strategy | Skills |")
		lines = append(lines, "|-------|----------|--------|")
		for _, stage := range agent.Stages {
			lines = append(lines, fmt.Sprintf("| %s | %s | %s |",
				stage.Name, stage.Strategy, strings.Join(stage.Skills, ", ")))
		}
		lines = append(lines, "")

		// Flat execution steps
		lines = append(lines, "## Execution")
		lines = append(lines, fmt.Sprintf(
			"Execute %d skills sequentially as independent subagents. Each skill runs in isolation with its own context. Pass the output of each skill as input to the next.", totalSkills))
		lines = append(lines, "")

		for i, skillName := range allSkills {
			skillPath := fmt.Sprintf("%s/skills/%s/SKILL.md", outputDir, skillName)
			lines = append(lines, fmt.Sprintf("### Step %d: %s", i+1, generator.ToTitle(skillName)))

			effort := model.EffortMedium
			for _, s := range resolvedSkills {
				if s.Skill == skillName {
					if s.Strategy.Effort != "" {
						effort = s.Strategy.Effort
					}
					lines = append(lines, "Launch a subagent:")
					lines = append(lines, fmt.Sprintf("- Skill: `%s`", skillPath))
					lines = append(lines, fmt.Sprintf("- Model: %s", EffortToModel(effort)))

					consumes := formatIOPointers("In", s.Context.Consumes)
					produces := formatIOPointers("Out", s.Context.Produces)
					if consumes != "" {
						lines = append(lines, "- "+consumes)
					}
					if produces != "" {
						lines = append(lines, "- "+produces)
					}
					break
				}
			}
			lines = append(lines, "")
		}
	} else {
		// Flat pipeline
		lines = append(lines, fmt.Sprintf("You are %s. An orchestrator that coordinates %d specialized subagents.", generator.ToTitle(agent.Agent), len(agent.Skills)))
		lines = append(lines, "")

		// Orchestration
		lines = append(lines, "## Execution")
		n := len(agent.Skills)
		switch agent.Orchestration {
		case model.OrchestrationSequential:
			lines = append(lines, fmt.Sprintf(
				"Execute %d skills sequentially as independent subagents. Each skill runs in isolation with its own context. Pass the output of each skill as input to the next.", n))
		case model.OrchestrationParallel:
			lines = append(lines, fmt.Sprintf(
				"Launch %d skills as parallel subagents. Each skill runs independently. Collect all results.", n))
		case model.OrchestrationParallelThenMerge:
			lines = append(lines, fmt.Sprintf(
				"Launch %d skills as parallel subagents, then merge their outputs.", n))
		case model.OrchestrationAdaptive:
			lines = append(lines, fmt.Sprintf(
				"Dispatch %d skills as subagents, choosing execution order dynamically based on intermediate results.", n))
		}
		lines = append(lines, "")

		// Skill steps
		for i, skillName := range agent.Skills {
			skillPath := fmt.Sprintf("%s/skills/%s/SKILL.md", outputDir, skillName)
			lines = append(lines, fmt.Sprintf("### Step %d: %s", i+1, generator.ToTitle(skillName)))

			effort := model.EffortMedium
			for _, s := range resolvedSkills {
				if s.Skill == skillName {
					if s.Strategy.Effort != "" {
						effort = s.Strategy.Effort
					}
					lines = append(lines, "Launch a subagent:")
					lines = append(lines, fmt.Sprintf("- Skill: `%s`", skillPath))
					lines = append(lines, fmt.Sprintf("- Model: %s", EffortToModel(effort)))

					consumes := formatIOPointers("In", s.Context.Consumes)
					produces := formatIOPointers("Out", s.Context.Produces)
					if consumes != "" {
						lines = append(lines, "- "+consumes)
					}
					if produces != "" {
						lines = append(lines, "- "+produces)
					}
					break
				}
			}
			lines = append(lines, "")
		}
	}

	// Output
	if len(resolvedSkills) > 0 {
		seen := map[string]bool{}
		var unique []string
		for _, s := range resolvedSkills {
			for _, p := range s.Context.Produces {
				if !seen[p] {
					unique = append(unique, p)
					seen[p] = true
				}
			}
		}
		if len(unique) > 0 {
			lines = append(lines, "## Output")
			lines = append(lines, fmt.Sprintf("Produce a structured report containing: %s.", strings.Join(unique, ", ")))
			lines = append(lines, "")
		}
	}

	return strings.Join(lines, "\n")
}

// formatIOPointers formats I/O labels with contract pointers when available.
func formatIOPointers(label string, names []string) string {
	if len(names) == 0 {
		return ""
	}
	return label + ": " + strings.Join(names, ", ")
}

// GenerateCompactCopilotAgentMd generates a single-file agent with all skills inlined.
func GenerateCompactCopilotAgentMd(agent model.AgentComposition, resolvedSkills []model.SkillBehavior) string {
	var lines []string

	// Frontmatter
	lines = append(lines, "---")
	lines = append(lines, "name: "+agent.Agent)
	if agent.Description != "" {
		lines = append(lines, "description: "+agent.Description)
	}
	if len(resolvedSkills) > 0 {
		tools := ResolveCopilotAgentTools(resolvedSkills)
		hasRead := false
		for _, t := range tools {
			if t == "read" {
				hasRead = true
				break
			}
		}
		if !hasRead {
			tools = append([]string{"read"}, tools...)
		}
		quoted := make([]string, len(tools))
		for i, t := range tools {
			quoted[i] = fmt.Sprintf("%q", t)
		}
		lines = append(lines, fmt.Sprintf("tools: [%s]", strings.Join(quoted, ", ")))
	}
	lines = append(lines, "---")
	lines = append(lines, "")

	// Intro
	lines = append(lines, fmt.Sprintf("You are %s. %s", generator.ToTitle(agent.Agent), agent.Description))
	lines = append(lines, "")

	// Orchestration (1 line)
	n := len(agent.Skills)
	switch agent.Orchestration {
	case model.OrchestrationSequential:
		lines = append(lines, fmt.Sprintf("Execute %d skills in order.", n))
	case model.OrchestrationParallel:
		lines = append(lines, fmt.Sprintf("Execute %d skills concurrently.", n))
	case model.OrchestrationParallelThenMerge:
		lines = append(lines, fmt.Sprintf("Execute %d skills concurrently, then merge outputs.", n))
	case model.OrchestrationAdaptive:
		lines = append(lines, fmt.Sprintf("Choose execution order dynamically for %d skills.", n))
	}
	lines = append(lines, "")

	// Inline skills
	for _, skillName := range agent.Skills {
		for _, s := range resolvedSkills {
			if s.Skill == skillName {
				lines = append(lines, generator.FormatCompactSkill(s))
				break
			}
		}
	}

	// Output
	if len(resolvedSkills) > 0 {
		seen := map[string]bool{}
		var unique []string
		for _, s := range resolvedSkills {
			for _, p := range s.Context.Produces {
				if !seen[p] {
					unique = append(unique, p)
					seen[p] = true
				}
			}
		}
		if len(unique) > 0 {
			lines = append(lines, "## Output")
			lines = append(lines, fmt.Sprintf("Produce a structured report containing: %s.", strings.Join(unique, ", ")))
			lines = append(lines, "")
		}
	}

	return strings.Join(lines, "\n")
}
