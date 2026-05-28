// Package registry provides the research-backed recommendation registry.
// The default registry is embedded via //go:embed; a local override at
// .reify/research-registry.yaml takes precedence when present.
package registry

import (
	"crypto/sha256"
	"embed"
	"fmt"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

//go:embed data/research-registry.yaml
var embeddedFS embed.FS

// Recommendation describes a single research-backed recommendation entry.
type Recommendation struct {
	ID               string `yaml:"id"`
	Title            string `yaml:"title"`
	Citation         string `yaml:"citation"`
	Paper            string `yaml:"paper"`
	URL              string `yaml:"url"`
	Finding          string `yaml:"finding"`
	Confidence       string `yaml:"confidence"`
	DetectionPrompt  string `yaml:"detection_prompt"`
	SuggestionPrompt string `yaml:"suggestion_prompt"`
}

// registryFile is the on-disk YAML schema.
type registryFile struct {
	Version         string           `yaml:"version"`
	LatestVersion   string           `yaml:"latest_version,omitempty"`
	UpdateURL       string           `yaml:"update_url,omitempty"`
	Recommendations []Recommendation `yaml:"recommendations"`
}

// Registry holds the loaded recommendations indexed by ID.
type Registry struct {
	Version       string
	LatestVersion string // latest published version (from embedded registry)
	UpdateURL     string // URL for downloading updated registry
	Source        string // "embedded" or "local"
	SourcePath    string // path to local override if used
	entries       []Recommendation
	index         map[string]Recommendation
}

// Load reads the registry. If projectDir is non-empty and contains
// .reify/research-registry.yaml, that file is used instead of the
// embedded default. Pass "" to always use the embedded registry.
func Load(projectDir string) (*Registry, error) {
	// Try local override first
	if projectDir != "" {
		localPath := filepath.Join(projectDir, ".reify", "research-registry.yaml")
		if data, err := os.ReadFile(localPath); err == nil {
			reg, err := parse(data)
			if err != nil {
				return nil, fmt.Errorf("parse local registry %s: %w", localPath, err)
			}
			reg.Source = "local"
			reg.SourcePath = localPath
			return reg, nil
		}
	}

	// Fall back to embedded
	data, err := embeddedFS.ReadFile("data/research-registry.yaml")
	if err != nil {
		return nil, fmt.Errorf("read embedded registry: %w", err)
	}

	reg, err := parse(data)
	if err != nil {
		return nil, fmt.Errorf("parse embedded registry: %w", err)
	}
	reg.Source = "embedded"
	return reg, nil
}

func parse(data []byte) (*Registry, error) {
	var f registryFile
	if err := yaml.Unmarshal(data, &f); err != nil {
		return nil, err
	}

	idx := make(map[string]Recommendation, len(f.Recommendations))
	for _, r := range f.Recommendations {
		idx[r.ID] = r
	}

	return &Registry{
		Version:       f.Version,
		LatestVersion: f.LatestVersion,
		UpdateURL:     f.UpdateURL,
		entries:       f.Recommendations,
		index:         idx,
	}, nil
}

// Get returns a recommendation by ID. The second return value is false if
// the ID is not found.
func (r *Registry) Get(id string) (Recommendation, bool) {
	rec, ok := r.index[id]
	return rec, ok
}

// All returns all recommendations in registry order.
func (r *Registry) All() []Recommendation {
	return r.entries
}

// NeedsUpdate returns true when LatestVersion is set and differs from Version.
func (r *Registry) NeedsUpdate() bool {
	return r.LatestVersion != "" && r.Version != r.LatestVersion
}

// promptVersion must be bumped when buildPrompt() template structure changes
// (instructions, example format, category prefix). Registry content changes
// are captured by hashing the entries themselves.
const promptVersion = "v2-cot" // bumped: added reasoning field to CoT prompt

// PromptHash returns a short hash of prompt version + all registry entry fields
// that feed into the LLM prompt. Used as part of cache keys so any prompt
// change invalidates the cache.
func (r *Registry) PromptHash() string {
	h := sha256.New()
	h.Write([]byte(promptVersion))
	for _, e := range r.entries {
		h.Write([]byte(e.ID))
		h.Write([]byte(e.DetectionPrompt))
		h.Write([]byte(e.Citation))
		h.Write([]byte(e.Finding))
	}
	return fmt.Sprintf("%x", h.Sum(nil))[:8]
}
