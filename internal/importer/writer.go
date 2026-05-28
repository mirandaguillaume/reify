package importer

import (
	"fmt"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

// WriteImportResult writes all skills and the optional agent from the given
// ImportResult to disk under outputDir. Skills are written to
// {outputDir}/skills/{skill-name}.skill.yaml and the agent (if present) to
// {outputDir}/agents/{agent-name}.agent.yaml.
//
// If RawYAML is non-empty for a result it is written verbatim; otherwise the
// model struct is marshaled with gopkg.in/yaml.v3.
//
// An error is returned if any target file already exists (conflict detection).
// Written file paths are returned on success.
func WriteImportResult(result ImportResult, outputDir string) ([]string, error) {
	var written []string

	for _, sr := range result.Skills {
		name := sr.Skill.Skill
		if name == "" {
			name = "unknown"
		}
		dir := filepath.Join(outputDir, "skills")
		if err := os.MkdirAll(dir, 0755); err != nil {
			return written, fmt.Errorf("creating skills directory: %w", err)
		}

		path := filepath.Join(dir, name+".skill.yaml")
		if _, err := os.Stat(path); err == nil {
			return written, fmt.Errorf("file already exists: %s", path)
		}

		data, err := resolveSkillData(sr)
		if err != nil {
			return written, fmt.Errorf("marshaling skill %q: %w", name, err)
		}

		if err := os.WriteFile(path, data, 0644); err != nil {
			return written, fmt.Errorf("writing skill %q: %w", name, err)
		}
		written = append(written, path)
	}

	// Write contracts.
	for name, content := range result.Contracts {
		dir := filepath.Join(outputDir, "contracts")
		if err := os.MkdirAll(dir, 0755); err != nil {
			return written, fmt.Errorf("creating contracts directory: %w", err)
		}

		path := filepath.Join(dir, name+".md")
		if _, err := os.Stat(path); err == nil {
			return written, fmt.Errorf("contract file already exists: %s", path)
		}

		data := []byte(content)
		if len(data) == 0 || data[len(data)-1] != '\n' {
			data = append(data, '\n')
		}

		if err := os.WriteFile(path, data, 0644); err != nil {
			return written, fmt.Errorf("writing contract %q: %w", name, err)
		}
		written = append(written, path)
	}

	if result.Agent != nil {
		name := result.Agent.Agent.Agent
		if name == "" {
			name = "unknown"
		}
		dir := filepath.Join(outputDir, "agents")
		if err := os.MkdirAll(dir, 0755); err != nil {
			return written, fmt.Errorf("creating agents directory: %w", err)
		}

		path := filepath.Join(dir, name+".agent.yaml")
		if _, err := os.Stat(path); err == nil {
			return written, fmt.Errorf("file already exists: %s", path)
		}

		data, err := resolveAgentData(*result.Agent)
		if err != nil {
			return written, fmt.Errorf("marshaling agent %q: %w", name, err)
		}

		if err := os.WriteFile(path, data, 0644); err != nil {
			return written, fmt.Errorf("writing agent %q: %w", name, err)
		}
		written = append(written, path)
	}

	return written, nil
}

// resolveSkillData returns the YAML bytes for a SkillResult. If RawYAML is
// non-empty it is returned as-is (with a trailing newline); otherwise the
// Skill struct is marshaled.
func resolveSkillData(sr SkillResult) ([]byte, error) {
	if sr.RawYAML != "" {
		data := []byte(sr.RawYAML)
		if len(data) == 0 || data[len(data)-1] != '\n' {
			data = append(data, '\n')
		}
		return data, nil
	}
	return yaml.Marshal(sr.Skill)
}

// resolveAgentData returns the YAML bytes for an AgentResult. If RawYAML is
// non-empty it is returned as-is (with a trailing newline); otherwise the
// Agent struct is marshaled.
func resolveAgentData(ar AgentResult) ([]byte, error) {
	if ar.RawYAML != "" {
		data := []byte(ar.RawYAML)
		if len(data) == 0 || data[len(data)-1] != '\n' {
			data = append(data, '\n')
		}
		return data, nil
	}
	return yaml.Marshal(ar.Agent)
}
