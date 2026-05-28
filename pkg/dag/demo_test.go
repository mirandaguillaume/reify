package dag_test

import (
	"context"
	"fmt"
	"strings"
	"testing"

	"github.com/mirandaguillaume/reify/pkg/dag"
)

// TestDemo_CodeReviewPipeline demonstrates a realistic code-review pipeline
// using the new DAG primitives: Map (per-issue scoring), Filter (threshold),
// Reduce (aggregate), and Gate (halt on empty).
func TestDemo_CodeReviewPipeline(t *testing.T) {
	fmt.Println("\n=== Demo: Code Review Pipeline ===")
	fmt.Println("Pipeline: scan → map(score) → filter(≥70) → reduce(report) → gate(halt if empty)")

	d, err := dag.New(
		// 1. Scanner: finds issues in the diff
		&dag.Node{
			ID:       "scanner",
			Produces: []string{"issues"},
			Run: func(ctx context.Context, in map[string]any) (map[string]any, error) {
				issues := []any{
					map[string]any{"id": "BUG-1", "desc": "null pointer in auth.go:42", "severity": "high"},
					map[string]any{"id": "BUG-2", "desc": "missing error check in db.go:15", "severity": "medium"},
					map[string]any{"id": "STYLE-1", "desc": "unused import in utils.go:3", "severity": "low"},
					map[string]any{"id": "BUG-3", "desc": "race condition in cache.go:88", "severity": "high"},
					map[string]any{"id": "STYLE-2", "desc": "inconsistent naming in api.go:20", "severity": "low"},
				}
				fmt.Printf("  [scanner]  Found %d issues\n", len(issues))
				return map[string]any{"issues": issues}, nil
			},
		},

		// 2. Scorer: scores each issue independently (KindMap — parallel per item)
		&dag.Node{
			ID: "scorer", Kind: dag.KindMap,
			Consumes: []string{"issues"},
			Produces: []string{"scored"},
			Run: func(ctx context.Context, in map[string]any) (map[string]any, error) {
				issue := in["issues"].(map[string]any)
				scores := map[string]int{"high": 90, "medium": 60, "low": 30}
				score := scores[issue["severity"].(string)]
				scored := map[string]any{
					"id": issue["id"], "desc": issue["desc"],
					"score": score,
				}
				fmt.Printf("  [scorer]   %s → score=%d\n", issue["id"], score)
				return map[string]any{"scored": scored}, nil
			},
		},

		// 3. Filter: keep only issues with score ≥ 70 (KindFilter)
		&dag.Node{
			ID: "threshold", Kind: dag.KindFilter,
			Consumes: []string{"scored"},
			Produces: []string{"important"},
			Run: func(ctx context.Context, in map[string]any) (map[string]any, error) {
				issue := in["scored"].(map[string]any)
				keep := issue["score"].(int) >= 70
				if !keep {
					fmt.Printf("  [filter]   %s dropped (score=%d < 70)\n", issue["id"], issue["score"])
				}
				return map[string]any{"__keep": keep}, nil
			},
		},

		// 4. Reducer: aggregate into final report (KindReduce)
		&dag.Node{
			ID: "reporter", Kind: dag.KindReduce,
			Consumes: []string{"important"},
			Produces: []string{"report"},
			Run: func(ctx context.Context, in map[string]any) (map[string]any, error) {
				issue := in["important"].(map[string]any)
				acc := ""
				if v, ok := in["__accumulator"]; ok && v != nil {
					acc = v.(string)
				}
				line := fmt.Sprintf("- [%s] %s (confidence: %d%%)\n", issue["id"], issue["desc"], issue["score"])
				return map[string]any{"__accumulator": acc + line}, nil
			},
		},
	)
	if err != nil {
		t.Fatal(err)
	}

	results, err := d.Execute(context.Background())
	if err != nil {
		t.Fatal(err)
	}

	fmt.Println("\n  === Final Report ===")
	fmt.Print("  ", strings.ReplaceAll(results["report"].(string), "\n", "\n  "))
	fmt.Println()
}

