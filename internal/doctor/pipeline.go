package doctor

import (
	"github.com/mirandaguillaume/reify/internal/doctor/llmutil"
	"github.com/mirandaguillaume/reify/internal/doctor/parser"
)

// PipelineOpts configures the doctor pipeline.
type PipelineOpts struct {
	Mode        string // "default", "thorough", "security"
	MaxFindings int
	Gate        *QualityGate
}

// Report holds the complete result of a doctor pipeline run.
type Report struct {
	FilePath    string
	Format      string
	LLMFindings []llmutil.Finding
	AllFindings []llmutil.Finding // post-processed
	GateResult  GateResult
}

// RunPipeline executes the doctor pipeline: post-process + gate.
// Static checks were removed because they relied on keyword heuristics over
// subjective instruction prose. All findings now come from the LLM analyzer.
func RunPipeline(analysis *parser.AgentAnalysis, llmFindings []llmutil.Finding, opts PipelineOpts) *Report {
	if analysis == nil {
		return &Report{GateResult: GateResult{Pass: true}}
	}
	if opts.Gate == nil {
		opts.Gate = DefaultGate()
	}

	report := &Report{
		Format:      analysis.Format,
		LLMFindings: llmFindings,
	}
	report.GateResult = opts.Gate.Evaluate(llmFindings)
	report.AllFindings = PostProcess(llmFindings, opts.MaxFindings)

	return report
}
