// pkg/dag/dag.go
package dag

import (
	"context"
	"fmt"
	"time"
)

// NodeKind distinguishes execution semantics.
type NodeKind string

const (
	KindTask   NodeKind = "task"
	KindRouter NodeKind = "router"
	KindMerge  NodeKind = "merge"
)

// Node is a unit of work in the DAG.
type Node struct {
	ID          string
	Kind        NodeKind
	Consumes    []string
	Produces    []string
	Run         func(ctx context.Context, inputs map[string]any) (map[string]any, error)
	MaxRetries  int
	Timeout     time.Duration
	Config      *NodeConfig
	Middlewares []Middleware
}

// DAG is a directed graph auto-wired by type-matching edges.
// Forward edges (MaxIterations=0) preserve acyclicity; back-edges (MaxIterations>0)
// allow bounded cycles for iterative patterns.
type DAG struct {
	nodes map[string]*Node
	edges map[EdgeKey]*Edge
	adj   map[string][]*Edge // outgoing edges (from → edges)
	rev   map[string][]*Edge // incoming edges (to → edges)
}

// New creates a DAG and auto-wires edges from Produces/Consumes type matching.
func New(nodes ...*Node) (*DAG, error) {
	d := &DAG{
		nodes: make(map[string]*Node, len(nodes)),
		edges: make(map[EdgeKey]*Edge),
		adj:   make(map[string][]*Edge, len(nodes)),
		rev:   make(map[string][]*Edge, len(nodes)),
	}

	for _, n := range nodes {
		if _, exists := d.nodes[n.ID]; exists {
			return nil, fmt.Errorf("dag: duplicate node ID %q", n.ID)
		}
		if n.Kind == "" {
			n.Kind = KindTask
		}
		d.nodes[n.ID] = n
		d.adj[n.ID] = nil
		d.rev[n.ID] = nil
	}

	producer := make(map[string]string, len(nodes))
	for _, n := range nodes {
		for _, p := range n.Produces {
			if existing, conflict := producer[p]; conflict {
				return nil, fmt.Errorf("dag: type %q produced by both %q and %q", p, existing, n.ID)
			}
			producer[p] = n.ID
		}
	}

	for _, n := range nodes {
		for _, c := range n.Consumes {
			if src, ok := producer[c]; ok && src != n.ID {
				d.addEdge(src, n.ID)
			}
		}
	}

	return d, nil
}

// AddEdge adds a manual directed edge from → to.
func (d *DAG) AddEdge(from, to string) error {
	if _, ok := d.nodes[from]; !ok {
		return fmt.Errorf("dag: unknown node %q", from)
	}
	if _, ok := d.nodes[to]; !ok {
		return fmt.Errorf("dag: unknown node %q", to)
	}
	// Cycle guard: if from is reachable from to, adding from→to would create a cycle
	if d.reachable(to, from) {
		return fmt.Errorf("dag: adding edge %q→%q would create a cycle", from, to)
	}
	d.addEdge(from, to)
	return nil
}

// RemoveEdge removes the edge from → to.
func (d *DAG) RemoveEdge(from, to string) error {
	if _, ok := d.nodes[from]; !ok {
		return fmt.Errorf("dag: unknown node %q", from)
	}
	if _, ok := d.nodes[to]; !ok {
		return fmt.Errorf("dag: unknown node %q", to)
	}
	delete(d.edges, EdgeKey{From: from, To: to})
	adj := d.adj[from]
	for i, e := range adj {
		if e.To == to {
			d.adj[from] = append(adj[:i], adj[i+1:]...)
			break
		}
	}
	rev := d.rev[to]
	for i, e := range rev {
		if e.From == from {
			d.rev[to] = append(rev[:i], rev[i+1:]...)
			break
		}
	}
	return nil
}

// AddBackEdge adds a back-edge that permits a bounded cycle.
// MaxIterations must be > 0.
func (d *DAG) AddBackEdge(from, to string, maxIter int) error {
	if _, ok := d.nodes[from]; !ok {
		return fmt.Errorf("dag: unknown node %q", from)
	}
	if _, ok := d.nodes[to]; !ok {
		return fmt.Errorf("dag: unknown node %q", to)
	}
	if maxIter <= 0 {
		return fmt.Errorf("dag: back-edge requires MaxIterations > 0, got %d", maxIter)
	}
	key := EdgeKey{From: from, To: to}
	if _, exists := d.edges[key]; exists {
		return fmt.Errorf("dag: edge %q→%q already exists", from, to)
	}
	e := &Edge{From: from, To: to, MaxIterations: maxIter}
	d.edges[key] = e
	d.adj[from] = append(d.adj[from], e)
	d.rev[to] = append(d.rev[to], e)
	return nil
}

// Edges returns all edges in the graph.
func (d *DAG) Edges() []*Edge {
	out := make([]*Edge, 0, len(d.edges))
	for _, e := range d.edges {
		out = append(out, e)
	}
	return out
}

// ForwardEdges returns all forward (non-back) outgoing edges from nodeID.
func (d *DAG) ForwardEdges(nodeID string) []*Edge {
	var out []*Edge
	for _, e := range d.adj[nodeID] {
		if !e.IsBackEdge() {
			out = append(out, e)
		}
	}
	return out
}

// HasBackEdges returns true if the graph contains any back-edges.
func (d *DAG) HasBackEdges() bool {
	for _, e := range d.edges {
		if e.IsBackEdge() {
			return true
		}
	}
	return false
}

// Downstream returns the node IDs that nodeID points to (all edge types).
func (d *DAG) Downstream(nodeID string) []string {
	edges := d.adj[nodeID]
	if len(edges) == 0 {
		return nil
	}
	out := make([]string, len(edges))
	for i, e := range edges {
		out[i] = e.To
	}
	return out
}

// Upstream returns the node IDs that point to nodeID (all edge types).
func (d *DAG) Upstream(nodeID string) []string {
	edges := d.rev[nodeID]
	if len(edges) == 0 {
		return nil
	}
	out := make([]string, len(edges))
	for i, e := range edges {
		out[i] = e.From
	}
	return out
}

// Nodes returns all node IDs.
func (d *DAG) Nodes() []string {
	ids := make([]string, 0, len(d.nodes))
	for id := range d.nodes {
		ids = append(ids, id)
	}
	return ids
}

// Node returns the node with the given ID, or nil.
func (d *DAG) Node(id string) *Node { return d.nodes[id] }

func (d *DAG) addEdge(from, to string) {
	key := EdgeKey{From: from, To: to}
	if _, exists := d.edges[key]; exists {
		return
	}
	e := &Edge{From: from, To: to, MaxIterations: 0}
	d.edges[key] = e
	d.adj[from] = append(d.adj[from], e)
	d.rev[to] = append(d.rev[to], e)
}

// reachable returns true if target is reachable from start via forward edges (BFS).
func (d *DAG) reachable(start, target string) bool {
	if start == target {
		return true
	}
	visited := make(map[string]bool)
	queue := []string{start}
	visited[start] = true
	for len(queue) > 0 {
		cur := queue[0]
		queue = queue[1:]
		for _, e := range d.adj[cur] {
			if e.IsBackEdge() {
				continue
			}
			if e.To == target {
				return true
			}
			if !visited[e.To] {
				visited[e.To] = true
				queue = append(queue, e.To)
			}
		}
	}
	return false
}
