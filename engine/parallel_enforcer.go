package engine

// Category: Execution

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/dpopsuev/tako/circuit"
)

// ArtifactStoreKey is the well-known key for a shared ArtifactStore
// in WalkerState.Context.
const ArtifactStoreKey = "__artifact_store"

// ArtifactStore is a thread-safe store of named artifacts.
type ArtifactStore struct {
	mu      sync.RWMutex
	outputs map[string]circuit.Artifact
}

// Set stores an artifact under the given node name.
func (s *ArtifactStore) Set(name string, a circuit.Artifact) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.outputs == nil {
		s.outputs = make(map[string]circuit.Artifact)
	}
	s.outputs[name] = a
}

// Get retrieves an artifact by node name, or nil if absent.
func (s *ArtifactStore) Get(name string) circuit.Artifact {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.outputs[name]
}

// All returns a snapshot of all stored artifacts.
func (s *ArtifactStore) All() map[string]circuit.Artifact {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make(map[string]circuit.Artifact, len(s.outputs))
	for k, v := range s.outputs {
		out[k] = v
	}
	return out
}

// Len returns the number of stored artifacts.
func (s *ArtifactStore) Len() int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return len(s.outputs)
}

// artifactCaptureObserver captures EventNodeExit artifacts into an ArtifactStore.
type artifactCaptureObserver struct {
	store         *ArtifactStore
	observedNodes map[string]bool
	inner         circuit.WalkObserver
}

func (o *artifactCaptureObserver) OnEvent(e *circuit.WalkEvent) {
	if e.Type == circuit.EventNodeExit && e.Artifact != nil {
		if len(o.observedNodes) == 0 || o.observedNodes[e.Node] {
			o.store.Set(e.Node, e.Artifact)
		}
	}
	if o.inner != nil {
		o.inner.OnEvent(e)
	}
}

// ParallelEnforcerConfig configures a parallel enforcement circuit.
type ParallelEnforcerConfig struct {
	EnforcerDef   *circuit.CircuitDef
	Registries    *GraphRegistries
	ObservedNodes []string
	CheckInterval time.Duration
	Router        *FindingRouter
	DrainTimeout  time.Duration
}

// RunWithEnforcer runs a work circuit with a parallel enforcer circuit.
func RunWithEnforcer(
	ctx context.Context,
	workDef *circuit.CircuitDef,
	workReg *GraphRegistries,
	enforcerCfg *ParallelEnforcerConfig,
) ([]circuit.Finding, error) {

	router := enforcerCfg.Router
	if router == nil {
		router = NewFindingRouter(nil, FindingHandlers{})
	}

	store := &ArtifactStore{}

	observed := make(map[string]bool, len(enforcerCfg.ObservedNodes))
	for _, n := range enforcerCfg.ObservedNodes {
		observed[n] = true
	}

	workRunner, err := NewRunnerWith(workDef, workReg)
	if err != nil {
		return nil, fmt.Errorf("build work runner: %w", err)
	}
	if dg, ok := workRunner.Graph.(*DefaultGraph); ok {
		dg.observer = &artifactCaptureObserver{store: store, observedNodes: observed}
	}

	enforcerReg := enforcerCfg.Registries
	enforcerRunner, err := NewRunnerWith(enforcerCfg.EnforcerDef, enforcerReg)
	if err != nil {
		return nil, fmt.Errorf("build enforcer runner: %w", err)
	}

	enforcerCtx, cancelEnforcer := context.WithCancel(ctx)
	defer cancelEnforcer()

	workWalker := circuit.NewProcessWalker("work")
	workWalker.State().Context[circuit.FindingCollectorKey] = router

	enforcerWalker := circuit.NewProcessWalker("enforcer")
	enforcerWalker.State().Context[circuit.FindingCollectorKey] = router
	enforcerWalker.State().Context[ArtifactStoreKey] = store

	var workErr error
	workDone := make(chan struct{})
	go func() {
		defer close(workDone)
		workErr = workRunner.Walk(ctx, workWalker, string(workDef.Start))
	}()

	var enforcerErr error
	enforcerDone := make(chan struct{})
	go func() {
		defer close(enforcerDone)
		enforcerErr = enforcerRunner.Walk(enforcerCtx, enforcerWalker, string(enforcerCfg.EnforcerDef.Start))
	}()

	<-workDone

	drainTimeout := enforcerCfg.DrainTimeout
	if drainTimeout == 0 {
		drainTimeout = 500 * time.Millisecond
	}
	select {
	case <-enforcerDone:
	case <-time.After(drainTimeout):
		cancelEnforcer()
		<-enforcerDone
	}

	_ = enforcerErr

	return router.Findings(), workErr
}
