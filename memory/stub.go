package memory

import "sync"

// StubMesh is an in-memory mesh — append-only, no fusion/fission/decay.
type StubMesh struct {
	mu    sync.RWMutex
	nodes map[string]KnowledgeNode
	edges []Edge
}

var _ Mesh = (*StubMesh)(nil)

func NewStubMesh() *StubMesh {
	return &StubMesh{nodes: make(map[string]KnowledgeNode)}
}

func (m *StubMesh) AddNode(node KnowledgeNode) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.nodes[node.ID] = node
	return nil
}

func (m *StubMesh) AddEdge(edge Edge) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.edges = append(m.edges, edge)
	return nil
}

func (m *StubMesh) Node(id string) (KnowledgeNode, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	n, ok := m.nodes[id]
	if !ok {
		return KnowledgeNode{}, ErrNodeNotFound
	}
	return n, nil
}

func (m *StubMesh) Neighbors(id string) ([]KnowledgeNode, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	if _, ok := m.nodes[id]; !ok {
		return nil, ErrNodeNotFound
	}
	var out []KnowledgeNode
	for _, e := range m.edges {
		if e.From == id {
			if n, ok := m.nodes[e.To]; ok {
				out = append(out, n)
			}
		}
	}
	return out, nil
}

func (m *StubMesh) Walk(startID string, fn WalkFunc) error {
	m.mu.RLock()
	defer m.mu.RUnlock()
	n, ok := m.nodes[startID]
	if !ok {
		return ErrNodeNotFound
	}
	fn(n, 0)
	return nil
}

func (m *StubMesh) Nodes() []KnowledgeNode {
	m.mu.RLock()
	defer m.mu.RUnlock()
	out := make([]KnowledgeNode, 0, len(m.nodes))
	for _, n := range m.nodes {
		out = append(out, n)
	}
	return out
}

func (m *StubMesh) Edges() []Edge {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return append([]Edge(nil), m.edges...)
}
