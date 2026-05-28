package doctor

import (
	"embed"
	"io/fs"
	"strings"
)

//go:embed specs/*.skill.yaml
var skillFS embed.FS

//go:embed specs/doctor.agent.yaml
var agentSpec []byte

// SkillSpecs returns a map of skill name → YAML content for all embedded doctor skills.
func SkillSpecs() (map[string][]byte, error) {
	specs := make(map[string][]byte)
	entries, err := fs.ReadDir(skillFS, "specs")
	if err != nil {
		return nil, err
	}
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".skill.yaml") {
			continue
		}
		data, err := skillFS.ReadFile("specs/" + entry.Name())
		if err != nil {
			return nil, err
		}
		name := strings.TrimSuffix(entry.Name(), ".skill.yaml")
		specs[name] = data
	}
	return specs, nil
}

// AgentSpec returns a copy of the raw YAML content of the embedded doctor agent definition.
func AgentSpec() []byte {
	out := make([]byte, len(agentSpec))
	copy(out, agentSpec)
	return out
}
