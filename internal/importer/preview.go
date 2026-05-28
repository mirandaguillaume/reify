package importer

import (
	"fmt"
	"io"
	"strings"

	"github.com/mirandaguillaume/reify/internal/linter"
)

// FormatPreview writes a human-readable dry-run summary of an ImportResult to w.
// It shows the decomposition summary, per-skill quality details, agent details
// when present, and any accumulated warnings.
func FormatPreview(result ImportResult, w io.Writer) {
	// Header line: decomposition summary.
	if result.Agent != nil {
		fmt.Fprintf(w, "Decomposition: Input agent → %d skill(s) + 1 agent\n", len(result.Skills))
	} else {
		fmt.Fprintf(w, "Result: %d skill(s)\n", len(result.Skills))
	}

	fmt.Fprintln(w)

	// Per-skill section.
	for _, sr := range result.Skills {
		fmt.Fprintf(w, "Skill: %s  [%d/100]\n", sr.Skill.Skill, sr.Score.Total)

		// Context: consumes / produces.
		if len(sr.Skill.Context.Consumes) > 0 {
			fmt.Fprintf(w, "  consumes: %s\n", strings.Join(sr.Skill.Context.Consumes, ", "))
		}
		if len(sr.Skill.Context.Produces) > 0 {
			fmt.Fprintf(w, "  produces: %s\n", strings.Join(sr.Skill.Context.Produces, ", "))
		}

		// Security summary.
		fmt.Fprintf(w, "  security: filesystem=%s  network=%s\n",
			sr.Skill.Security.Filesystem, sr.Skill.Security.Network)

		// Lint issues.
		hasIssues := false
		for _, li := range sr.LintIssues {
			prefix := "  ⚠"
			if li.Severity == linter.SeverityError {
				prefix = "  ✗"
			}
			fmt.Fprintf(w, "%s [%s] %s\n", prefix, li.Rule, li.Message)
			hasIssues = true
		}

		// Loop risks.
		for _, lr := range sr.LoopRisks {
			fmt.Fprintf(w, "  ⚠ loop risk (%s): %s\n", lr.Type, lr.Message)
			hasIssues = true
		}

		if !hasIssues {
			fmt.Fprintln(w, "  ✓ all checks pass")
		}

		fmt.Fprintln(w)
	}

	// Agent section.
	if result.Agent != nil {
		ar := result.Agent
		fmt.Fprintf(w, "Agent: %s  [%d/100]\n", ar.Agent.Agent, ar.Score.Total)

		if len(ar.Agent.Skills) > 0 {
			fmt.Fprintf(w, "  skills: %s\n", strings.Join(ar.Agent.Skills, ", "))
		}
		if ar.Agent.Orchestration != "" {
			fmt.Fprintf(w, "  orchestration: %s\n", ar.Agent.Orchestration)
		}
		if len(ar.Agent.Consumes) > 0 {
			fmt.Fprintf(w, "  consumes: %s\n", strings.Join(ar.Agent.Consumes, ", "))
		}
		if len(ar.Agent.Produces) > 0 {
			fmt.Fprintf(w, "  produces: %s\n", strings.Join(ar.Agent.Produces, ", "))
		}

		// Dependency issues.
		if len(ar.DepIssues) > 0 {
			for _, di := range ar.DepIssues {
				fmt.Fprintf(w, "  ✗ dependency [%s]: %s\n", di.Type, di.Message)
			}
		} else {
			fmt.Fprintln(w, "  ✓ dependencies satisfied")
		}

		// Ordering issues.
		if len(ar.OrderingIssues) > 0 {
			for _, oi := range ar.OrderingIssues {
				fmt.Fprintf(w, "  ⚠ ordering [%s]: %s\n", oi.Type, oi.Message)
			}
		} else {
			fmt.Fprintln(w, "  ✓ skill ordering valid")
		}

		fmt.Fprintln(w)
	}

	// Contracts section.
	if len(result.Contracts) > 0 {
		fmt.Fprintf(w, "Contracts: %d extracted\n", len(result.Contracts))
		for name := range result.Contracts {
			fmt.Fprintf(w, "  %s.md\n", name)
		}
		fmt.Fprintln(w)
	}

	// Warnings at the end.
	if len(result.Warnings) > 0 {
		fmt.Fprintln(w, "Warnings:")
		for _, w2 := range result.Warnings {
			fmt.Fprintf(w, "  - %s\n", w2)
		}
	}
}
