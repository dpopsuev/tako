package state

// Category: Processing & Support

import (
	"sync"

	"github.com/dpopsuev/origami/core"
)

// OutputCapture collects artifacts produced at each node during a walk.
// It implements WalkObserver and is safe for concurrent use during
// parallel fan-out walks.
type OutputCapture struct {
	mu        sync.RWMutex
	artifacts map[string]core.Artifact
}

// NewOutputCapture creates an OutputCapture ready for use.
func NewOutputCapture() *OutputCapture {
	return &OutputCapture{
		artifacts: make(map[string]core.Artifact),
	}
}

// NewCapture returns a WalkObserver that captures artifacts and an ArtifactCapture
// to read them after the walk. Use the observer with MultiObserver or run config.
func NewCapture() (core.WalkObserver, core.ArtifactCapture) {
	c := NewOutputCapture()
	return c, c
}

// OnEvent implements WalkObserver. It captures artifacts from node_exit events.
func (c *OutputCapture) OnEvent(e core.WalkEvent) {
	if e.Type != core.EventNodeExit || e.Node == "" {
		return
	}
	if e.Artifact == nil {
		return
	}
	c.mu.Lock()
	c.artifacts[e.Node] = e.Artifact
	c.mu.Unlock()
}

// Artifacts returns a copy of all captured node artifacts.
func (c *OutputCapture) Artifacts() map[string]core.Artifact {
	c.mu.RLock()
	defer c.mu.RUnlock()
	out := make(map[string]core.Artifact, len(c.artifacts))
	for k, v := range c.artifacts {
		out[k] = v
	}
	return out
}

// ArtifactAt returns the artifact for a specific node, if captured.
func (c *OutputCapture) ArtifactAt(node string) (core.Artifact, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	a, ok := c.artifacts[node]
	return a, ok
}

// Reset clears all captured artifacts.
func (c *OutputCapture) Reset() {
	c.mu.Lock()
	c.artifacts = make(map[string]core.Artifact)
	c.mu.Unlock()
}
