package qualitygate

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/mirandaguillaume/reify/pkg/dag"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// handler is a test helper that returns fixed outputs.
func handler(outputs map[string]any) dag.MiddlewareFunc {
	return func(ctx context.Context, inputs map[string]any) (map[string]any, error) {
		return outputs, nil
	}
}

// failHandler returns a fixed error.
func failHandler(err error) dag.MiddlewareFunc {
	return func(ctx context.Context, inputs map[string]any) (map[string]any, error) {
		return nil, err
	}
}

// ─── Policy passthrough tests ──────────────────────────────────────────────

func TestNilPolicy_Passthrough(t *testing.T) {
	mw := QualityGateMiddleware(nil)
	out := map[string]any{"result": "hello world"}
	got, err := mw(context.Background(), nil, handler(out))
	require.NoError(t, err)
	assert.Equal(t, out, got)
}

func TestEmptyTemplates_Passthrough(t *testing.T) {
	mw := QualityGateMiddleware(&QualityGatePolicy{})
	out := map[string]any{"result": "hello world"}
	got, err := mw(context.Background(), nil, handler(out))
	require.NoError(t, err)
	assert.Equal(t, out, got)
}

func TestNoMarkersInTemplate_Passthrough(t *testing.T) {
	// A template that produces no markers (plain prose) → pass-through
	mw := QualityGateMiddleware(&QualityGatePolicy{
		ProducesTemplate: "Just some prose with no headings or list items.",
	})
	out := map[string]any{"result": "anything goes"}
	got, err := mw(context.Background(), nil, handler(out))
	require.NoError(t, err)
	assert.Equal(t, out, got)
}

// ─── ProducesTemplate tests ────────────────────────────────────────────────

func TestProducesTemplate_Pass(t *testing.T) {
	policy := &QualityGatePolicy{
		ProducesTemplate: "## Bugs\n- **location**: description",
	}
	mw := QualityGateMiddleware(policy)

	out := map[string]any{
		"bugs": "## Bugs\n- **src/main.go:12**: off-by-one error in loop boundary",
	}
	got, err := mw(context.Background(), nil, handler(out))
	require.NoError(t, err)
	assert.Equal(t, out, got)
}

func TestProducesTemplate_Fail_MissingHeading(t *testing.T) {
	policy := &QualityGatePolicy{
		ProducesTemplate: "## Bugs\n- **location**: description",
	}
	mw := QualityGateMiddleware(policy)

	out := map[string]any{
		"bugs": "I found a bug at line 12. The loop counter overflows.",
	}
	_, err := mw(context.Background(), nil, handler(out))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "quality gate")
	assert.Contains(t, err.Error(), "## bugs")
}

func TestProducesTemplate_Fail_MissingListItem(t *testing.T) {
	policy := &QualityGatePolicy{
		ProducesTemplate: "## Bugs\n- **location**: description",
	}
	mw := QualityGateMiddleware(policy)

	// Has the heading but no bold list items
	out := map[string]any{
		"bugs": "## Bugs\nThere are no bugs in this code.",
	}
	_, err := mw(context.Background(), nil, handler(out))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "quality gate")
}

func TestProducesTemplate_CaseInsensitive(t *testing.T) {
	policy := &QualityGatePolicy{
		ProducesTemplate: "## Bugs",
	}
	mw := QualityGateMiddleware(policy)

	// Heading with different casing
	out := map[string]any{"result": "## BUGS\nsome findings"}
	got, err := mw(context.Background(), nil, handler(out))
	require.NoError(t, err)
	assert.Equal(t, out, got)
}

func TestProducesTemplate_MultipleOutputValues(t *testing.T) {
	policy := &QualityGatePolicy{
		ProducesTemplate: "## Summary",
	}
	mw := QualityGateMiddleware(policy)

	// Marker appears in second output value
	out := map[string]any{
		"part1": "some analysis",
		"part2": "## Summary\nAll done.",
	}
	got, err := mw(context.Background(), nil, handler(out))
	require.NoError(t, err)
	assert.Equal(t, out, got)
}

// ─── ConsumesTemplate tests ────────────────────────────────────────────────

