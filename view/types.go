package view

import (
	"time"

	"github.com/dpopsuev/origami/circuit"
)

// NodeVisualState represents the visual state of a node during a walk.
type NodeVisualState string

const (
	NodeIdle      NodeVisualState = "idle"
	NodeActive    NodeVisualState = "active"
	NodeCompleted NodeVisualState = "completed"
	NodeError     NodeVisualState = "error"
)

// NodeState holds the visual state for a single node in the circuit.
type NodeState struct {
	Name    string          `json:"name"`
	State   NodeVisualState `json:"state"`
	Zone    string          `json:"zone,omitempty"`
	Element string          `json:"element,omitempty"`
}

// WalkerPosition tracks a walker's current location in the circuit.
type WalkerPosition struct {
	WalkerID string `json:"walker_id"`
	Node     string `json:"node"`
	Element  string `json:"element,omitempty"`
}

// CaseVisualState represents the lifecycle state of a case in the TUI.
type CaseVisualState string

const (
	CasePending   CaseVisualState = "pending"
	CaseActive    CaseVisualState = "active"
	CaseCompleted CaseVisualState = "completed"
	CaseError     CaseVisualState = "error"
)

// CaseInfo tracks a case's lifecycle state and current position.
type CaseInfo struct {
	CaseID  string          `json:"case_id"`
	State   CaseVisualState `json:"state"`
	Node    string          `json:"node"`
	Element string          `json:"element,omitempty"`
}

// CircuitSnapshot is the full point-in-time state of a circuit:
// topology metadata, node visual states, walker positions, debug state.
type CircuitSnapshot struct {
	CircuitName string                     `json:"circuit_name"`
	Def         *circuit.CircuitDef      `json:"def,omitempty"`
	Nodes       map[string]NodeState       `json:"nodes"`
	Walkers     map[string]WalkerPosition  `json:"walkers"`
	Cases       map[string]CaseInfo        `json:"cases,omitempty"`
	Breakpoints map[string]bool            `json:"breakpoints"`
	Paused      bool                       `json:"paused"`
	Completed   bool                       `json:"completed"`
	Error       string                     `json:"error,omitempty"`
	Timestamp   time.Time                  `json:"timestamp"`
	CaseResults []CaseResult               `json:"case_results,omitempty"`
}

// DiffType classifies the kind of state change in a StateDiff.
type DiffType string

const (
	DiffNodeState         DiffType = "node_state"
	DiffWalkerMoved       DiffType = "walker_moved"
	DiffWalkerAdded       DiffType = "walker_added"
	DiffWalkerRemoved     DiffType = "walker_removed"
	DiffBreakpointSet     DiffType = "breakpoint_set"
	DiffBreakpointCleared DiffType = "breakpoint_cleared"
	DiffPaused            DiffType = "paused"
	DiffResumed           DiffType = "resumed"
	DiffCompleted         DiffType = "completed"
	DiffError             DiffType = "error"
	DiffReset             DiffType = "reset"
)

// StateDiff is an incremental change to the circuit's visual state.
// Subscribers receive a stream of diffs to keep their rendering in sync.
type StateDiff struct {
	Type      DiffType        `json:"type"`
	Node      string          `json:"node,omitempty"`
	Walker    string          `json:"walker,omitempty"`
	State     NodeVisualState `json:"state,omitempty"`
	Error     string          `json:"error,omitempty"`
	Timestamp time.Time       `json:"timestamp"`
}
