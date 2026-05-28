package model

import (
	"fmt"
	"strings"

	"github.com/mirandaguillaume/reify/pkg/sandbox"
	"gopkg.in/yaml.v3"
)

// MemoryType represents the type of memory a skill uses.
type MemoryType string

const (
	MemoryShortTerm    MemoryType = "short-term"
	MemoryConversation MemoryType = "conversation"
	MemoryLongTerm     MemoryType = "long-term"
)

// TraceLevel represents the verbosity of observability tracing.
type TraceLevel string

const (
	TraceLevelMinimal  TraceLevel = "minimal"
	TraceLevelStandard TraceLevel = "standard"
	TraceLevelDetailed TraceLevel = "detailed"
)

// AccessLevel represents filesystem access permissions.
type AccessLevel string

const (
	AccessNone      AccessLevel = "none"
	AccessReadOnly  AccessLevel = "read-only"
	AccessReadWrite AccessLevel = "read-write"
	AccessFull      AccessLevel = "full"
)

// NetworkAccess represents network access permissions.
type NetworkAccess string

const (
	NetworkNone      NetworkAccess = "none"
	NetworkAllowlist NetworkAccess = "allowlist"
	NetworkFull      NetworkAccess = "full"
)

// SandboxType represents the sandboxing level.
type SandboxType string

const (
	SandboxNone      SandboxType = "none"
	SandboxContainer SandboxType = "container"
	SandboxVM        SandboxType = "vm"
)

// NegotiationStrategy represents how file conflicts are resolved.
type NegotiationStrategy string

const (
	NegotiationYield    NegotiationStrategy = "yield"
	NegotiationOverride NegotiationStrategy = "override"
	NegotiationMerge    NegotiationStrategy = "merge"
)

// EffortLevel indicates the computational effort a skill requires.
type EffortLevel string

const (
	EffortLight  EffortLevel = "light"
	EffortMedium EffortLevel = "medium"
	EffortHeavy  EffortLevel = "heavy"
)

// ContextFacet defines what data a skill consumes and produces.
type ContextFacet struct {
	Consumes []string   `yaml:"consumes"`
	Produces []string   `yaml:"produces"`
	Memory   MemoryType `yaml:"memory"`
}

// StrategyFacet defines the tools and approach a skill uses.
type StrategyFacet struct {
	Tools    []string    `yaml:"tools"`
	Approach string      `yaml:"approach"`
	Steps    []string    `yaml:"steps,omitempty"`
	Effort   EffortLevel `yaml:"effort,omitempty"`
}

// GuardrailRule can be either a plain string or a map[string]interface{}.
type GuardrailRule struct {
	stringVal *string
	mapVal    map[string]interface{}
}

// UnmarshalYAML implements custom YAML unmarshaling for GuardrailRule.
func (g *GuardrailRule) UnmarshalYAML(value *yaml.Node) error {
	switch value.Kind {
	case yaml.ScalarNode:
		g.stringVal = &value.Value
		return nil
	case yaml.MappingNode:
		m := make(map[string]interface{})
		if err := value.Decode(&m); err != nil {
			return err
		}
		g.mapVal = m
		return nil
	default:
		return fmt.Errorf("guardrail rule must be a string or a mapping, got %v", value.Kind)
	}
}

// StringValue returns the string value if this rule is a string.
func (g GuardrailRule) StringValue() (string, bool) {
	if g.stringVal != nil {
		return *g.stringVal, true
	}
	return "", false
}

// MapValue returns the map value if this rule is a mapping.
func (g GuardrailRule) MapValue() (map[string]interface{}, bool) {
	if g.mapVal != nil {
		return g.mapVal, true
	}
	return nil, false
}

// HasKey returns true if this rule is a mapping and contains the given key.
func (g GuardrailRule) HasKey(key string) bool {
	if g.mapVal == nil {
		return false
	}
	_, ok := g.mapVal[key]
	return ok
}

// ContainsString returns true if this rule is a string and contains the substring.
func (g GuardrailRule) ContainsString(substr string) bool {
	if g.stringVal == nil {
		return false
	}
	return strings.Contains(*g.stringVal, substr)
}

// ObservabilityFacet defines tracing and metrics configuration.
type ObservabilityFacet struct {
	TraceLevel TraceLevel `yaml:"trace_level"`
	Metrics    []string   `yaml:"metrics"`
}

// FileAccessConfig declares file access permissions for a skill.
type FileAccessConfig struct {
	Read  []string `yaml:"read,omitempty"`
	Write []string `yaml:"write,omitempty"`
	Deny  []string `yaml:"deny,omitempty"`
}

// ToPolicy converts the config to a sandbox FileAccessPolicy.
// Returns nil if the receiver is nil (no policy = allow all).
func (f *FileAccessConfig) ToPolicy() *sandbox.FileAccessPolicy {
	if f == nil {
		return nil
	}
	return &sandbox.FileAccessPolicy{
		ReadGlobs:  f.Read,
		WriteGlobs: f.Write,
		DenyGlobs:  f.Deny,
	}
}

// SecurityFacet defines access and sandboxing constraints.
type SecurityFacet struct {
	Filesystem AccessLevel       `yaml:"filesystem"`
	Network    NetworkAccess     `yaml:"network"`
	Secrets    []string          `yaml:"secrets"`
	Sandbox    SandboxType       `yaml:"sandbox,omitempty"`
	FileAccess *FileAccessConfig `yaml:"file_access,omitempty"`
}

// NegotiationFacet defines conflict resolution behavior.
type NegotiationFacet struct {
	FileConflicts NegotiationStrategy `yaml:"file_conflicts"`
	Priority      int                 `yaml:"priority"`
}

// GuardrailsFacet is a list of guardrail rules.
type GuardrailsFacet = []GuardrailRule

// WhenToUseFacet describes when a skill should and should not be used.
type WhenToUseFacet struct {
	Triggers   []string `yaml:"triggers,omitempty"`
	DontUse    []string `yaml:"dont_use,omitempty"`
	Especially []string `yaml:"especially,omitempty"`
}

// IsEmpty returns true if no when-to-use guidance is provided.
func (w WhenToUseFacet) IsEmpty() bool {
	return len(w.Triggers) == 0 && len(w.DontUse) == 0 && len(w.Especially) == 0
}

// AntiPattern documents a common mistake and the correct approach.
type AntiPattern struct {
	Excuse  string `yaml:"excuse"`
	Reality string `yaml:"reality"`
}

// CodeExample provides a concrete code snippet demonstrating usage.
type CodeExample struct {
	Label string `yaml:"label"`
	Code  string `yaml:"code"`
	Lang  string `yaml:"lang,omitempty"`
}

// SkillBehavior is the complete skill specification with all facets.
type SkillBehavior struct {
	Skill         string             `yaml:"skill"`
	Version       string             `yaml:"version"`
	Context       ContextFacet       `yaml:"context"`
	Strategy      StrategyFacet      `yaml:"strategy"`
	Guardrails    GuardrailsFacet    `yaml:"guardrails"`
	Observability ObservabilityFacet `yaml:"observability"`
	Security      SecurityFacet      `yaml:"security"`
	Negotiation   NegotiationFacet   `yaml:"negotiation"`
	WhenToUse     WhenToUseFacet     `yaml:"when_to_use,omitempty"`
	AntiPatterns  []AntiPattern      `yaml:"anti_patterns,omitempty"`
	Examples      []CodeExample      `yaml:"examples,omitempty"`
}
