package llm

import "context"

// Provider abstracts an LLM API for text completion.
type Provider interface {
	Complete(prompt string) (string, error)
}

// Tool is an executable capability offered to an LLM during agentic execution.
// Run is called by the provider when the LLM emits a tool_use block.
type Tool struct {
	Name        string
	Description string
	InputSchema map[string]any
	Run         func(ctx context.Context, input map[string]any) (string, error)
}

// AgenticStats reports the resources consumed by a CompleteWithTools call.
type AgenticStats struct {
	APICalls  int
	ToolCalls map[string]int // tool name → number of invocations
}

// AgenticOptions configures a CompleteWithTools call.
type AgenticOptions struct {
	// OnIteration is called before each API request (0 = initial call).
	// Return a non-nil error to abort the loop immediately.
	OnIteration func(iterNum int) error
}

// AgenticProvider extends Provider with multi-turn tool-use support.
// CompleteWithTools runs the agentic loop: prompt → tool calls → results → … → final text.
type AgenticProvider interface {
	Provider
	CompleteWithTools(ctx context.Context, prompt string, tools []Tool, opts ...AgenticOptions) (string, AgenticStats, error)
}

// StructuredProvider extends Provider with constrained-decoding support.
// Providers that implement this interface (e.g. Anthropic Structured Outputs)
// return a typed map[string]any directly, eliminating format non-determinism.
type StructuredProvider interface {
	Provider
	CompleteStructured(ctx context.Context, prompt string, jsonSchema map[string]any) (map[string]any, error)
}

// TokenUsage holds the actual token counts returned by the API for one call.
type TokenUsage struct {
	InputTokens  int
	OutputTokens int
}

// TokenAwareProvider is an optional extension to Provider for providers that
// return real token counts from the API response (not a char/4 estimate).
// callCountingProvider checks for this interface and uses the exact counts
// when available, falling back to the char/4 heuristic otherwise.
type TokenAwareProvider interface {
	Provider
	CompleteWithUsage(prompt string) (string, TokenUsage, error)
}
