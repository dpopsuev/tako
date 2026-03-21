package stubs

import (
	"context"
	"sync"

	framework "github.com/dpopsuev/origami"
)

// StubWalker implements framework.Walker with canned artifacts per node.
// Thread-safe, supports error injection and visit tracking.
type StubWalker struct {
	mu        sync.Mutex
	id        string
	artifacts map[string]framework.Artifact
	err       error
	visited   []string
	identity  framework.AgentIdentity
	state     *framework.WalkerState
}

// NewStubWalker creates a walker that returns canned artifacts per node name.
func NewStubWalker(id string, artifacts map[string]framework.Artifact) *StubWalker {
	if artifacts == nil {
		artifacts = make(map[string]framework.Artifact)
	}
	return &StubWalker{
		id:        id,
		artifacts: artifacts,
		identity:  framework.AgentIdentity{PersonaName: id},
		state:     framework.NewWalkerState(id),
	}
}

// Handle returns the canned artifact for the given node, or an error if SetError was called.
func (w *StubWalker) Handle(_ context.Context, node framework.Node, _ framework.NodeContext) (framework.Artifact, error) {
	w.mu.Lock()
	defer w.mu.Unlock()

	name := node.Name()
	w.visited = append(w.visited, name)

	if w.err != nil {
		return nil, w.err
	}

	if art, ok := w.artifacts[name]; ok {
		return art, nil
	}
	return NewStubArtifact(w.id, name), nil
}

// Identity returns the walker's AgentIdentity.
func (w *StubWalker) Identity() framework.AgentIdentity {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.identity
}

// SetIdentity sets the walker's AgentIdentity.
func (w *StubWalker) SetIdentity(id framework.AgentIdentity) {
	w.mu.Lock()
	w.identity = id
	w.mu.Unlock()
}

// State returns the walker's state.
func (w *StubWalker) State() *framework.WalkerState {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.state
}

// SetError injects a global error. Handle will return this error for all nodes.
func (w *StubWalker) SetError(err error) {
	w.mu.Lock()
	w.err = err
	w.mu.Unlock()
}

// Visited returns a copy of all visited node names in order.
func (w *StubWalker) Visited() []string {
	w.mu.Lock()
	defer w.mu.Unlock()
	out := make([]string, len(w.visited))
	copy(out, w.visited)
	return out
}

// WithArtifact sets the canned artifact for a specific node.
func (w *StubWalker) WithArtifact(node string, art framework.Artifact) {
	w.mu.Lock()
	w.artifacts[node] = art
	w.mu.Unlock()
}

// Reset clears visit tracking and injected errors.
func (w *StubWalker) Reset() {
	w.mu.Lock()
	w.visited = nil
	w.err = nil
	w.mu.Unlock()
}

