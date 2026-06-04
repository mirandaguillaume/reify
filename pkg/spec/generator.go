package spec

import (
	"fmt"
	"sort"
	"sync"

	"github.com/mirandaguillaume/reify/pkg/model"
)

// Generator is the core interface every build target must implement.
type Generator interface {
	Target() string
	DefaultOutputDir() string
	ContextDir() string
}

// SkillGenerator generates skill files.
type SkillGenerator interface {
	GenerateSkill(skill model.SkillBehavior) string
	SkillPath(name string) string
}

// AgentGenerator generates agent files.
type AgentGenerator interface {
	GenerateAgent(agent model.AgentComposition, skills []model.SkillBehavior, outputDir string) string
	AgentPath(name string) string
}

// InstructionsGenerator generates framework-level instructions. Optional.
type InstructionsGenerator interface {
	GenerateInstructions(skills []model.SkillBehavior, agents []model.AgentComposition) string
	InstructionsPath() string
}

// ConfigFile is one harness-config artefact a target wants written,
// relative to the build output dir (or escaping it with ../ for root
// files, like instructions do).
type ConfigFile struct {
	Path    string
	Content string
}

// ConfigGenerator compiles project-level harness config (hooks, MCP
// servers, native skills) for a target. Optional. A target that supports
// a feature natively emits it verbatim; one that does not degrades it
// (e.g. a hook → a prose instruction) and returns a warning. The builder
// writes the files and surfaces the warnings in BuildResult.
type ConfigGenerator interface {
	GenerateConfig(cfg model.ProjectConfig) (files []ConfigFile, warnings []string)
}

// LoadingGenerator emits auxiliary files a target needs in order to LOAD its
// instructions — e.g. aider's `.aider.conf.yml` with `read: CONVENTIONS.md`,
// without which the instructions file is ignored. Optional. Unlike
// ConfigGenerator, these are emitted on EVERY build (they are about loading
// the prose, not porting --input runtime config). The builder writes them
// with backup and surfaces any warnings.
type LoadingGenerator interface {
	LoadingFiles() (files []ConfigFile, warnings []string)
}

// FullGenerator is the composition of Generator + SkillGenerator + AgentGenerator.
type FullGenerator interface {
	Generator
	SkillGenerator
	AgentGenerator
}

// GeneratorOptions holds build-time options for generators.
type GeneratorOptions struct {
	Compact      bool
	Contracts    map[string]string // name → format template content (from contracts/ dir)
	ContractsDir string            // absolute path to contracts/ dir (for file references)
}

// Configurable is an optional interface for generators that accept build-time options.
type Configurable interface {
	SetOptions(opts GeneratorOptions)
}

// GeneratorFactory creates a new Generator instance.
type GeneratorFactory func() Generator

var (
	mu       sync.RWMutex
	registry = map[string]GeneratorFactory{}
)

// Register adds a generator factory for a build target.
func Register(name string, factory GeneratorFactory) {
	mu.Lock()
	defer mu.Unlock()
	registry[name] = factory
}

// Get returns a new Generator for the given target name.
func Get(name string) (Generator, error) {
	mu.RLock()
	defer mu.RUnlock()
	factory, ok := registry[name]
	if !ok {
		return nil, fmt.Errorf("unknown build target: %q. Available targets: %v", name, availableLocked())
	}
	return factory(), nil
}

// Available returns sorted list of registered target names.
func Available() []string {
	mu.RLock()
	defer mu.RUnlock()
	return availableLocked()
}

func availableLocked() []string {
	keys := make([]string, 0, len(registry))
	for k := range registry {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

// Reset clears the registry. Used only in tests.
func Reset() {
	mu.Lock()
	defer mu.Unlock()
	registry = map[string]GeneratorFactory{}
}