func TestConsumesTemplate_Pass(t *testing.T) {
	policy := &QualityGatePolicy{
		ConsumesTemplate: "## Bugs\n- **location**: description",
	}
	mw := QualityGateMiddleware(policy)

	inputs := map[string]any{
		"bugs": "## Bugs\n- **src/main.go**: buffer overflow",
	}
	out := map[string]any{"fixed": "patch applied"}
	got, err := mw(context.Background(), inputs, handler(out))
	require.NoError(t, err)
	assert.Equal(t, out, got)
}

func TestConsumesTemplate_Fail(t *testing.T) {
	policy := &QualityGatePolicy{
		ConsumesTemplate: "## Bugs\n- **location**: description",
	}
	mw := QualityGateMiddleware(policy)

	// Input is missing the expected structure
	inputs := map[string]any{
		"bugs": "here are some issues i found",
	}
	out := map[string]any{"fixed": "whatever"}
	_, err := mw(context.Background(), inputs, handler(out))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "quality gate")
}

func TestConsumesTemplate_FailDoesNotCallHandler(t *testing.T) {
	policy := &QualityGatePolicy{
		ConsumesTemplate: "## Required",
	}
	mw := QualityGateMiddleware(policy)

	handlerCalled := false
	h := func(ctx context.Context, inputs map[string]any) (map[string]any, error) {
		handlerCalled = true
		return map[string]any{}, nil
	}

	_, err := mw(context.Background(), map[string]any{"x": "no heading here"}, h)
	require.Error(t, err)
	assert.False(t, handlerCalled, "handler must not be called when input pre-check fails")
}

func TestBothTemplates_Pass(t *testing.T) {
	policy := &QualityGatePolicy{
		ConsumesTemplate: "## Input",
		ProducesTemplate: "## Output",
	}
	mw := QualityGateMiddleware(policy)

	inputs := map[string]any{"in": "## Input\nsome data"}
	out := map[string]any{"out": "## Output\nsome result"}
	got, err := mw(context.Background(), inputs, handler(out))
	require.NoError(t, err)
	assert.Equal(t, out, got)
}

func TestProducesTemplate_AsteriskBulletInOutput_Pass(t *testing.T) {
	// goldmark parses both "- item" and "* item" as list nodes, so asterisk-bullet
	// output satisfies a dash-bullet template's needsList constraint.
	policy := &QualityGatePolicy{
		ProducesTemplate: "## Bugs\n- **location**: description",
	}
	mw := QualityGateMiddleware(policy)

	out := map[string]any{
		"bugs": "## Bugs\n* **src/main.go:12**: off-by-one error in loop boundary",
	}
	got, err := mw(context.Background(), nil, handler(out))
	require.NoError(t, err)
	assert.Equal(t, out, got)
}

// ─── Handler error passthrough ─────────────────────────────────────────────

func TestHandlerError_Passthrough(t *testing.T) {
	policy := &QualityGatePolicy{
		ProducesTemplate: "## Expected",
	}
	mw := QualityGateMiddleware(policy)

	innerErr := errors.New("LLM provider timeout")
	_, err := mw(context.Background(), nil, failHandler(innerErr))
	require.Error(t, err)
	assert.Equal(t, innerErr, err)
}

// ─── Non-string outputs ────────────────────────────────────────────────────

func TestNonStringOutputsIgnored(t *testing.T) {
	policy := &QualityGatePolicy{
		ProducesTemplate: "## Found",
	}
	mw := QualityGateMiddleware(policy)

	out := map[string]any{
		"count":   42,
		"active":  true,
		"tags":    []any{"a", "b"},
		"message": "## Found\nsome issue",
	}
	got, err := mw(context.Background(), nil, handler(out))
	require.NoError(t, err)
	assert.Equal(t, out, got)
}

func TestNonStringOutputsOnly_MissingStructure(t *testing.T) {
	policy := &QualityGatePolicy{
		ProducesTemplate: "## Expected",
	}
	mw := QualityGateMiddleware(policy)

	out := map[string]any{
		"count":  42,
		"active": true,
	}
	_, err := mw(context.Background(), nil, handler(out))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "quality gate")
}

func TestEmptyStringOutput_MissingStructure(t *testing.T) {
	policy := &QualityGatePolicy{
		ProducesTemplate: "## Expected",
	}
	mw := QualityGateMiddleware(policy)

	out := map[string]any{"result": ""}
	_, err := mw(context.Background(), nil, handler(out))
	require.Error(t, err)
}

