package doctor

import (
	"context"
	"fmt"
	"io"
	"os"
	"time"

	"github.com/mirandaguillaume/reify/internal/doctor/analyzer"
	doctorctx "github.com/mirandaguillaume/reify/internal/doctor/context"
	"github.com/mirandaguillaume/reify/internal/doctor/llmutil"
	"github.com/mirandaguillaume/reify/internal/doctor/parser"
	"github.com/mirandaguillaume/reify/internal/doctor/registry"
	"github.com/mirandaguillaume/reify/internal/llm"
	"github.com/mirandaguillaume/reify/pkg/dag"
)

// DoctorInput holds the inputs needed to run the doctor DAG.
type DoctorInput struct {
	FilePath    string
	Content     []byte
	Provider    llm.Provider
	Registry    *registry.Registry
	ProjectRoot string
	Debug       bool
}

// DoctorResult holds the outputs from a doctor DAG execution.
type DoctorResult struct {
	Format      string
	Analysis    *parser.AgentAnalysis
	LLMFindings []llmutil.Finding
	CtxFindings []llmutil.Finding
	AllFindings []llmutil.Finding
}

// BuildDAG constructs the doctor analysis DAG from hardcoded node definitions.
// The embedded YAMLs (SkillSpecs, AgentSpec) establish the spec contract
// but do not yet drive node construction at runtime.
// TODO(FR50): derive topology from parsed embedded specs via internal/yaml/.
//
// The DAG has 4 nodes across 3 layers:
//
//	Layer 0: format-detector
//	Layer 1: analyzer ‖ context-enricher  (parallel)
//	Layer 2: recommendation-builder        (terminal)
//
// Note: Layer 3 (rewriter) was removed from scope in Story 4.2 (Epic 8 retro).
// If re-introduced, set its Run=nil when --fix is false to skip it conditionally.
//
// Each node is wrapped with:
//   - panic recovery (wrapWithPanic) — prevents a single node crash from halting the DAG
//   - retry (wrapWithRetry) on LLM nodes — analyzer retries up to 3 total attempts
//   - debug logging (wrapWithDebug) — emits [DEBUG] lines to stderr when debug=true
func BuildDAG(debug bool) (*dag.DAG, error) {
	var debugWriter io.Writer
	if debug {
		debugWriter = os.Stderr
	}

	applyWrappers := func(n *dag.Node, maxAttempts int) *dag.Node {
		n = wrapWithPanic(n)
		if maxAttempts > 1 {
			n = wrapWithRetry(n, maxAttempts)
		}
		n = wrapWithDebug(n, debugWriter)
		return n
	}

	nodes := []*dag.Node{
		applyWrappers(formatDetectorNode(), 1),       // deterministic: no retry
		applyWrappers(analyzerNode(), 3),             // LLM: 3 total attempts (2 retries)
		applyWrappers(contextEnricherNode(), 1),      // deterministic: no retry
		applyWrappers(recommendationBuilderNode(), 1), // deterministic: no retry
	}
	return dag.New(nodes...)
}

// RunDAG executes the doctor DAG with the given input and returns the result.
// Set concurrency to 1 for local providers (Ollama) to serialize LLM calls.
func RunDAG(ctx context.Context, d *dag.DAG, input DoctorInput, concurrency int) (*DoctorResult, error) {
	inputs := map[string]any{
		"agent_file":      input.Content,
		"agent_file_path": input.FilePath,
		"provider":        input.Provider,
		"registry":        input.Registry,
		"project_root":    input.ProjectRoot,
		"debug":           input.Debug,
	}

	opts := []dag.Option{dag.WithInputs(inputs)}
	if concurrency > 0 {
		opts = append(opts, dag.WithConcurrency(concurrency))
	}

	outputs, err := d.Execute(ctx, opts...)
	if err != nil {
		return nil, fmt.Errorf("doctor DAG execution: %w", err)
	}

	rec, ok := outputs["recommendations"]
	if !ok {
		return nil, fmt.Errorf("doctor DAG: recommendation-builder did not produce output (missing 'recommendations' key)")
	}
	result, ok := rec.(*DoctorResult)
	if !ok {
		return nil, fmt.Errorf("doctor DAG: 'recommendations' output has unexpected type %T", rec)
	}
	return result, nil
}

