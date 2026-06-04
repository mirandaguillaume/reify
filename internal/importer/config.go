package importer

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"gopkg.in/yaml.v3"

	"github.com/mirandaguillaume/reify/pkg/model"
)

// ImportProjectConfig reads harness config from a .claude/ directory and an
// optional .mcp.json file, returning a unified model.ProjectConfig.
//
//   - claudeDir: path to a .claude/ directory (may contain settings.json and
//     skills/<name>/SKILL.md files).
//   - mcpFile: path to a .mcp.json file (empty string = skip).
//
// Missing files are not errors — they simply contribute nothing to the result.
// Malformed files that exist (invalid JSON, unparseable YAML) are errors.
func ImportProjectConfig(claudeDir, mcpFile string) (model.ProjectConfig, error) {
	var cfg model.ProjectConfig

	hooks, err := parseHooks(filepath.Join(claudeDir, "settings.json"))
	if err != nil {
		return model.ProjectConfig{}, err
	}
	cfg.Hooks = hooks

	if mcpFile != "" {
		servers, err := parseMCPServers(mcpFile)
		if err != nil {
			return model.ProjectConfig{}, err
		}
		cfg.MCPServers = servers
	}

	skills, err := parseNativeSkills(filepath.Join(claudeDir, "skills"))
	if err != nil {
		return model.ProjectConfig{}, err
	}
	cfg.NativeSkills = skills

	return cfg, nil
}

// ---- hooks ----------------------------------------------------------------

// settingsJSON mirrors the part of Claude Code's settings.json we care about.
type settingsJSON struct {
	Hooks map[string][]hookGroup `json:"hooks"`
}

// hookGroup is one entry in the per-event array: a matcher + inner hooks.
type hookGroup struct {
	Matcher string      `json:"matcher"`
	Hooks   []hookEntry `json:"hooks"`
}

// hookEntry is one element of a hookGroup's inner hook list.
type hookEntry struct {
	Type    string `json:"type"`
	Command string `json:"command"`
}

// parseHooks reads <claudeDir>/settings.json and flattens the nested hook
// structure into []model.Hook. Events are sorted alphabetically; within each
// event the array order and inner hook order are preserved.
// Only entries with type=="command" are included.
// Returns nil (not an error) when the file does not exist.
func parseHooks(settingsPath string) ([]model.Hook, error) {
	data, err := os.ReadFile(settingsPath)
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}

	var s settingsJSON
	if err := json.Unmarshal(data, &s); err != nil {
		return nil, err
	}

	if len(s.Hooks) == 0 {
		return nil, nil
	}

	// Sort events alphabetically for determinism.
	events := make([]string, 0, len(s.Hooks))
	for event := range s.Hooks {
		events = append(events, event)
	}
	sort.Strings(events)

	var hooks []model.Hook
	for _, event := range events {
		for _, group := range s.Hooks[event] {
			for _, entry := range group.Hooks {
				if entry.Type != "command" {
					continue
				}
				hooks = append(hooks, model.Hook{
					Event:   event,
					Matcher: group.Matcher,
					Command: entry.Command,
				})
			}
		}
	}
	return hooks, nil
}

// ---- MCP servers ----------------------------------------------------------

// mcpJSON mirrors the top-level shape of a .mcp.json file.
type mcpJSON struct {
	MCPServers map[string]model.MCPServer `json:"mcpServers"`
}

// parseMCPServers reads a .mcp.json file and returns the mcpServers map.
// Returns nil (not an error) when the file does not exist.
func parseMCPServers(mcpFile string) (map[string]model.MCPServer, error) {
	data, err := os.ReadFile(mcpFile)
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}

	var m mcpJSON
	if err := json.Unmarshal(data, &m); err != nil {
		return nil, err
	}
	return m.MCPServers, nil
}

// ---- native skills --------------------------------------------------------

// skillFrontmatter holds the YAML frontmatter fields of a SKILL.md file.
type skillFrontmatter struct {
	Name                   string `yaml:"name"`
	Description            string `yaml:"description"`
	DisableModelInvocation bool   `yaml:"disable-model-invocation"`
}

// parseNativeSkills walks <claudeDir>/skills/<name>/SKILL.md files and
// returns them as []model.NativeSkill sorted by Name. Returns nil (not an
// error) when the skills directory does not exist.
func parseNativeSkills(skillsDir string) ([]model.NativeSkill, error) {
	entries, err := os.ReadDir(skillsDir)
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}

	var skills []model.NativeSkill
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		skillPath := filepath.Join(skillsDir, entry.Name(), "SKILL.md")
		skill, err := parseSkillFile(skillPath, entry.Name())
		if err != nil {
			return nil, err
		}
		skills = append(skills, skill)
	}

	sort.Slice(skills, func(i, j int) bool {
		return skills[i].Name < skills[j].Name
	})
	return skills, nil
}

// parseSkillFile parses one SKILL.md file. dirName is used as a fallback when
// the frontmatter does not provide a name.
func parseSkillFile(path, dirName string) (model.NativeSkill, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return model.NativeSkill{}, err
	}

	fm, body, err := extractSkillFrontmatter(string(data))
	if err != nil {
		return model.NativeSkill{}, err
	}

	name := fm.Name
	if name == "" {
		name = dirName
	}

	return model.NativeSkill{
		Name:                   name,
		Description:            fm.Description,
		DisableModelInvocation: fm.DisableModelInvocation,
		Body:                   strings.TrimSpace(body),
	}, nil
}

// extractSkillFrontmatter splits a SKILL.md into its YAML frontmatter and
// the remaining body. The frontmatter is delimited by the first two "---"
// lines. If no frontmatter block is found an empty struct and the full content
// are returned.
func extractSkillFrontmatter(content string) (skillFrontmatter, string, error) {
	lines := strings.SplitAfter(content, "\n")

	start := -1
	for i, line := range lines {
		if strings.TrimSpace(line) == "---" {
			start = i
			break
		}
	}
	if start == -1 {
		return skillFrontmatter{}, content, nil
	}

	end := -1
	for i := start + 1; i < len(lines); i++ {
		if strings.TrimSpace(lines[i]) == "---" {
			end = i
			break
		}
	}
	if end == -1 {
		return skillFrontmatter{}, content, nil
	}

	yamlBlock := strings.Join(lines[start+1:end], "")
	body := strings.Join(lines[end+1:], "")

	var fm skillFrontmatter
	if err := yaml.Unmarshal([]byte(yamlBlock), &fm); err != nil {
		return skillFrontmatter{}, content, err
	}
	return fm, body, nil
}
