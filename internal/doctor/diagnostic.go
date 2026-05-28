package doctor

import (
	"context"
	"errors"
	"fmt"
	"io"
	"runtime/debug"
	"sort"
	"strings"
	"time"

	"github.com/mirandaguillaume/reify/pkg/dag"
)

// NodeError captures rich diagnostic information when a DAG node fails.
// It satisfies the error interface and wraps the underlying error for
// compatibility with errors.Is / errors.As chains.
type NodeError struct {
	NodeID    string   // ID of the node that failed
	InputKeys []string // sorted keys present in the node's input map
	Err       error    // the final (last-attempt) error
	Attempts  int      // total number of execution attempts made
	AllErrors []error  // per-attempt errors; populated when retry is configured
}

// Error returns an actionable message including the node ID, failure reason,
// input keys, and all failure reasons when more than one attempt was made.
//
// Example (single failure):
//
//	node "analyzer" failed: LLM timeout. Input: [detected_format, provider, registry]
//
// Example (retry exhausted, 3 total attempts):
//
//	node "analyzer" failed (3 attempts): [1] conn refused [2] rate limited [3] LLM timeout. Input: [detected_format, provider, registry]
func (e *NodeError) Error() string {
	var sb strings.Builder
	if e.Attempts > 1 && len(e.AllErrors) > 1 {
		fmt.Fprintf(&sb, "node %q failed (%d attempts):", e.NodeID, e.Attempts)
		for i, err := range e.AllErrors {
			fmt.Fprintf(&sb, " [%d] %s", i+1, err)
		}
	} else {
		fmt.Fprintf(&sb, "node %q failed: %s", e.NodeID, e.Err)
	}
	if len(e.InputKeys) > 0 {
		fmt.Fprintf(&sb, ". Input: [%s]", strings.Join(e.InputKeys, ", "))
	}
	return sb.String()
}

// Unwrap allows errors.Is / errors.As to inspect the underlying error.
func (e *NodeError) Unwrap() error { return e.Err }

// wrapWithPanic wraps a node's Run function to recover from panics, converting
// them into NodeError values so a single panicking node cannot crash the whole DAG.
// Nodes with nil Run are returned unchanged.
func wrapWithPanic(n *dag.Node) *dag.Node {
	if n.Run == nil {
		return n
	}
	nodeID := n.ID
	original := n.Run
	n.Run = func(ctx context.Context, inputs map[string]any) (out map[string]any, err error) {
		defer func() {
			if r := recover(); r != nil {
				panicErr := fmt.Errorf("panic: %v\nstack:\n%s", r, debug.Stack())
				err = &NodeError{
					NodeID:    nodeID,
					InputKeys: sortedKeys(inputs),
					Err:       panicErr,
					Attempts:  1,
					AllErrors: []error{panicErr},
				}
			}
		}()
		return original(ctx, inputs)
	}
	return n
}

// wrapWithRetry wraps a node's Run function with a retry loop that collects all
// attempt errors into a NodeError on exhaustion.
//
// maxAttempts is the total number of execution attempts (including the first).
// For example, maxAttempts=3 means 1 initial attempt + 2 retries.
//
// The node's MaxRetries field is reset to 0 to prevent pkg/dag from also retrying
// the already-retrying Run function (which would cause duplicate retries).
//
// Nodes with nil Run or maxAttempts ≤ 1 are returned unchanged.
func wrapWithRetry(n *dag.Node, maxAttempts int) *dag.Node {
	if n.Run == nil || maxAttempts <= 1 {
		return n
	}
	n.MaxRetries = 0 // prevent pkg/dag from double-retrying this closure
	nodeID := n.ID
	original := n.Run
	n.Run = func(ctx context.Context, inputs map[string]any) (map[string]any, error) {
		var allErrors []error
		for attempt := 0; attempt < maxAttempts; attempt++ {
			if ctx.Err() != nil {
				break
			}
			out, err := original(ctx, inputs)
			if err == nil {
				return out, nil
			}
			// Skip further retries for panic-originated errors (deterministic failures).
			var ne *NodeError
			if errors.As(err, &ne) && strings.HasPrefix(ne.Err.Error(), "panic:") {
				allErrors = append(allErrors, err)
				break
			}
			allErrors = append(allErrors, err)
		}
		// Guard: context cancelled before any attempt could run.
		if len(allErrors) == 0 {
			allErrors = append(allErrors, ctx.Err())
		}
		lastErr := allErrors[len(allErrors)-1]
		return nil, &NodeError{
			NodeID:    nodeID,
			InputKeys: sortedKeys(inputs),
			Err:       lastErr,
			Attempts:  len(allErrors),
			AllErrors: allErrors,
		}
	}
	return n
}

// wrapWithDebug instruments a node's Run function to emit structured diagnostic
// lines to w on node start, LLM call detection, completion, and failure.
// When w is nil the node is returned unchanged (debug disabled).
//
// For nodes that consume "provider" (LLM nodes), an extra line reports that an
// LLM call is in progress with the available input context keys.
func wrapWithDebug(n *dag.Node, w io.Writer) *dag.Node {
	if w == nil || n.Run == nil {
		return n
	}
	nodeID := n.ID
	isLLMNode := false
	for _, c := range n.Consumes {
		if c == "provider" {
			isLLMNode = true
			break
		}
	}
	original := n.Run
	n.Run = func(ctx context.Context, inputs map[string]any) (map[string]any, error) {
		start := time.Now()
		keys := sortedKeys(inputs)
		fmt.Fprintf(w, "[DEBUG] Node %q starting, input keys: [%s]\n",
			nodeID, strings.Join(keys, ", "))
		if isLLMNode {
			fmt.Fprintf(w, "[DEBUG] Node %q: LLM analysis call, context keys: [%s]\n",
				nodeID, strings.Join(keys, ", "))
		}

		out, err := original(ctx, inputs)
		dur := time.Since(start).Round(time.Millisecond)

		if err != nil {
			fmt.Fprintf(w, "[DEBUG] Node %q failed after %s: %v\n", nodeID, dur, err)
			return nil, err
		}
		fmt.Fprintf(w, "[DEBUG] Node %q completed in %s, output keys: [%s]\n",
			nodeID, dur, strings.Join(sortedKeys(out), ", "))
		return out, nil
	}
	return n
}

// sortedKeys returns the sorted keys of a map[string]any.
func sortedKeys(m map[string]any) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}
