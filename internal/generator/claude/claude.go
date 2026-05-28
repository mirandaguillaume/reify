package claude

import (
	"github.com/mirandaguillaume/reify/pkg/model"
	"github.com/mirandaguillaume/reify/pkg/spec"
)

type claudeGenerator struct {
	compact      bool
	contracts    map[string]string
	contractsDir string
}

func (g *claudeGenerator) Target() string           { return "claude" }
func (g *claudeGenerator) DefaultOutputDir() string { return ".claude" }

func (g *claudeGenerator) GenerateSkill(skill model.SkillBehavior) string {
	return GenerateSkillMd(skill, g.contracts, g.contractsDir)
}

func (g *claudeGenerator) SetOptions(opts spec.GeneratorOptions) {
	g.compact = opts.Compact
	g.contracts = opts.Contracts
	g.contractsDir = opts.ContractsDir
}

func (g *claudeGenerator) GenerateAgent(agent model.AgentComposition, skills []model.SkillBehavior, outputDir string) string {
	if g.compact {
		return GenerateCompactAgentMd(agent, skills)
	}
	return GenerateAgentMd(agent, skills, outputDir)
}

func (g *claudeGenerator) GenerateInstructions(skills []model.SkillBehavior, agents []model.AgentComposition) string {
	return GenerateClaudeMd(skills, agents)
}

func (g *claudeGenerator) InstructionsPath() string     { return "CLAUDE.md" }
func (g *claudeGenerator) SkillPath(name string) string { return "skills/" + name + "/SKILL.md" }
func (g *claudeGenerator) AgentPath(name string) string { return "agents/" + name + ".md" }
func (g *claudeGenerator) ContextDir() string           { return "context" }

func init() {
	spec.Register("claude", func() spec.Generator { return &claudeGenerator{} })
}