func TestEmptyOutputMap_MissingStructure(t *testing.T) {
	policy := &QualityGatePolicy{
		ProducesTemplate: "## Expected",
	}
	mw := QualityGateMiddleware(policy)

	_, err := mw(context.Background(), nil, handler(map[string]any{}))
	require.Error(t, err)
}

// ─── DAG integration tests ─────────────────────────────────────────────────

func TestDAGRetryIntegration(t *testing.T) {
	calls := 0
	policy := &QualityGatePolicy{
		ProducesTemplate: "## Findings\n- **location**: description",
	}

	node := &dag.Node{
		ID:       "analyze",
		Kind:     dag.KindTask,
		Consumes: []string{"input"},
		Produces: []string{"output"},
		Run: func(ctx context.Context, inputs map[string]any) (map[string]any, error) {
			calls++
			if calls == 1 {
				// First attempt: missing required structure
				return map[string]any{
					"output": "I found some issues in the code.",
				}, nil
			}
			// Second attempt: proper structured output
			return map[string]any{
				"output": "## Findings\n- **src/main.go:12**: buffer overflow detected",
			}, nil
		},
		MaxRetries:  1,
		Middlewares: []dag.Middleware{QualityGateMiddleware(policy)},
	}

	d, err := dag.New(node)
	require.NoError(t, err)

	out, err := d.Execute(context.Background(), dag.WithInputs(map[string]any{
		"input": "check this code",
	}))
	require.NoError(t, err)
	assert.Equal(t, 2, calls, "expected exactly 2 calls (1 failed + 1 retry)")
	assert.Contains(t, out["output"].(string), "## Findings")
}

func TestDAGRetryExhausted(t *testing.T) {
	policy := &QualityGatePolicy{
		ProducesTemplate: "## Required",
	}

	node := &dag.Node{
		ID:       "stuck",
		Kind:     dag.KindTask,
		Consumes: []string{"input"},
		Produces: []string{"output"},
		Run: func(ctx context.Context, inputs map[string]any) (map[string]any, error) {
			return map[string]any{"output": "some generic response without structure"}, nil
		},
		MaxRetries:  2,
		Middlewares: []dag.Middleware{QualityGateMiddleware(policy)},
	}

	d, err := dag.New(node)
	require.NoError(t, err)

	_, err = d.Execute(context.Background(), dag.WithInputs(map[string]any{
		"input": "check this code",
	}))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "quality gate")
}

// ─── collectStrings tests ──────────────────────────────────────────────────

func TestCollectStrings(t *testing.T) {
	tests := []struct {
		name    string
		outputs map[string]any
		want    []string
	}{
		{
			name:    "empty map",
			outputs: map[string]any{},
			want:    []string{},
		},
		{
			name:    "single string",
			outputs: map[string]any{"a": "hello"},
			want:    []string{"hello"},
		},
		{
			name:    "mixed types — non-strings skipped",
			outputs: map[string]any{"a": "hello", "b": 42, "c": "world"},
			want:    []string{"hello", "world"},
		},
		{
			name:    "heading in second value stays line-start after join",
			outputs: map[string]any{"a": "intro text", "b": "## Section\ncontent"},
			want:    []string{"intro text", "## Section"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := collectStrings(tt.outputs)
			for _, want := range tt.want {
				assert.Contains(t, got, want)
			}
		})
	}
}

// ─── Middleware chain composition ──────────────────────────────────────────

func TestMiddlewareChainComposition(t *testing.T) {
	var order []string

	logging := func(ctx context.Context, inputs map[string]any, next dag.MiddlewareFunc) (map[string]any, error) {
		order = append(order, "logging-pre")
		out, err := next(ctx, inputs)
		order = append(order, "logging-post")
		return out, err
	}

	policy := &QualityGatePolicy{ProducesTemplate: "## OK"}
	gate := QualityGateMiddleware(policy)

	chain := dag.ChainMiddlewares(logging, gate)

	h := handler(map[string]any{"result": "## OK\neverything is fine"})
	out, err := chain(context.Background(), nil, h)
	require.NoError(t, err)
	assert.Contains(t, out["result"].(string), "## OK")
	assert.Equal(t, []string{"logging-pre", "logging-post"}, order)
}

