// pkg/dag/validate.go
package dag

import "fmt"

// Topology names a structural pattern that can be validated against the actual graph shape.
type Topology string

const (
	TopoPipeline     Topology = "pipeline"
	TopoFanOut       Topology = "fan-out"
	TopoFanIn        Topology = "fan-in"
	TopoDiamond      Topology = "diamond"
	TopoWideParallel Topology = "wide-parallel"
	TopoHourglass    Topology = "hourglass"
)

// HasCycle returns true if the forward-edge subgraph contains a directed cycle.
// Back-edges are excluded — they intentionally create bounded cycles.
// Uses DFS with three-colour marking: 0=unvisited, 1=in-stack, 2=done.
func (d *DAG) HasCycle() bool {
	colour := make(map[string]int, len(d.nodes))
	var dfs func(id string) bool
	dfs = func(id string) bool {
		colour[id] = 1
		for _, e := range d.adj[id] {
			if e.IsBackEdge() {
				continue
			}
			next := e.To
			if colour[next] == 1 {
				return true
			}
			if colour[next] == 0 && dfs(next) {
				return true
			}
		}
		colour[id] = 2
		return false
	}
	for id := range d.nodes {
		if colour[id] == 0 {
			if dfs(id) {
				return true
			}
		}
	}
	return false
}

// ValidateTopology checks that the declared topology hint matches the actual graph shape.
// Returns nil if the shape matches, or a descriptive error.
func (d *DAG) ValidateTopology(declared Topology) error {
	layers := d.Layers()
	n := len(d.nodes)

	switch declared {
	case TopoPipeline:
		for _, layer := range layers {
			if len(layer) != 1 {
				return fmt.Errorf("topology mismatch: pipeline requires all layers to have 1 node, got layer with %d", len(layer))
			}
		}
		for id := range d.nodes {
			if len(d.adj[id]) > 1 {
				return fmt.Errorf("topology mismatch: pipeline requires max out-degree 1, node %q has %d", id, len(d.adj[id]))
			}
		}

	case TopoFanOut:
		sources := sourcesFromLayers(layers)
		if len(sources) != 1 {
			return fmt.Errorf("topology mismatch: fan-out requires exactly 1 source, got %d", len(sources))
		}
		if len(d.adj[sources[0]]) <= 1 {
			return fmt.Errorf("topology mismatch: fan-out requires source to have >1 downstream, got %d", len(d.adj[sources[0]]))
		}

	case TopoFanIn:
		sinks := d.sinks()
		if len(sinks) != 1 {
			return fmt.Errorf("topology mismatch: fan-in requires exactly 1 sink, got %d", len(sinks))
		}
		if len(d.rev[sinks[0]]) <= 1 {
			return fmt.Errorf("topology mismatch: fan-in requires sink to have >1 upstream, got %d", len(d.rev[sinks[0]]))
		}

	case TopoDiamond:
		if n < 4 {
			return fmt.Errorf("topology mismatch: diamond requires ≥4 nodes, got %d", n)
		}
		sources := sourcesFromLayers(layers)
		sinks := d.sinks()
		if len(sources) != 1 || len(sinks) != 1 {
			return fmt.Errorf("topology mismatch: diamond requires 1 source and 1 sink, got %d sources and %d sinks", len(sources), len(sinks))
		}
		hasWideMiddle := false
		for i := 1; i < len(layers)-1; i++ {
			if len(layers[i]) > 1 {
				hasWideMiddle = true
				break
			}
		}
		if !hasWideMiddle {
			return fmt.Errorf("topology mismatch: diamond requires a wide middle layer")
		}

	case TopoWideParallel:
		if len(layers) != 1 {
			return fmt.Errorf("topology mismatch: wide-parallel requires 1 layer, got %d", len(layers))
		}
		for id := range d.nodes {
			if len(d.adj[id]) > 0 || len(d.rev[id]) > 0 {
				return fmt.Errorf("topology mismatch: wide-parallel requires no edges, node %q has edges", id)
			}
		}

	case TopoHourglass:
		if len(layers) < 3 {
			return fmt.Errorf("topology mismatch: hourglass requires ≥3 layers, got %d", len(layers))
		}
		if len(layers[0]) <= 1 {
			return fmt.Errorf("topology mismatch: hourglass requires wide first layer, got %d nodes", len(layers[0]))
		}
		if len(layers[len(layers)-1]) <= 1 {
			return fmt.Errorf("topology mismatch: hourglass requires wide last layer, got %d nodes", len(layers[len(layers)-1]))
		}
		hasBottleneck := false
		for i := 1; i < len(layers)-1; i++ {
			if len(layers[i]) == 1 {
				hasBottleneck = true
				break
			}
		}
		if !hasBottleneck {
			return fmt.Errorf("topology mismatch: hourglass requires a single-node bottleneck layer")
		}
	}

	return nil
}

