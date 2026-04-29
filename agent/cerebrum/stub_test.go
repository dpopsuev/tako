package cerebrum

import (
	"context"
	"time"

	"github.com/dpopsuev/tako/memory"
)

type stubCompleter struct {
	response string
	err      error
}

func (s *stubCompleter) Complete(_ context.Context, _ string) (string, error) {
	return s.response, s.err
}

type stubMesh struct {
	nodes []string
}

func (s *stubMesh) AddNode(_ memory.KnowledgeNode) error             { return nil }
func (s *stubMesh) AddEdge(_ memory.Edge) error                      { return nil }
func (s *stubMesh) Node(_ string) (memory.KnowledgeNode, error)      { return memory.KnowledgeNode{}, nil }
func (s *stubMesh) Neighbors(_ string) ([]memory.KnowledgeNode, error) { return nil, nil }
func (s *stubMesh) Walk(_ string, _ memory.WalkFunc) error           { return nil }
func (s *stubMesh) Edges() []memory.Edge                             { return nil }

func (s *stubMesh) Nodes() []memory.KnowledgeNode {
	out := make([]memory.KnowledgeNode, len(s.nodes))
	for i, content := range s.nodes {
		out[i] = memory.KnowledgeNode{
			ID:        content,
			Content:   content,
			Tier:      memory.Knowledge,
			CreatedAt: time.Now(),
		}
	}
	return out
}
