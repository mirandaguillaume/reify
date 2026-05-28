// pkg/dag/layers.go
package dag

import "sort"

// sortedKeys returns the keys of a map[string]*Node in alphabetical order.
func sortedKeys(m map[string]*Node) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

// Layers returns the topological layer decomposition using Kahn's algorithm
// with longest-path distance assignment.
//
// Layer 0 contains all source nodes (in-degree 0).
// Layer i contains nodes whose longest path from any source is exactly i hops.
// All layers are non-empty. Nodes within each layer are sorted alphabetically
// for deterministic output.
func (d *DAG) Layers() [][]string {
	if len(d.nodes) == 0 {
		return nil
	}

	// Compute in-degree for all nodes (forward edges only)
	inDegree := make(map[string]int, len(d.nodes))
	for id := range d.nodes {
		inDegree[id] = 0
	}
	for _, edges := range d.adj {
		for _, e := range edges {
			if !e.IsBackEdge() {
				inDegree[e.To]++
			}
		}
	}

	// dist[id] = longest path distance from any source node
	dist := make(map[string]int, len(d.nodes))
	for id := range d.nodes {
		dist[id] = -1
	}

	// Seed: source nodes (in-degree 0) start at distance 0
	queue := make([]string, 0, len(d.nodes))
	for _, id := range sortedKeys(d.nodes) {
		if inDegree[id] == 0 {
			dist[id] = 0
			queue = append(queue, id)
		}
	}

	// Kahn traversal with longest-path update (forward edges only)
	for len(queue) > 0 {
		cur := queue[0]
		queue = queue[1:]
		for _, e := range d.adj[cur] {
			if e.IsBackEdge() {
				continue
			}
			next := e.To
			if dist[cur]+1 > dist[next] {
				dist[next] = dist[cur] + 1
			}
			inDegree[next]--
			if inDegree[next] == 0 {
				queue = append(queue, next)
			}
		}
	}

	// Find max distance
	maxDist := 0
	for _, v := range dist {
		if v > maxDist {
			maxDist = v
		}
	}

	// Bucket nodes by distance
	layers := make([][]string, maxDist+1)
	for _, id := range sortedKeys(d.nodes) {
		if dist[id] >= 0 {
			layers[dist[id]] = append(layers[dist[id]], id)
		}
	}

	// Remove trailing empty layers (safety)
	for len(layers) > 0 && len(layers[len(layers)-1]) == 0 {
		layers = layers[:len(layers)-1]
	}

	return layers
}
