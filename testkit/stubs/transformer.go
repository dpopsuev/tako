// Package stubs provides reusable test doubles for the origami framework.
// All stubs are thread-safe, support error injection, and track calls.
package stubs

import (
	"context"
	"sync"

	"github.com/dpopsuev/origami/circuit"
	"github.com/dpopsuev/origami/engine"
)

// StubTransformer implements engine.Instrument with canned artifacts.
// Configure per-node responses via WithArtifact/SetError before use.
type StubTransformer struct {
	mu        sync.Mutex
	name      string
	artifacts map[string]circuit.Artifact
	errors    map[string]error
	calls     []string
}

// NewStubTransformer creates a transformer that returns canned artifacts
// for each node name. Pass artifacts as name/artifact pairs.
func NewStubTransformer(name string, artifacts map[string]circuit.Artifact) *StubTransformer {
	if artifacts == nil {
		artifacts = make(map[string]circuit.Artifact)
	}
	return &StubTransformer{
		name:      name,
		artifacts: artifacts,
		errors:    make(map[string]error),
	}
}

func (s *StubTransformer) Name() string { return s.name }

func (s *StubTransformer) Transform(_ context.Context, tc *engine.InstrumentContext) (any, error) {
	s.mu.Lock()
	s.calls = append(s.calls, tc.NodeName)
	err := s.errors[tc.NodeName]
	art := s.artifacts[tc.NodeName]
	s.mu.Unlock()

	if err != nil {
		return nil, err
	}
	if art != nil {
		return art, nil
	}
	return NewStubArtifact(s.name, tc.NodeName), nil
}

// SetError injects an error for a specific node. Transform will return
// this error when called for that node.
func (s *StubTransformer) SetError(node string, err error) {
	s.mu.Lock()
	s.errors[node] = err
	s.mu.Unlock()
}

// WithArtifact sets the canned artifact for a specific node.
func (s *StubTransformer) WithArtifact(node string, art circuit.Artifact) {
	s.mu.Lock()
	s.artifacts[node] = art
	s.mu.Unlock()
}

// Calls returns the ordered list of node names that Transform was called with.
func (s *StubTransformer) Calls() []string {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := make([]string, len(s.calls))
	copy(out, s.calls)
	return out
}

// CallCount returns how many times Transform was called.
func (s *StubTransformer) CallCount() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return len(s.calls)
}

// Reset clears call tracking and injected errors.
func (s *StubTransformer) Reset() {
	s.mu.Lock()
	s.calls = nil
	s.errors = make(map[string]error)
	s.mu.Unlock()
}
