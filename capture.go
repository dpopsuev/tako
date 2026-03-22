package framework

// Category: Processing & Support — aliases to core/ (interface) and state/ (implementation).

import (
	"github.com/dpopsuev/origami/core"
	"github.com/dpopsuev/origami/engine"
	"github.com/dpopsuev/origami/state"
)

// ArtifactCapture provides access to artifacts captured during a walk.
// Obtain one via NewCapture() and use the returned WalkObserver during the walk.
type ArtifactCapture = core.ArtifactCapture

// outputCapture collects artifacts produced at each node during a walk.
type outputCapture = state.OutputCapture

// newOutputCapture creates an outputCapture ready for use.
func newOutputCapture() *outputCapture { return state.NewOutputCapture() }

// NewCapture returns a WalkObserver that captures artifacts and an ArtifactCapture
// to read them after the walk. Use the observer with MultiObserver or run config.
func NewCapture() (WalkObserver, ArtifactCapture) {
	return state.NewCapture()
}

// withOutputCapture attaches an outputCapture as a walk observer.
// If another observer is already set, both are composed via MultiObserver.
func withOutputCapture(capture *outputCapture) RunOption {
	return engine.WithOutputCapture(capture)
}
