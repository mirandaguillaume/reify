package dag

import (
	"context"
	"time"
)

// Collection node kinds operate on []any slices.
const (
	KindMap     NodeKind = "map"      // []any → parallel f(item) → []any
	KindFilter  NodeKind = "filter"   // []any → subset by __keep predicate
	KindFlatMap NodeKind = "flat-map" // item → []any (expand per item)
	KindReduce  NodeKind = "reduce"   // []any → scalar via fold
	KindBatch   NodeKind = "batch"    // []any → chunks of N → parallel per-chunk
	KindZip     NodeKind = "zip"      // ([]any, []any) → []any of [a,b] pairs
)

// Control-flow node kinds.
const (
	KindGate     NodeKind = "gate"     // halt pipeline if __halt=true
	KindFallback NodeKind = "fallback" // try primary, then backups in order
	KindRace     NodeKind = "race"     // run N nodes in parallel, first wins
	KindCache    NodeKind = "cache"    // memoize by input hash
)

// RunFunc is the function signature for node execution.
type RunFunc func(ctx context.Context, inputs map[string]any) (map[string]any, error)

// NodeConfig holds kind-specific settings for a node.
type NodeConfig struct {
	BatchSize    int           // KindBatch: items per chunk
	RaceTimeout  time.Duration // KindRace: max wait for competitors
	Fallbacks    []RunFunc     // KindFallback: backup functions tried in order
	Competitors  []RunFunc     // KindRace: competing functions run in parallel
	CacheKeyFunc func(inputs map[string]any) string
}