// CycleInfo describes a bounded cycle formed by a back-edge.
type CycleInfo struct {
	BackEdge      *Edge    // the back-edge that closes the cycle
	Path          []string // forward-edge path from BackEdge.To → BackEdge.From
	MaxIterations int      // bound from the back-edge
}

// FindCycles returns all bounded cycles in the graph.
// Each cycle is identified by a back-edge plus the forward path it closes.
func (d *DAG) FindCycles() []CycleInfo {
	var cycles []CycleInfo
	for _, e := range d.edges {
		if !e.IsBackEdge() {
			continue
		}
		// Find forward path from e.To → e.From using BFS
		path := d.forwardPath(e.To, e.From)
		if path != nil {
			cycles = append(cycles, CycleInfo{
				BackEdge:      e,
				Path:          path,
				MaxIterations: e.MaxIterations,
			})
		}
	}
	return cycles
}

// ValidateBoundedCycles checks that:
//  1. The forward-edge subgraph is acyclic
//  2. Every back-edge closes a real cycle (forward path exists from To → From)
func (d *DAG) ValidateBoundedCycles() error {
	if d.HasCycle() {
		return fmt.Errorf("dag: forward-edge subgraph contains a cycle")
	}
	for _, e := range d.edges {
		if !e.IsBackEdge() {
			continue
		}
		path := d.forwardPath(e.To, e.From)
		if path == nil {
			return fmt.Errorf("dag: back-edge %q→%q does not close a cycle (no forward path from %q to %q)", e.From, e.To, e.To, e.From)
		}
	}
	return nil
}

// forwardPath returns the forward-edge path from start to target, or nil if unreachable.
func (d *DAG) forwardPath(start, target string) []string {
	if start == target {
		return []string{start}
	}
	parent := make(map[string]string)
	visited := map[string]bool{start: true}
	queue := []string{start}
	for len(queue) > 0 {
		cur := queue[0]
		queue = queue[1:]
		for _, e := range d.adj[cur] {
			if e.IsBackEdge() || visited[e.To] {
				continue
			}
			parent[e.To] = cur
			if e.To == target {
				// Reconstruct path
				path := []string{target}
				for p := target; p != start; {
					p = parent[p]
					path = append(path, p)
				}
				// Reverse
				for i, j := 0, len(path)-1; i < j; i, j = i+1, j-1 {
					path[i], path[j] = path[j], path[i]
				}
				return path
			}
			visited[e.To] = true
			queue = append(queue, e.To)
		}
	}
	return nil
}

func sourcesFromLayers(layers [][]string) []string {
	if len(layers) == 0 {
		return nil
	}
	return layers[0]
}

func (d *DAG) sinks() []string {
	sinks := make([]string, 0)
	for _, id := range sortedKeys(d.nodes) {
		if len(d.adj[id]) == 0 {
			sinks = append(sinks, id)
		}
	}
	return sinks
}