func formatDetectorNode() *dag.Node {
	return &dag.Node{
		ID:       "format-detector",
		Kind:     dag.KindTask,
		Consumes: []string{"agent_file", "agent_file_path"},
		Produces: []string{"detected_format"},
		Timeout:  5 * time.Second,
		Run: func(ctx context.Context, inputs map[string]any) (map[string]any, error) {
			content, _ := inputs["agent_file"].([]byte)
			path, _ := inputs["agent_file_path"].(string)

			p, err := parser.DetectFormat(path, content)
			if err != nil {
				return nil, fmt.Errorf("format detection: %w", err)
			}
			analysis, err := p.Parse(content)
			if err != nil {
				return nil, fmt.Errorf("parse: %w", err)
			}
			return map[string]any{"detected_format": analysis}, nil
		},
	}
}

func analyzerNode() *dag.Node {
	return &dag.Node{
		ID:       "analyzer",
		Kind:     dag.KindTask,
		Consumes: []string{"detected_format", "provider", "registry"},
		Produces: []string{"analysis_results"},
		Timeout:  120 * time.Second,
		Run: func(ctx context.Context, inputs map[string]any) (map[string]any, error) {
			analysis, _ := inputs["detected_format"].(*parser.AgentAnalysis)
			provider, _ := inputs["provider"].(llm.Provider)
			reg, _ := inputs["registry"].(*registry.Registry)

			if provider == nil {
				return map[string]any{"analysis_results": []llmutil.Finding(nil)}, nil
			}

			findings, err := analyzer.Analyze(analysis, provider, reg)
			if err != nil {
				return nil, fmt.Errorf("LLM analysis: %w", err)
			}
			return map[string]any{"analysis_results": findings}, nil
		},
	}
}

func contextEnricherNode() *dag.Node {
	return &dag.Node{
		ID:       "context-enricher",
		Kind:     dag.KindTask,
		Consumes: []string{"detected_format", "project_root"},
		Produces: []string{"context_recommendations"},
		Timeout:  15 * time.Second,
		Run: func(ctx context.Context, inputs map[string]any) (map[string]any, error) {
			analysis, _ := inputs["detected_format"].(*parser.AgentAnalysis)
			projectRoot, _ := inputs["project_root"].(string)

			findings, err := doctorctx.Enrich(analysis, projectRoot)
			if err != nil {
				// Context enrichment is best-effort; return empty findings on error
				return map[string]any{"context_recommendations": []llmutil.Finding(nil)}, nil
			}
			return map[string]any{"context_recommendations": findings}, nil
		},
	}
}

func recommendationBuilderNode() *dag.Node {
	return &dag.Node{
		ID:       "recommendation-builder",
		Kind:     dag.KindTask,
		Consumes: []string{"detected_format", "analysis_results", "context_recommendations"},
		Produces: []string{"recommendations"},
		Timeout:  5 * time.Second,
		Run: func(ctx context.Context, inputs map[string]any) (map[string]any, error) {
			llmFindings, _ := inputs["analysis_results"].([]llmutil.Finding)
			ctxFindings, _ := inputs["context_recommendations"].([]llmutil.Finding)

			var all []llmutil.Finding
			all = append(all, llmFindings...)
			all = append(all, ctxFindings...)

			result := &DoctorResult{
				AllFindings: all,
				LLMFindings: llmFindings,
				CtxFindings: ctxFindings,
			}
			if analysis, ok := inputs["detected_format"].(*parser.AgentAnalysis); ok {
				result.Analysis = analysis
				result.Format = analysis.Format
			}

			return map[string]any{"recommendations": result}, nil
		},
	}
}
