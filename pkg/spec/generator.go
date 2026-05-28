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
