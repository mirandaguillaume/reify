package static

import (
	"fmt"
	"strings"

	"github.com/mirandaguillaume/reify/internal/doctor/llmutil"
	"github.com/mirandaguillaume/reify/internal/doctor/parser"
)

func init() {
	RegisterCheck(&unclosedFenceCheck{})
}

// unclosedFenceCheck detects code fences that are opened but never closed.
//
// Many static checks (padding, secrets, drift, vague, readability, duplicates)
// toggle an `inCodeFence` flag line-by-line and skip content inside fences.
// When a file has an unclosed fence, the toggle stays open through end-of-file
// and the rest of the content is silently skipped — meaning real findings can
// hide inside what appears to be "code". Surfacing the unclosed fence as its
// own finding warns the user that something is wrong with the document.
//
// Story 4-0 AC #6.
type unclosedFenceCheck struct{}

func (u *unclosedFenceCheck) ID() string              { return "unclosed-code-fence" }
func (u *unclosedFenceCheck) Tags() []string          { return []string{"default"} }
func (u *unclosedFenceCheck) Category() string        { return "structure" }
func (u *unclosedFenceCheck) DefaultSeverity() string { return "moderate" }

func (u *unclosedFenceCheck) Run(analysis *parser.AgentAnalysis) []llmutil.Finding {
	if analysis == nil || len(analysis.RawContent) == 0 {
		return nil
	}

	lines := strings.Split(NormalizeContent(string(analysis.RawContent)), "\n")
	// Use a depth counter instead of a bool toggle: depth>0 means we are inside
	// an unclosed fence. Fences in Markdown do not nest, so depth is either 0
	// (closed) or 1 (open). The counter makes the invariant explicit and correctly
	// handles an odd total number of ``` lines by reporting the first unclosed one.
	depth := 0
	openFenceLine := 0

	for lineNum, line := range lines {
		if IsCodeFenceLine(strings.TrimSpace(line)) {
			if depth == 0 {
				// Opening a new fence — record this line in case it stays unclosed.
				openFenceLine = lineNum + 1
				depth = 1
			} else {
				// Closing the open fence.
				depth = 0
				openFenceLine = 0
			}
		}
	}

	if depth == 0 {
		return nil
	}

	return []llmutil.Finding{{
		Category:             "structure",
		Issue:                "Unclosed code fence — content after this line is silently skipped by static checks",
		Confidence:           "high",
		CurrentState:         fmt.Sprintf("Opening fence at line %d", openFenceLine),
		SuggestedImprovement: "Close the code fence with ``` on its own line so subsequent content is analyzed.",
	}}
}
