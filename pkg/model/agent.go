package model

// OrchestrationStrategy defines how skills are orchestrated within an agent.
type OrchestrationStrategy string

const (
	OrchestrationSequential       OrchestrationStrategy = "sequential"
	OrchestrationParallel         OrchestrationStrategy = "parallel"
	OrchestrationParallelThenMerge OrchestrationStrategy = "parallel-then-merge"
	OrchestrationAdaptive         OrchestrationStrategy = "adaptive"
)

// Stage represents a named group of skills with an orchestration strategy.
type Stage struct {
	Name     string                `yaml:"name"`
	Strategy OrchestrationStrategy `yaml:"strategy"`
	Skills   []string              `yaml:"skills"`
}

// AgentComposition defines an agent as a composition of skills.
type AgentComposition struct {
	Agent         string                `yaml:"agent"`
	Skills        []string              `yaml:"skills,omitempty"`
	Orchestration OrchestrationStrategy `yaml:"orchestration,omitempty"`
	Description   string                `yaml:"description,omitempty"`
	Consumes      []string              `yaml:"consumes,omitempty"`
	Produces      []string              `yaml:"produces,omitempty"`
	Stages        []Stage               `yaml:"stages,omitempty"`
}

// AllSkills returns all skill names, whether from flat Skills or from Stages.
func (a AgentComposition) AllSkills() []string {
	if len(a.Stages) > 0 {
		var all []string
		for _, stage := range a.Stages {
			all = append(all, stage.Skills...)
		}
		return all
	}
	return a.Skills
}
