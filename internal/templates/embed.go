package templates

import "embed"

//go:embed *.yaml
var FS embed.FS

func SkillTemplate() ([]byte, error) {
	return FS.ReadFile("skill.yaml")
}
