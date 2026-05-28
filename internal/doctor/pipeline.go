package doctor

import (
	"github.com/mirandaguillaume/reify/internal/doctor/llmutil"
	"github.com/mirandaguillaume/reify/internal/doctor/parser"
	"github.com/mirandaguillaume/reify/internal/doctor/static"
)

// PipelineOpts configures the doctor pipeline.
type PipelineOpts struct {
	Mode         string // "default", "quick", "thorough", "security"
	MaxFindings  int
	Gate         *QualityGate
	SectionCount int // number of section categories for structural score
}

// Report holds the complete result of a doctor pipeline run.
type Report struct {
	FilePath        string
	Format          string
	StructuralScore StructuralResult
	Indicators      []static.Indicator
	StaticFindings  []llmutil.Finding
	LLMFindings     []llmutil.Finding
	AllFindings     []llmutil.Finding // merged + post-processed
	GateResult      GateResult
}

// RunPipeline executes the full doctor pipeline: static → merge → post-process → gate.
// LLM findings are passed in separately (caller handles LLM call + cache).
//
// A nil analysis returns an empty Report rather than panicking — this is a
// defensive guard for boundary callers (e.g., directory mode where a parser
// failure on one file shouldn't crash the whole run). Story 4-0 AC #1.
func RunPipeline(analysis *parser.AgentAnalysis, llmFindings []llmutil.Finding, opts PipelineOpts) *Report {
	if analysis == nil {
		// Return a report that passes (no findings, no gate failures) rather than a
		// zero-value Report whose GateResult.Pass=false would trigger ErrFindings in callers.
		return &Report{GateResult: GateResult{Pass: true}}
	}
	if opts.Gate == nil {
		opts.Gate = DefaultGate()
	}
	if opts.SectionCount <= 0 {
		opts.SectionCount = 15
	}

	report := &Report{
		Format: analysis.Format,
	}

	// Phase 1: Static checks
	report.StaticFindings = static.RunChecks(analysis, opts.Mode)
	report.StructuralScore = ComputeStructural(report.StaticFindings, opts.SectionCount)

	// Phase 1b: Indicators
	report.Indicators = static.CollectIndicators(analysis, opts.Mode)

	// Phase 2: Merge static + LLM
	report.LLMFindings = llmFindings
	var merged []llmutil.Finding
	merged = append(merged, report.StaticFindings...)
	merged = append(merged, report.LLMFindings...)

	// Phase 3: Quality gate (evaluate BEFORE truncation)
	report.GateResult = opts.Gate.Evaluate(report.StructuralScore, merged)

	// Phase 4: Post-process (dedup, sort, limit for display)
	report.AllFindings = PostProcess(merged, opts.MaxFindings)

	return report
}
