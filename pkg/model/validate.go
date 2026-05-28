package model

import "fmt"

// ValidateSkill checks a SkillBehavior for required fields and valid enum values.
// Returns a slice of error messages (empty if valid).
func ValidateSkill(s SkillBehavior) []string {
	var errs []string

	if s.Skill == "" {
		errs = append(errs, "skill name is required")
	}
	if s.Version == "" {
		errs = append(errs, "version is required")
	}
	if s.Strategy.Approach == "" {
		errs = append(errs, "strategy.approach is required")
	}

	// Validate enum: memory type
	switch s.Context.Memory {
	case MemoryShortTerm, MemoryConversation, MemoryLongTerm:
		// valid
	default:
		errs = append(errs, fmt.Sprintf(
			"invalid memory type %q: must be one of short-term, conversation, long-term",
			s.Context.Memory,
		))
	}

	// Validate enum: trace level
	switch s.Observability.TraceLevel {
	case TraceLevelMinimal, TraceLevelStandard, TraceLevelDetailed:
		// valid
	default:
		errs = append(errs, fmt.Sprintf(
			"invalid trace level %q: must be one of minimal, standard, detailed",
			s.Observability.TraceLevel,
		))
	}

	// Validate enum: filesystem access level
	switch s.Security.Filesystem {
	case AccessNone, AccessReadOnly, AccessReadWrite, AccessFull:
		// valid
	default:
		errs = append(errs, fmt.Sprintf(
			"invalid filesystem access level %q: must be one of none, read-only, read-write, full",
			s.Security.Filesystem,
		))
	}

	// Validate enum: network access
	switch s.Security.Network {
	case NetworkNone, NetworkAllowlist, NetworkFull:
		// valid
	default:
		errs = append(errs, fmt.Sprintf(
			"invalid network access %q: must be one of none, allowlist, full",
			s.Security.Network,
		))
	}

	// Validate enum: sandbox type (optional — empty is valid)
	if s.Security.Sandbox != "" {
		switch s.Security.Sandbox {
		case SandboxNone, SandboxContainer, SandboxVM:
			// valid
		default:
			errs = append(errs, fmt.Sprintf(
				"invalid sandbox type %q: must be one of none, container, vm",
				s.Security.Sandbox,
			))
		}
	}

	// Validate enum: negotiation strategy
	switch s.Negotiation.FileConflicts {
	case NegotiationYield, NegotiationOverride, NegotiationMerge:
		// valid
	default:
		errs = append(errs, fmt.Sprintf(
			"invalid negotiation strategy %q: must be one of yield, override, merge",
			s.Negotiation.FileConflicts,
		))
	}

	// Validate enum: effort level (optional — empty defaults to medium)
	if s.Strategy.Effort != "" {
		switch s.Strategy.Effort {
		case EffortLight, EffortMedium, EffortHeavy:
			// valid
		default:
			errs = append(errs, fmt.Sprintf(
				"invalid effort level %q: must be one of light, medium, heavy",
				s.Strategy.Effort,
			))
		}
	}

	return errs
}

// ValidateAgent checks an AgentComposition for required fields and valid enum values.
// Returns a slice of error messages (empty if valid).
func ValidateAgent(a AgentComposition) []string {
	var errs []string

	if a.Agent == "" {
		errs = append(errs, "agent name is required")
	}

	hasSkills := len(a.Skills) > 0
	hasStages := len(a.Stages) > 0

	if hasSkills && hasStages {
		errs = append(errs, "skills and stages are mutually exclusive")
	}

	if !hasSkills && !hasStages {
		errs = append(errs, "at least one skill is required")
	}

	if hasSkills {
		switch a.Orchestration {
		case OrchestrationSequential, OrchestrationParallel,
			OrchestrationParallelThenMerge, OrchestrationAdaptive:
		default:
			errs = append(errs, fmt.Sprintf(
				"invalid orchestration strategy %q: must be one of sequential, parallel, parallel-then-merge, adaptive",
				a.Orchestration,
			))
		}
	}

	if hasStages {
		seen := make(map[string]bool)
		for i, stage := range a.Stages {
			if stage.Name == "" {
				errs = append(errs, fmt.Sprintf("stage at index %d has no name", i))
			} else if seen[stage.Name] {
				errs = append(errs, fmt.Sprintf("duplicate stage name %q", stage.Name))
			}
			seen[stage.Name] = true

			if len(stage.Skills) == 0 {
				errs = append(errs, fmt.Sprintf("stage %q must have at least one skill", stage.Name))
			}

			switch stage.Strategy {
			case OrchestrationSequential, OrchestrationParallel,
				OrchestrationParallelThenMerge, OrchestrationAdaptive:
			default:
				errs = append(errs, fmt.Sprintf("stage %q has invalid strategy %q", stage.Name, stage.Strategy))
			}
		}
	}

	return errs
}