func TestMiddlewareChainComposition_GateRejects(t *testing.T) {
	var outerSawError bool

	logging := func(ctx context.Context, inputs map[string]any, next dag.MiddlewareFunc) (map[string]any, error) {
		out, err := next(ctx, inputs)
		if err != nil {
			outerSawError = true
		}
		return out, err
	}

	policy := &QualityGatePolicy{ProducesTemplate: "## Missing"}
	gate := QualityGateMiddleware(policy)

	chain := dag.ChainMiddlewares(logging, gate)

	h := handler(map[string]any{"result": "no structure here"})
	_, err := chain(context.Background(), nil, h)
	require.Error(t, err)
	assert.True(t, outerSawError)
	assert.Contains(t, err.Error(), "quality gate")
}

// ─── Input/context passthrough ─────────────────────────────────────────────

func TestInputsPassedThrough(t *testing.T) {
	policy := &QualityGatePolicy{
		ProducesTemplate: "## Result",
	}
	mw := QualityGateMiddleware(policy)

	var receivedInputs map[string]any
	h := func(ctx context.Context, inputs map[string]any) (map[string]any, error) {
		receivedInputs = inputs
		return map[string]any{"result": "## Result\nok"}, nil
	}

	inputs := map[string]any{"task": "analyze", "code": "func main() {}"}
	_, err := mw(context.Background(), inputs, h)
	require.NoError(t, err)
	assert.Equal(t, inputs, receivedInputs)
}

func TestContextPropagated(t *testing.T) {
	type ctxKey string
	policy := &QualityGatePolicy{
		ProducesTemplate: "## Result",
	}
	mw := QualityGateMiddleware(policy)

	ctx := context.WithValue(context.Background(), ctxKey("test"), "value")
	var receivedCtx context.Context
	h := func(ctx context.Context, inputs map[string]any) (map[string]any, error) {
		receivedCtx = ctx
		return map[string]any{"result": "## Result\nok"}, nil
	}

	_, err := mw(ctx, nil, h)
	require.NoError(t, err)
	assert.Equal(t, "value", receivedCtx.Value(ctxKey("test")))
}

// ─── Concurrent execution ──────────────────────────────────────────────────

func TestConcurrentDAGExecution(t *testing.T) {
	policy := &QualityGatePolicy{
		ProducesTemplate: "## Done",
	}

	nodes := make([]*dag.Node, 3)
	for i := 0; i < 3; i++ {
		idx := i
		nodes[i] = &dag.Node{
			ID:       fmt.Sprintf("node-%d", idx),
			Kind:     dag.KindTask,
			Produces: []string{fmt.Sprintf("out-%d", idx)},
			Run: func(ctx context.Context, inputs map[string]any) (map[string]any, error) {
				time.Sleep(time.Millisecond)
				return map[string]any{
					fmt.Sprintf("out-%d", idx): fmt.Sprintf("## Done\ntask %d complete", idx),
				}, nil
			},
			Middlewares: []dag.Middleware{QualityGateMiddleware(policy)},
		}
	}

	d, err := dag.New(nodes...)
	require.NoError(t, err)

	out, err := d.Execute(context.Background())
	require.NoError(t, err)

	for i := 0; i < 3; i++ {
		key := fmt.Sprintf("out-%d", i)
		assert.Contains(t, out[key].(string), "## Done")
	}
}


// ─── Wording regression (telemetry stability) ─────────────────────────────

// Pin the EXACT wording of qualitygate error messages — story 10-6 telemetry
// parses these strings. A typo or rewording must break these tests.
func TestQualityGate_ErrorWording_StableForTelemetry(t *testing.T) {
	t.Run("heading not found in output (AST)", func(t *testing.T) {
		err := ValidateProduces("## Verdict", "no heading here")
		require.Error(t, err)
		assert.Equal(t, `quality gate: heading "## verdict" not found`, err.Error())
	})

	t.Run("heading not found in input (AST)", func(t *testing.T) {
		err := ValidateConsumes("## Verdict", "no heading here")
		require.Error(t, err)
		assert.Equal(t, `quality gate: heading "## verdict" not found`, err.Error())
	})

	t.Run("flat template is passthrough — no error", func(t *testing.T) {
		err := ValidateProduces("- **field**: value", "no structure at all")
		require.NoError(t, err, "flat templates produce no structural constraint")
	})
}


