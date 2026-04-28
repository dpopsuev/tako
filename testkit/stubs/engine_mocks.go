// Package stubs provides mock implementations of engine contracts
// for testing without infrastructure dependencies.
package stubs

import (
	"context"

	"github.com/dpopsuev/tako/circuit"
	"github.com/dpopsuev/tako/engine"
)

// MockGraph is a configurable Graph for testing.
type MockGraph struct {
	NameVal  string
	NodeList []circuit.Node
	EdgeList []circuit.Edge
	ZoneList []engine.Zone
	WalkErr  error
	WalkFn   func(ctx context.Context, walker circuit.Walker, startNode string) error
}

func (g *MockGraph) Name() string          { return g.NameVal }
func (g *MockGraph) Nodes() []circuit.Node { return g.NodeList }
func (g *MockGraph) Edges() []circuit.Edge { return g.EdgeList }
func (g *MockGraph) Zones() []engine.Zone  { return g.ZoneList }

func (g *MockGraph) NodeByName(name string) (circuit.Node, bool) {
	for _, n := range g.NodeList {
		if n.Name() == name {
			return n, true
		}
	}
	return nil, false
}

func (g *MockGraph) EdgesFrom(nodeName string) []circuit.Edge {
	var out []circuit.Edge
	for _, e := range g.EdgeList {
		if e.From() == nodeName {
			out = append(out, e)
		}
	}
	return out
}

func (g *MockGraph) Walk(ctx context.Context, walker circuit.Walker, startNode string) error {
	if g.WalkFn != nil {
		return g.WalkFn(ctx, walker, startNode)
	}
	return g.WalkErr
}

// MockBatchWalker wraps BatchWalk for testability.
type MockBatchWalker struct {
	Results []engine.BatchWalkResult
}

func (m *MockBatchWalker) BatchWalk(_ context.Context, _ engine.BatchWalkConfig) []engine.BatchWalkResult {
	return m.Results
}

// MockTuner wraps TuneAll for testability.
type MockTuner struct {
	Err error
}

func (m *MockTuner) TuneAll(_ context.Context, _ engine.ManifestRegistry, _ string) error {
	return m.Err
}

// Compile-time checks.
var _ engine.Graph = (*MockGraph)(nil)