// TestDemo_BoundedCycle demonstrates iterative refinement using back-edges.
// A "drafter" writes, a "critic" reviews, and a back-edge loops them up to 3 times.
func TestDemo_BoundedCycle(t *testing.T) {
	fmt.Println("\n=== Demo: Iterative Refinement (Bounded Cycle) ===")
	fmt.Println("Loop: drafter → critic →(back-edge, max=3)→ drafter")

	iteration := 0

	// drafter produces "draft_out", critic consumes it and produces "critic_out".
	// The back-edge critic→drafter carries the feedback via the store (no auto-wire conflict).
	d, err := dag.New(
		&dag.Node{
			ID:       "drafter",
			Produces: []string{"draft_out"},
			Run: func(ctx context.Context, in map[string]any) (map[string]any, error) {
				iteration++
				quality := iteration * 30
				fb := ""
				if v, ok := in["critic_out"]; ok && v != nil {
					fb = fmt.Sprintf(" (incorporating: %s)", v)
				}
				fmt.Printf("  [drafter]  Iteration %d: quality=%d%%%s\n", iteration, quality, fb)
				return map[string]any{
					"draft_out": fmt.Sprintf("v%d", iteration),
					"quality":   quality,
				}, nil
			},
		},
		&dag.Node{
			ID:       "critic",
			Consumes: []string{"draft_out", "quality"},
			Produces: []string{"critic_out"},
			Run: func(ctx context.Context, in map[string]any) (map[string]any, error) {
				draft := in["draft_out"].(string)
				quality := in["quality"].(int)
				if quality >= 80 {
					fmt.Printf("  [critic]   Draft %s approved! (quality=%d%%)\n", draft, quality)
				} else {
					fmt.Printf("  [critic]   Draft %s needs work (quality=%d%%), requesting revision\n", draft, quality)
				}
				return map[string]any{"critic_out": fmt.Sprintf("improve clarity in %s", draft)}, nil
			},
		},
	)
	if err != nil {
		t.Fatal(err)
	}

	// Back-edge: critic → drafter, max 3 iterations
	if err := d.AddBackEdge("critic", "drafter", 3); err != nil {
		t.Fatal(err)
	}

	results, err := d.Execute(context.Background())
	if err != nil {
		t.Fatal(err)
	}

	fmt.Printf("\n  Final: %d iterations, last feedback: %q\n\n", iteration, results["critic_out"])
}

// TestDemo_FallbackWithRace demonstrates KindFallback and KindRace.
func TestDemo_FallbackAndRace(t *testing.T) {
	fmt.Println("\n=== Demo: Fallback + Race ===")

	calls := 0

	d, err := dag.New(
		// Fallback: try fast model, fall back to reliable model
		&dag.Node{
			ID: "translator", Kind: dag.KindFallback,
			Produces: []string{"translation"},
			Config: &dag.NodeConfig{
				Fallbacks: []dag.RunFunc{
					func(ctx context.Context, in map[string]any) (map[string]any, error) {
						fmt.Println("  [fallback] Backup model succeeded")
						return map[string]any{"translation": "Bonjour le monde"}, nil
					},
				},
			},
			Run: func(ctx context.Context, in map[string]any) (map[string]any, error) {
				calls++
				if calls == 1 {
					fmt.Println("  [primary]  Fast model failed (rate limited)")
					return nil, fmt.Errorf("rate limited")
				}
				return map[string]any{"translation": "Bonjour"}, nil
			},
		},

		// Race: two summarizers compete, fastest wins
		&dag.Node{
			ID: "summarizer", Kind: dag.KindRace,
			Consumes: []string{"translation"},
			Produces: []string{"summary"},
			Config: &dag.NodeConfig{
				Competitors: []dag.RunFunc{
					func(ctx context.Context, in map[string]any) (map[string]any, error) {
						fmt.Println("  [racer-B]  Competitor finished first")
						return map[string]any{"summary": "Short summary from model B"}, nil
					},
				},
			},
			Run: func(ctx context.Context, in map[string]any) (map[string]any, error) {
				// Primary is slightly slower (simulated by being second goroutine)
				fmt.Println("  [racer-A]  Primary also finished")
				return map[string]any{"summary": "Summary from model A"}, nil
			},
		},
	)
	if err != nil {
		t.Fatal(err)
	}

	results, err := d.Execute(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	fmt.Printf("\n  Result: %q\n\n", results["summary"])
}

// TestDemo_BatchProcessing demonstrates KindBatch for chunked parallel work.
func TestDemo_BatchProcessing(t *testing.T) {
	fmt.Println("\n=== Demo: Batch Processing ===")
	fmt.Println("Processing 10 files in batches of 3")

	d, err := dag.New(
		&dag.Node{
			ID:       "lister",
			Produces: []string{"files"},
			Run: func(ctx context.Context, in map[string]any) (map[string]any, error) {
				files := make([]any, 10)
				for i := range files {
					files[i] = fmt.Sprintf("file_%d.go", i+1)
				}
				return map[string]any{"files": files}, nil
			},
		},
		&dag.Node{
			ID: "linter", Kind: dag.KindBatch,
			Consumes: []string{"files"},
			Produces: []string{"results"},
			Config:   &dag.NodeConfig{BatchSize: 3},
			Run: func(ctx context.Context, in map[string]any) (map[string]any, error) {
				chunk := in["files"].([]any)
				names := make([]string, len(chunk))
				for i, f := range chunk {
					names[i] = f.(string)
				}
				fmt.Printf("  [linter]   Batch: [%s]\n", strings.Join(names, ", "))
				results := make([]any, len(chunk))
				for i, f := range chunk {
					results[i] = fmt.Sprintf("%s: OK", f)
				}
				return map[string]any{"results": results}, nil
			},
		},
	)
	if err != nil {
		t.Fatal(err)
	}

	results, err := d.Execute(context.Background())
	if err != nil {
		t.Fatal(err)
	}

	r := results["results"].([]any)
	fmt.Printf("\n  Processed %d files in batches of 3\n\n", len(r))
}