// ─── Benchmarks ────────────────────────────────────────────────────────────

func BenchmarkQualityGateMiddleware_NoTemplates(b *testing.B) {
	mw := QualityGateMiddleware(nil)
	out := map[string]any{"result": "hello world"}
	h := handler(out)
	ctx := context.Background()

	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		_, _ = mw(ctx, nil, h)
	}
}

func BenchmarkQualityGateMiddleware_Template_Pass(b *testing.B) {
	policy := &QualityGatePolicy{
		ProducesTemplate: "## Findings\n- **location**: description",
	}
	mw := QualityGateMiddleware(policy)
	out := map[string]any{
		"result": strings.Repeat("## Findings\n- **src/main.go**: issue found. ", 10),
	}
	h := handler(out)
	ctx := context.Background()

	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		_, _ = mw(ctx, nil, h)
	}
}

func BenchmarkQualityGateMiddleware_Template_Fail(b *testing.B) {
	policy := &QualityGatePolicy{
		ProducesTemplate: "## Findings\n- **location**: description",
	}
	mw := QualityGateMiddleware(policy)
	out := map[string]any{
		"result": strings.Repeat("I found some issues in the code. ", 10),
	}
	h := handler(out)
	ctx := context.Background()

	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		_, _ = mw(ctx, nil, h)
	}
}

// ─── AST integration tests (Story 10-11, T6.1–T6.6) ─────────────────────────

// T6.1 — AC2: heading present but section is empty (no content beneath).
func TestValidateProduces_AST_EmptySection_Fails(t *testing.T) {
	tmpl := "## Verdict\n\nDecision text.\n"
	err := ValidateProduces(tmpl, "## Verdict\n")
	require.Error(t, err)
	assert.Equal(t, `quality gate: section "## verdict" is empty`, err.Error())
}

// T6.2 — AC3: section present but contains no list where template declares one.
func TestValidateProduces_AST_MissingList_Fails(t *testing.T) {
	tmpl := "## Issues\n\n- bug one\n- bug two\n"
	err := ValidateProduces(tmpl, "## Issues\n\nNo issues found in this run.\n")
	require.Error(t, err)
	assert.Equal(t, `quality gate: section "## issues" requires a list`, err.Error())
}

// T6.3 — AC4: required heading entirely absent from output.
func TestValidateProduces_AST_HeadingAbsent_Fails(t *testing.T) {
	tmpl := "## Verdict\n\nBody.\n## Summary\n\nMore body.\n"
	err := ValidateProduces(tmpl, "## Verdict\n\nApproved.\n")
	require.Error(t, err)
	assert.Equal(t, `quality gate: heading "## summary" not found`, err.Error())
}

// T6.4 — HTML comments in template have no validation effect.
func TestValidateProduces_AST_HTMLCommentsIgnored(t *testing.T) {
	// Template has heading + HTML comment — comment has no validation effect.
	tmpl := "## Verdict\n\nDecision.\n<!-- some comment -->\n"

	// Output satisfies the structural check (heading + body) → passes.
	err := ValidateProduces(tmpl, "## Verdict\n\nApproved.\n")
	require.NoError(t, err)
}

// T6.5 — flat templates (no headings) are pass-through — no constraint applied.
func TestValidateProduces_NoHeadings_Passthrough(t *testing.T) {
	tmpl := "- **verdict**: ...\n- **severity**: ..."
	// Any output satisfies a flat template — heading-based AST validation requires headings.
	err := ValidateProduces(tmpl, "just plain text with no structure")
	require.NoError(t, err)
}

// T6.6 — AC1 + full multi-section template: all constraints satisfied → nil.
func TestValidateProduces_AST_FullTemplate_Passes(t *testing.T) {
	tmpl := "## Verdict\n\nDecision.\n\n## Issues\n\n- item\n"
	output := "## Verdict\n\nApproved.\n\n## Issues\n\n- minor style nit\n"
	err := ValidateProduces(tmpl, output)
	require.NoError(t, err)
}

// AC1 additional: heading present with content → nil.
func TestValidateProduces_AST_HeadingWithContent_Passes(t *testing.T) {
	tmpl := "## Verdict\n\nSome content"
	err := ValidateProduces(tmpl, "## Verdict\n\nApproved — no issues found.")
	require.NoError(t, err)
}
