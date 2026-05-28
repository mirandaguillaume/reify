// Package qualitygate provides a DAG middleware that validates node outputs
// against format templates declared in SlotSpec contracts. When required
// structural markers are missing, the middleware returns an error that triggers
// the DAG's built-in retry mechanism (runWithRetry).
//
// Validation mode: AST structural validation — used by ValidateProduces /
// ValidateConsumes when the template has ## or ### headings. goldmark parses
// both template and output; invariants (heading present, section non-empty,
// list present) are asserted.
//
// Templates with no headings are pass-through (no structural constraint).
// Empty template = pass-through (NFR40 backward compatibility).
package qualitygate

import (
	"context"
	"fmt"
	"sort"
	"strings"

	"github.com/mirandaguillaume/reify/pkg/dag"
)

// QualityGatePolicy defines the format contracts for a step's inputs and outputs.
// Templates come from SlotSpec.ResolveTemplate() on consumes/produces slots.
// Empty template = no constraint (pass-through).
type QualityGatePolicy struct {
	// ProducesTemplate: format template validated against node output after execution.
	ProducesTemplate string
	// ConsumesTemplate: format template validated against node input before execution.
	ConsumesTemplate string
}

// QualityGateMiddleware returns a dag.Middleware that enforces structural format
// contracts declared in the policy templates.
//
// Pre-check: validates inputs against ConsumesTemplate before calling next.
// Post-check: validates output against ProducesTemplate after calling next.
//
// Consumes errors are wrapped as "consumes: <err>" so consumesRetryMiddleware
// can distinguish them from produces errors (both share the same AST error prefixes).
//
// If policy is nil, returns a no-op pass-through.
func QualityGateMiddleware(policy *QualityGatePolicy) dag.Middleware {
	if policy == nil {
		return passthrough
	}

	return func(ctx context.Context, inputs map[string]any, next dag.MiddlewareFunc) (map[string]any, error) {
		if policy.ConsumesTemplate != "" {
			if err := ValidateConsumes(policy.ConsumesTemplate, collectStrings(inputs)); err != nil {
				return nil, fmt.Errorf("consumes: %w", err)
			}
		}

		out, err := next(ctx, inputs)
		if err != nil {
			return nil, err
		}

		if policy.ProducesTemplate != "" {
			if err := ValidateProduces(policy.ProducesTemplate, collectStrings(out)); err != nil {
				return nil, err
			}
		}

		return out, nil
	}
}

// ValidateProduces checks a single text against a template's output contract.
// For templates with ## / ### headings it uses goldmark AST structural
// validation. Flat templates (no headings) are pass-through.
//
// Empty template = no constraint (returns nil).
func ValidateProduces(template, text string) error {
	if template == "" {
		return nil
	}
	reqs := extractStructuralRequirements(template)
	if reqs == nil {
		return nil
	}
	src := []byte(text)
	return validateASTStructure(reqs, parseMarkdown(src), src)
}

// ValidateConsumes mirrors ValidateProduces for the input side.
func ValidateConsumes(template, text string) error {
	if template == "" {
		return nil
	}
	reqs := extractStructuralRequirements(template)
	if reqs == nil {
		return nil
	}
	src := []byte(text)
	return validateASTStructure(reqs, parseMarkdown(src), src)
}

// passthrough is a no-op middleware used when no constraints are active.
var passthrough dag.Middleware = func(ctx context.Context, inputs map[string]any, next dag.MiddlewareFunc) (map[string]any, error) {
	return next(ctx, inputs)
}

// collectStrings concatenates all string values from the output map,
// sorted by key, separated by "\n\n" so markdown headings in individual
// values remain at line-start boundaries after concatenation (required for
// AST heading detection). Keys are sorted to make validation deterministic
// across calls with identical inputs. Non-string values are silently skipped.
func collectStrings(outputs map[string]any) string {
	keys := make([]string, 0, len(outputs))
	for k := range outputs {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	var b strings.Builder
	for _, k := range keys {
		if s, ok := outputs[k].(string); ok {
			if b.Len() > 0 {
				b.WriteString("\n\n")
			}
			b.WriteString(s)
		}
	}
	return b.String()
}
