package copilot

import (
	"github.com/mirandaguillaume/reify/pkg/model"
	"github.com/mirandaguillaume/reify/pkg/spec"
)

type copilotGenerator struct {
	compact   bool
	contracts map[string]string
}

func (g *copilotGenerator) Target() string           { return "copilot" }
func (g *copilotGenerator) DefaultOutputDir() string { return ".github" }

func (g *copilotGenerator) GenerateSkill(skill model.SkillBehavior) string {
	return GenerateCopilotSkillMd(skill, g.contracts)
}

func (g *copilotGenerator) SetOptions(opts spec.GeneratorOptions) {
	g.compact = opts.Compact
	g.contracts = opts.Contracts
}

func (g *copilotGenerator) GenerateAgent(agent model.AgentComposition, skills []model.SkillBehavior, outputDir string) string {
	if g.compact {
		return GenerateCompactCopilotAgentMd(agent, skills)
	}
	return GenerateCopilotAgentMd(agent, skills, outputDir)
}

func (g *copilotGenerator) GenerateInstructions(skills []model.SkillBehavior, agents []model.AgentComposition) string {
	return GenerateCopilotInstructions(skills, agents)
}

func (g *copilotGenerator) SkillPath(name string) string { return "skills/" + name + "/SKILL.md" }
func (g *copilotGenerator) AgentPath(name string) string { return "agents/" + name + ".agent.md" }
func (g *copilotGenerator) ContextDir() string           { return "context" }
func (g *copilotGenerator) InstructionsPath() string     { return "copilot-instructions.md" }

func init() {
	spec.Register("copilot", func() spec.Generator { return &copilotGenerator{} })
}
