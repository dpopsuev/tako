package memory

import "errors"

var (
	ErrNodeNotFound = errors.New("memory: node not found")
	ErrEdgeNotFound = errors.New("memory: edge not found")
)

// WalkFunc is called for each node during a mesh walk.
type WalkFunc func(node KnowledgeNode, depth int) bool

// Mesh is the living knowledge graph with metabolism (fusion, fission, decay).
type Mesh interface {
	AddNode(node KnowledgeNode) error
	AddEdge(edge Edge) error
	Node(id string) (KnowledgeNode, error)
	Neighbors(id string) ([]KnowledgeNode, error)
	Walk(startID string, fn WalkFunc) error
	Nodes() []KnowledgeNode
	Edges() []Edge
}
