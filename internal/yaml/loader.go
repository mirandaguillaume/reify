package yamlloader

import (
	"fmt"
	"strings"

	"github.com/mirandaguillaume/reify/pkg/model"
	"gopkg.in/yaml.v3"
)

// ParseSkillYAML unmarshals a YAML string into a SkillBehavior and validates it.
func ParseSkillYAML(content string) (model.SkillBehavior, error) {
	var skill model.SkillBehavior
	if err := yaml.Unmarshal([]byte(content), &skill); err != nil {
		return skill, fmt.Errorf("YAML syntax error: %w", err)
	}
	errs := model.ValidateSkill(skill)
	if len(errs) > 0 {
		return skill, fmt.Errorf("validation failed: %s", strings.Join(errs, "; "))
	}
	return skill, nil
}

// ParseAgentYAML unmarshals a YAML string into an AgentComposition and validates it.
func ParseAgentYAML(content string) (model.AgentComposition, error) {
	var agent model.AgentComposition
	if err := yaml.Unmarshal([]byte(content), &agent); err != nil {
		return agent, fmt.Errorf("YAML syntax error: %w", err)
	}
	errs := model.ValidateAgent(agent)
	if len(errs) > 0 {
		return agent, fmt.Errorf("validation failed: %s", strings.Join(errs, "; "))
	}
	return agent, nil
}
