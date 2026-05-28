package dag

// EdgeKey uniquely identifies a directed edge.
type EdgeKey struct {
	From, To string
}

// Edge is a directed connection between two nodes.
type Edge struct {
	From          string
	To            string
	MaxIterations int // 0 = forward-edge (cycle-preventing); >0 = back-edge (bounded cycle)
}

// IsBackEdge returns true if this edge permits a cycle.
func (e *Edge) IsBackEdge() bool {
	return e.MaxIterations > 0
}
