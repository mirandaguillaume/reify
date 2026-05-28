package dag

import "sync"

// dataStore is a concurrency-safe key-value store for inter-node data passing.
type dataStore struct {
	mu   sync.RWMutex
	data map[string]any
}

func (s *dataStore) gather(consumes []string) map[string]any {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make(map[string]any, len(consumes))
	for _, k := range consumes {
		if v, ok := s.data[k]; ok {
			out[k] = v
		}
	}
	return out
}

func (s *dataStore) put(key string, val any) {
	s.mu.Lock()
	s.data[key] = val
	s.mu.Unlock()
}

func (s *dataStore) putAll(m map[string]any) {
	s.mu.Lock()
	for k, v := range m {
		s.data[k] = v
	}
	s.mu.Unlock()
}

// terminalOutputs returns values produced by nodes with no forward outgoing edges.
func (s *dataStore) terminalOutputs(d *DAG) map[string]any {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make(map[string]any)
	for id, n := range d.nodes {
		hasForward := false
		for _, e := range d.adj[id] {
			if !e.IsBackEdge() {
				hasForward = true
				break
			}
		}
		if !hasForward {
			for _, p := range n.Produces {
				if v, ok := s.data[p]; ok {
					out[p] = v
				}
			}
		}
	}
	return out
}
