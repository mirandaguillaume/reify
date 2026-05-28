package analyzer

import (
	"math"
	"strings"

	"github.com/mirandaguillaume/reify/pkg/model"
)

// SkillBreakdown contains per-facet scores for a skill.
type SkillBreakdown struct {
	Context       int
	Strategy      int
	Guardrails    int
	Observability int
	Security      int
	WhenToUse     int
	AntiPatterns  int
	Examples      int
}

// SkillScore is the overall score and per-facet breakdown for a skill.
type SkillScore struct {
	Skill     string
	Total     int
	Breakdown SkillBreakdown
}

// AgentBreakdown contains per-dimension scores for an agent.
type AgentBreakdown struct {
	Description   int
	Composition   int
	DataFlow      int
	Orchestration int
}

// AgentScore is the overall score and per-dimension breakdown for an agent.
type AgentScore struct {
	Agent     string
	Total     int
	Breakdown AgentBreakdown
}

// Skill scoring weights (base facets = 90, bonus facets = 10)
const (
	weightContext       = 18
	weightStrategy      = 22
	weightGuardrails    = 18
	weightObservability = 14
	weightSecurity      = 18
	weightWhenToUse     = 3
	weightAntiPatterns  = 2
	weightExamples      = 5
)

// Agent scoring weights
const (
	weightDescription   = 20
	weightComposition   = 25
	weightDataFlow      = 35
	weightOrchestration = 20
)

func clampRound(score float64, max int) int {
	return int(math.Min(float64(max), math.Round(score)))
}

func scoreContext(skill model.SkillBehavior) int {
	score := 0.0
	max := float64(weightContext)

	// Has consumes defined
	if len(skill.Context.Consumes) > 0 {
		score += max * 0.35
	}
	// Has produces defined
	if len(skill.Context.Produces) > 0 {
		score += max * 0.35
	}
	// Memory type scoring
	switch skill.Context.Memory {
	case model.MemoryConversation:
		score += max * 0.15
	case model.MemoryLongTerm:
		score += max * 0.1
	case model.MemoryShortTerm:
		score += max * 0.15
	}
	// Bonus: both consumes and produces = well-defined I/O contract
	if len(skill.Context.Consumes) > 0 && len(skill.Context.Produces) > 0 {
		score += max * 0.15
	}

	return clampRound(score, weightContext)
}

func scoreStrategy(skill model.SkillBehavior) int {
	score := 0.0
	max := float64(weightStrategy)

	// Has tools
	if len(skill.Strategy.Tools) > 0 {
		score += max * 0.35
	}
	// Has approach defined
	if len(skill.Strategy.Approach) > 0 {
		score += max * 0.25
	}
	// Has steps defined
	if len(skill.Strategy.Steps) > 0 {
		score += max * 0.25
		// Bonus for detailed steps (3+)
		if len(skill.Strategy.Steps) >= 3 {
			score += max * 0.15
		}
	}

	return clampRound(score, weightStrategy)
}

func scoreGuardrails(skill model.SkillBehavior) int {
	if len(skill.Guardrails) == 0 {
		return 0
	}

	score := 0.0
	max := float64(weightGuardrails)

	// Has at least one guardrail
	score += max * 0.5

	// Has timeout (critical for agent safety)
	hasTimeout := false
	for _, g := range skill.Guardrails {
		if s, ok := g.StringValue(); ok && strings.Contains(s, "timeout") {
			hasTimeout = true
			break
		}
		if g.HasKey("timeout") {
			hasTimeout = true
			break
		}
	}
	if hasTimeout {
		score += max * 0.3
	}

	// Multiple guardrails = defense in depth
	if len(skill.Guardrails) >= 2 {
		score += max * 0.2
	}

	return clampRound(score, weightGuardrails)
}

