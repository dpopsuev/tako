package kami

import (
	"fmt"
	"sync"

	"github.com/dpopsuev/origami/circuit"
)

// DebugState represents the execution state of a debugged circuit.
type DebugState int

const (
	StateRunning DebugState = iota
	StatePaused
)

func (s DebugState) String() string {
	switch s {
	case StateRunning:
		return "running"
	case StatePaused:
		return "paused"
	default:
		return "unknown"
	}
}

// CircuitSnapshot is the inspectable state of a circuit at a point in time.
type CircuitSnapshot struct {
	State       string            `json:"state"`
	CurrentNode string            `json:"current_node,omitempty"`
	Breakpoints []string          `json:"breakpoints"`
	NodesVisited []string         `json:"nodes_visited"`
	Artifacts   map[string]string `json:"artifacts,omitempty"`
}

// Assertion is a configurable invariant check that runs after each node.
type Assertion struct {
	Name      string
	Predicate func(snapshot CircuitSnapshot) error
}

// DebugController provides breakpoint management, execution control,
// and circuit inspection. It wraps an EventBridge and intercepts
// WalkEvents to implement pause-at-breakpoint semantics.
type DebugController struct {
	mu          sync.Mutex
	state       DebugState
	breakpoints map[string]bool
	currentNode string
	visited     []string
	artifacts   map[string]string
	assertions  []Assertion

	// gate is used to block the walk goroutine when paused
	gate chan struct{}

	bridge *EventBridge
}

// NewDebugController creates a debug controller attached to a bridge.
func NewDebugController(bridge *EventBridge) *DebugController {
	return &DebugController{
		state:       StateRunning,
		breakpoints: make(map[string]bool),
		artifacts:   make(map[string]string),
		gate:        make(chan struct{}, 1),
		bridge:      bridge,
	}
}

// SetBreakpoint enables a breakpoint on a node.
func (d *DebugController) SetBreakpoint(node string) {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.breakpoints[node] = true
}

// ClearBreakpoint removes a breakpoint from a node.
func (d *DebugController) ClearBreakpoint(node string) {
	d.mu.Lock()
	defer d.mu.Unlock()
	delete(d.breakpoints, node)
}

// ListBreakpoints returns all active breakpoint node names.
func (d *DebugController) ListBreakpoints() []string {
	d.mu.Lock()
	defer d.mu.Unlock()
	out := make([]string, 0, len(d.breakpoints))
	for node := range d.breakpoints {
		out = append(out, node)
	}
	return out
}

// Pause pauses execution. The walk goroutine will block at the next
// node boundary (OnEvent for node_enter).
func (d *DebugController) Pause() {
	d.mu.Lock()
	defer d.mu.Unlock()
	if d.state != StatePaused {
		d.state = StatePaused
		d.bridge.Emit(Event{Type: EventPaused, Node: d.currentNode})
	}
}

// Resume unpauses execution.
func (d *DebugController) Resume() {
	d.mu.Lock()
	if d.state != StatePaused {
		d.mu.Unlock()
		return
	}
	d.state = StateRunning
	d.mu.Unlock()

	d.bridge.Emit(Event{Type: EventResumed})

	// Unblock the gate — non-blocking send in case nobody is waiting
	select {
	case d.gate <- struct{}{}:
	default:
	}
}

// AdvanceNode resumes execution until the next node_enter, then pauses again.
func (d *DebugController) AdvanceNode() {
	d.Resume()
}

// Snapshot returns the current circuit state.
func (d *DebugController) Snapshot() CircuitSnapshot {
	d.mu.Lock()
	defer d.mu.Unlock()

	bps := make([]string, 0, len(d.breakpoints))
	for node := range d.breakpoints {
		bps = append(bps, node)
	}

	visited := make([]string, len(d.visited))
	copy(visited, d.visited)

	arts := make(map[string]string, len(d.artifacts))
	for k, v := range d.artifacts {
		arts[k] = v
	}

	return CircuitSnapshot{
		State:        d.state.String(),
		CurrentNode:  d.currentNode,
		Breakpoints:  bps,
		NodesVisited: visited,
		Artifacts:    arts,
	}
}

// AddAssertion registers an invariant check.
func (d *DebugController) AddAssertion(a Assertion) {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.assertions = append(d.assertions, a)
}

// RunAssertions runs all registered assertions against the current snapshot.
func (d *DebugController) RunAssertions() []error {
	snap := d.Snapshot()
	d.mu.Lock()
	assertions := make([]Assertion, len(d.assertions))
	copy(assertions, d.assertions)
	d.mu.Unlock()

	var errs []error
	for _, a := range assertions {
		if err := a.Predicate(snap); err != nil {
			errs = append(errs, fmt.Errorf("assertion %q: %w", a.Name, err))
		}
	}
	return errs
}

// OnEvent implements circuit.WalkObserver. It intercepts walk events
// to implement breakpoint and pause semantics, then forwards to the bridge.
func (d *DebugController) OnEvent(we circuit.WalkEvent) {
	switch we.Type {
	case circuit.EventNodeEnter:
		d.mu.Lock()
		d.currentNode = we.Node
		d.visited = append(d.visited, we.Node)

		shouldPause := d.breakpoints[we.Node]
		d.mu.Unlock()

		// Forward to bridge first so subscribers see the event
		d.bridge.OnEvent(we)

		if shouldPause {
			d.bridge.Emit(Event{Type: EventBreakpointHit, Node: we.Node})
			d.mu.Lock()
			d.state = StatePaused
			d.mu.Unlock()
		}

		d.mu.Lock()
		paused := d.state == StatePaused
		d.mu.Unlock()

		if paused {
			// Block until Resume or AdvanceNode
			<-d.gate

			// If AdvanceNode was used, re-pause at next node
			// (AdvanceNode calls Resume which sets state to Running;
			// it becomes paused again at the next node_enter)
		}

	case circuit.EventNodeExit:
		d.mu.Lock()
		if we.Artifact != nil {
			d.artifacts[we.Node] = we.Artifact.Type()
		}
		d.mu.Unlock()
		d.bridge.OnEvent(we)

	default:
		d.bridge.OnEvent(we)
	}
}

// State returns the current debug state.
func (d *DebugController) State() DebugState {
	d.mu.Lock()
	defer d.mu.Unlock()
	return d.state
}
