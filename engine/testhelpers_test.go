package engine

import (
	"context"
	"time"

	"github.com/dpopsuev/origami/circuit"
	"github.com/dpopsuev/troupe/identity"
)

// --- Shared test helpers for engine/ test files ---

type stubNode struct {
	name     string
	element  identity.Element
	artifact circuit.Artifact
	err      error
}

func (n *stubNode) Name() string               { return n.name }
func (n *stubNode) Approach() identity.Element { return n.element }
func (n *stubNode) Process(_ context.Context, _ circuit.NodeContext) (circuit.Artifact, error) {
	return n.artifact, n.err
}

type stubArtifact struct {
	typ        string
	confidence float64
	raw        any
}

func (a *stubArtifact) Type() string        { return a.typ }
func (a *stubArtifact) Confidence() float64 { return a.confidence }
func (a *stubArtifact) Raw() any            { return a.raw }

type stubEdge struct {
	id, from, to string
	loop         bool
	result       *circuit.Transition
}

func (e *stubEdge) ID() string       { return e.id }
func (e *stubEdge) From() string     { return e.from }
func (e *stubEdge) To() string       { return e.to }
func (e *stubEdge) IsLoop() bool     { return e.loop }
func (e *stubEdge) IsShortcut() bool { return false }
func (e *stubEdge) Evaluate(_ circuit.Artifact, _ *circuit.WalkerState) *circuit.Transition {
	if e.result != nil {
		return e.result
	}
	return &circuit.Transition{NextNode: e.to}
}

type slowNode struct {
	name     string
	duration time.Duration
}

func (n *slowNode) Name() string               { return n.name }
func (n *slowNode) Approach() identity.Element { return "" }
func (n *slowNode) Process(ctx context.Context, _ circuit.NodeContext) (circuit.Artifact, error) {
	select {
	case <-time.After(n.duration):
		return &stubArtifact{typ: "slow", confidence: 1.0, raw: "done"}, nil
	case <-ctx.Done():
		return &stubArtifact{typ: "slow", confidence: 0, raw: "canceled"}, ctx.Err()
	}
}

type echoTransformer struct{}

func (t *echoTransformer) Name() string { return "echo" }
func (t *echoTransformer) Transform(_ context.Context, tc *InstrumentContext) (any, error) {
	return map[string]any{"echoed": tc.Input, "node": tc.NodeName}, nil
}

type stubWalker struct {
	identity identity.Archetype
	state    *circuit.WalkerState
	visited  []string
}

func (w *stubWalker) Identity() identity.Archetype       { return w.identity }
func (w *stubWalker) SetIdentity(id *identity.Archetype) { w.identity = *id }
func (w *stubWalker) State() *circuit.WalkerState        { return w.state }
func (w *stubWalker) Handle(_ context.Context, node circuit.Node, nc circuit.NodeContext) (circuit.Artifact, error) {
	w.visited = append(w.visited, node.Name())
	return node.Process(context.Background(), nc)
}

type testArtifact struct {
	typeName   string
	confidence float64
	raw        any
}

func (a *testArtifact) Type() string        { return a.typeName }
func (a *testArtifact) Confidence() float64 { return a.confidence }
func (a *testArtifact) Raw() any            { return a.raw }