func scoreObservability(skill model.SkillBehavior) int {
	score := 0.0
	max := float64(weightObservability)

	// Has metrics
	if len(skill.Observability.Metrics) > 0 {
		score += max * 0.4
		if len(skill.Observability.Metrics) >= 2 {
			score += max * 0.15
		}
	}

	// Trace level
	switch skill.Observability.TraceLevel {
	case model.TraceLevelDetailed:
		score += max * 0.45
	case model.TraceLevelStandard:
		score += max * 0.3
	case model.TraceLevelMinimal:
		score += max * 0.1
	}

	return clampRound(score, weightObservability)
}

func scoreSecurity(skill model.SkillBehavior) int {
	score := 0.0
	max := float64(weightSecurity)

	// Filesystem: more restrictive = higher score
	switch skill.Security.Filesystem {
	case model.AccessNone:
		score += max * 0.4
	case model.AccessReadOnly:
		score += max * 0.35
	case model.AccessReadWrite:
		score += max * 0.15
	case model.AccessFull:
		score += max * 0.05
	}

	// Network: more restrictive = higher score
	switch skill.Security.Network {
	case model.NetworkNone:
		score += max * 0.35
	case model.NetworkAllowlist:
		score += max * 0.2
	case model.NetworkFull:
		score += max * 0.05
	}

	// No secrets = better (principle of least privilege)
	if len(skill.Security.Secrets) == 0 {
		score += max * 0.15
	} else {
		score += max * 0.05
	}

	// Sandbox bonus
	if skill.Security.Sandbox == model.SandboxContainer || skill.Security.Sandbox == model.SandboxVM {
		score += max * 0.1
	}

	return clampRound(score, weightSecurity)
}

func scoreWhenToUse(skill model.SkillBehavior) int {
	if skill.WhenToUse.IsEmpty() {
		return 0
	}
	score := 0.0
	max := float64(weightWhenToUse)
	if len(skill.WhenToUse.Triggers) > 0 {
		score += max * 0.4
	}
	if len(skill.WhenToUse.DontUse) > 0 {
		score += max * 0.3
	}
	if len(skill.WhenToUse.Especially) > 0 {
		score += max * 0.3
	}
	return clampRound(score, weightWhenToUse)
}

func scoreAntiPatterns(skill model.SkillBehavior) int {
	if len(skill.AntiPatterns) == 0 {
		return 0
	}
	score := float64(weightAntiPatterns) * 0.5
	if len(skill.AntiPatterns) >= 2 {
		score = float64(weightAntiPatterns)
	}
	return clampRound(score, weightAntiPatterns)
}

func scoreExamples(skill model.SkillBehavior) int {
	if len(skill.Examples) == 0 {
		return 0
	}
	score := float64(weightExamples) * 0.5
	if len(skill.Examples) >= 2 {
		score = float64(weightExamples)
	}
	return clampRound(score, weightExamples)
}

// ScoreSkill calculates the AX quality score for a skill.
func ScoreSkill(skill model.SkillBehavior) SkillScore {
	breakdown := SkillBreakdown{
		Context:       scoreContext(skill),
		Strategy:      scoreStrategy(skill),
		Guardrails:    scoreGuardrails(skill),
		Observability: scoreObservability(skill),
		Security:      scoreSecurity(skill),
		WhenToUse:     scoreWhenToUse(skill),
		AntiPatterns:  scoreAntiPatterns(skill),
		Examples:      scoreExamples(skill),
	}

	total := breakdown.Context + breakdown.Strategy + breakdown.Guardrails +
		breakdown.Observability + breakdown.Security +
		breakdown.WhenToUse + breakdown.AntiPatterns + breakdown.Examples

	return SkillScore{
		Skill:     skill.Skill,
		Total:     total,
		Breakdown: breakdown,
	}
}

func scoreDescription(agent model.AgentComposition) int {
	max := weightDescription
	if agent.Description == "" {
		return 0
	}

	score := float64(max) * 0.6 // Has description at all

	// Longer description = more informative
	words := countWords(agent.Description)
	if words >= 5 {
		score += float64(max) * 0.2
	}
	if words >= 10 {
		score += float64(max) * 0.2
	}

	return clampRound(score, max)
}

func countWords(s string) int {
	return len(strings.Fields(s))
}

