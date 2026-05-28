package analyzer

import (
	"fmt"
	"strings"

	"github.com/mirandaguillaume/reify/pkg/model"
)

// IssueType represents the type of dependency issue.
type IssueType string

const (
	IssueCircular    IssueType = "circular"
	IssueMissing     IssueType = "missing"
	IssueUnmetContext IssueType = "unmet-context"
)

// DependencyIssue represents a problem found in skill dependencies.
type DependencyIssue struct {
	Type    IssueType
	Skill   string
	Message string
	Details []string
}

// toSkillMap builds a lookup map from skill name to SkillBehavior.
func toSkillMap(skills []model.SkillBehavior) map[string]model.SkillBehavior {
	m := make(map[string]model.SkillBehavior)
	for _, s := range skills {
		m[s.Skill] = s
	}
	return m
}

// CheckMissingDependencies checks that all skills referenced in the agent exist.
func CheckMissingDependencies(agent model.AgentComposition, skills []model.SkillBehavior) []DependencyIssue {
	var issues []DependencyIssue
	skillMap := toSkillMap(skills)

	for _, name := range agent.Skills {
		if _, ok := skillMap[name]; !ok {
			issues = append(issues, DependencyIssue{
				Type:    IssueMissing,
				Skill:   name,
				Message: fmt.Sprintf("Agent %q references skill %q which does not exist", agent.Agent, name),
			})
		}
	}

	return issues
}

// CheckCircularDependencies detects cycles in the consumes/produces graph within an agent.
func CheckCircularDependencies(agent model.AgentComposition, skills []model.SkillBehavior) []DependencyIssue {
	var issues []DependencyIssue
	skillMap := toSkillMap(skills)

	// Build producer map: data item -> skill name
	producerOf := make(map[string]string)
	for _, name := range agent.Skills {
		if s, ok := skillMap[name]; ok {
			for _, p := range s.Context.Produces {
				producerOf[p] = name
			}
		}
	}

	// Build adjacency: skill A -> skill B if A consumes something B produces
	adj := make(map[string][]string)
	for _, name := range agent.Skills {
		if s, ok := skillMap[name]; ok {
			for _, c := range s.Context.Consumes {
				if provider, exists := producerOf[c]; exists && provider != name {
					adj[name] = append(adj[name], provider)
				}
			}
		}
	}

	// DFS cycle detection
	visited := map[string]bool{}
	inStack := map[string]bool{}

	var dfs func(name string, path []string)
	dfs = func(name string, path []string) {
		if inStack[name] {
			startIdx := indexOf(path, name)
			cycle := append(path[startIdx:], name)
			issues = append(issues, DependencyIssue{
				Type:    IssueCircular,
				Skill:   name,
				Message: fmt.Sprintf("Circular dependency detected: %s", strings.Join(cycle, " -> ")),
				Details: cycle,
			})
			return
		}
		if visited[name] {
			return
		}

		visited[name] = true
		inStack[name] = true

		for _, dep := range adj[name] {
			newPath := make([]string, len(path)+1)
			copy(newPath, path)
			newPath[len(path)] = name
			dfs(dep, newPath)
		}

		inStack[name] = false
	}

	for _, name := range agent.Skills {
		if !visited[name] {
			dfs(name, nil)
		}
	}

	return issues
}

// CheckUnmetContext validates that every skill's consumes are satisfied by
// another skill's produces within the agent, or by the agent's own consumes.
func CheckUnmetContext(agent model.AgentComposition, skills []model.SkillBehavior) []DependencyIssue {
	var issues []DependencyIssue
	skillMap := toSkillMap(skills)

	// Collect all produces from skills in this agent
	allProduced := make(map[string]bool)
	for _, name := range agent.Skills {
		if s, ok := skillMap[name]; ok {
			for _, p := range s.Context.Produces {
				allProduced[p] = true
			}
		}
	}

	// Agent-level consumes are also available as inputs
	for _, c := range agent.Consumes {
		allProduced[c] = true
	}

	// Check each skill's consumes
	for _, name := range agent.Skills {
		s, ok := skillMap[name]
		if !ok {
			continue
		}
		for _, c := range s.Context.Consumes {
			if !allProduced[c] {
				issues = append(issues, DependencyIssue{
					Type:    IssueUnmetContext,
					Skill:   name,
					Message: fmt.Sprintf("Skill %q consumes %q but no skill in agent %q produces it and it is not in agent consumes", name, c, agent.Agent),
				})
			}
		}
	}

	return issues
}

// CheckAgentProduces validates that the agent's declared produces match
// what its skills actually produce.
func CheckAgentProduces(agent model.AgentComposition, skills []model.SkillBehavior) []DependencyIssue {
	var issues []DependencyIssue
	skillMap := toSkillMap(skills)

	allProduced := make(map[string]bool)
	for _, name := range agent.Skills {
		if s, ok := skillMap[name]; ok {
			for _, p := range s.Context.Produces {
				allProduced[p] = true
			}
		}
	}

	for _, p := range agent.Produces {
		if !allProduced[p] {
			issues = append(issues, DependencyIssue{
				Type:    IssueUnmetContext,
				Skill:   agent.Agent,
				Message: fmt.Sprintf("Agent %q declares produces %q but no skill produces it", agent.Agent, p),
			})
		}
	}

	return issues
}

// CheckDependencies analyzes an agent and its skills for dependency issues.
func CheckDependencies(agent model.AgentComposition, skills []model.SkillBehavior) []DependencyIssue {
	var issues []DependencyIssue
	issues = append(issues, CheckMissingDependencies(agent, skills)...)
	issues = append(issues, CheckCircularDependencies(agent, skills)...)
	issues = append(issues, CheckUnmetContext(agent, skills)...)
	issues = append(issues, CheckAgentProduces(agent, skills)...)
	return issues
}

// indexOf returns the index of item in slice, or -1 if not found.
func indexOf(slice []string, item string) int {
	for i, s := range slice {
		if s == item {
			return i
		}
	}
	return -1
}

// containsString checks if a string slice contains a given string.
func containsString(slice []string, item string) bool {
	for _, s := range slice {
		if s == item {
			return true
		}
	}
	return false
}