func scoreComposition(agent model.AgentComposition) int {
	max := weightComposition
	score := 0.0

	// At least 2 skills = meaningful composition
	if len(agent.Skills) >= 2 {
		score += float64(max) * 0.6
	} else if len(agent.Skills) == 1 {
		score += float64(max) * 0.3
	}

	// 3+ skills = rich pipeline
	if len(agent.Skills) >= 3 {
		score += float64(max) * 0.2
	}

	// No duplicates
	unique := make(map[string]bool)
	for _, s := range agent.Skills {
		unique[s] = true
	}
	if len(unique) == len(agent.Skills) {
		score += float64(max) * 0.2
	}

	return clampRound(score, max)
}

func scoreDataFlow(agent model.AgentComposition, resolvedSkills []model.SkillBehavior) int {
	max := weightDataFlow

	// Can't evaluate without resolved skills
	if len(resolvedSkills) < 2 {
		return int(math.Round(float64(max) * 0.5))
	}

	// Only relevant for sequential -- parallel doesn't have data flow concerns
	if agent.Orchestration != model.OrchestrationSequential {
		return max
	}

	// Resolve skills in agent order
	skillByName := make(map[string]model.SkillBehavior)
	for _, s := range resolvedSkills {
		skillByName[s.Skill] = s
	}

	var orderedSkills []model.SkillBehavior
	for _, name := range agent.Skills {
		if s, ok := skillByName[name]; ok {
			orderedSkills = append(orderedSkills, s)
		}
	}

	if len(orderedSkills) < 2 {
		return int(math.Round(float64(max) * 0.5))
	}

	// Collect all items produced by ANY skill in the pipeline
	allProduced := make(map[string]bool)
	for _, skill := range orderedSkills {
		for _, item := range skill.Context.Produces {
			allProduced[item] = true
		}
	}

	// Check: does each inter-skill consumer come after its producer?
	producedBefore := make(map[string]bool)
	interSkillConsumes := 0
	satisfiedConsumes := 0

	for _, skill := range orderedSkills {
		for _, item := range skill.Context.Consumes {
			if !allProduced[item] {
				continue // environment input, skip
			}
			interSkillConsumes++
			if producedBefore[item] {
				satisfiedConsumes++
			}
		}
		for _, item := range skill.Context.Produces {
			producedBefore[item] = true
		}
	}

	score := 0.0
	if interSkillConsumes == 0 {
		// No inter-skill data flow -- skills are independent, give good score
		score = float64(max) * 0.8
	} else {
		// Ratio of satisfied inter-skill data flow
		ratio := float64(satisfiedConsumes) / float64(interSkillConsumes)
		score = float64(max) * ratio
	}

	return clampRound(score, max)
}

func scoreOrchestration(agent model.AgentComposition, resolvedSkills []model.SkillBehavior) int {
	max := weightOrchestration
	score := float64(max) * 0.5 // Base score for having an orchestration strategy

	// Sequential or parallel with resolved skills = well-designed pipeline
	if (agent.Orchestration == model.OrchestrationSequential ||
		agent.Orchestration == model.OrchestrationParallel ||
		agent.Orchestration == model.OrchestrationParallelThenMerge) && len(resolvedSkills) >= 2 {
		score += float64(max) * 0.3
	}

	// Has description matching orchestration style
	if agent.Description != "" {
		score += float64(max) * 0.2
	}

	return clampRound(score, max)
}

// ScoreAgent calculates the AX quality score for an agent composition.
func ScoreAgent(agent model.AgentComposition, resolvedSkills []model.SkillBehavior) AgentScore {
	breakdown := AgentBreakdown{
		Description:   scoreDescription(agent),
		Composition:   scoreComposition(agent),
		DataFlow:      scoreDataFlow(agent, resolvedSkills),
		Orchestration: scoreOrchestration(agent, resolvedSkills),
	}

	total := breakdown.Description + breakdown.Composition +
		breakdown.DataFlow + breakdown.Orchestration

	return AgentScore{
		Agent:     agent.Agent,
		Total:     total,
		Breakdown: breakdown,
	}
}
