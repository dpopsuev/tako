package circuit

// Category: Core Primitives

import "context"

// Walker is an agent traversing a graph. It combines identity
// (who the agent is) with processing capability (how it handles nodes).
type Walker interface {
	Identity() AgentIdentity
	SetIdentity(*AgentIdentity)
	State() *WalkerState
	Handle(ctx context.Context, node Node, nc NodeContext) (Artifact, error)
}

// WalkerState tracks a walker's progress through a graph.
// It mirrors orchestrate.CaseState with string-based node names.
type WalkerState struct {
	ID                string              `json:"id"`
	CurrentNode       string              `json:"current_node"`
	LoopCounts        map[string]int      `json:"loop_counts"`
	Status            string              `json:"status"` // running, paused, done, error
	History           []StepRecord        `json:"history"`
	Context           map[string]any      `json:"context"`
	Outputs           map[string]Artifact `json:"-"`
	ConfidenceHistory []float64           `json:"confidence_history,omitempty"`
}

// NewWalkerState creates a WalkerState with initialized maps.
func NewWalkerState(id string) *WalkerState {
	return &WalkerState{
		ID:         id,
		Status:     "running",
		LoopCounts: make(map[string]int),
		Context:    make(map[string]any),
		Outputs:    make(map[string]Artifact),
	}
}

// RecordStep appends a step to the history and updates the current node.
func (ws *WalkerState) RecordStep(node, outcome, edgeID, timestamp string) {
	ws.History = append(ws.History, StepRecord{
		Node:      node,
		Outcome:   outcome,
		EdgeID:    edgeID,
		Timestamp: timestamp,
	})
	ws.CurrentNode = node
}

// IncrementLoop increments the loop counter for an edge and returns the new count.
func (ws *WalkerState) IncrementLoop(edgeID string) int {
	ws.LoopCounts[edgeID]++
	return ws.LoopCounts[edgeID]
}

// RecordConfidence appends a confidence value to the history.
func (ws *WalkerState) RecordConfidence(value float64) {
	ws.ConfidenceHistory = append(ws.ConfidenceHistory, value)
}

// MergeContext merges additions into the walker's accumulated context.
func (ws *WalkerState) MergeContext(additions map[string]any) {
	if additions == nil {
		return
	}
	for k, v := range additions {
		ws.Context[k] = v
	}
}

// TrajectoryType classifies a confidence convergence pattern.
type TrajectoryType string

const (
	TrajectoryUnderdamped      TrajectoryType = "underdamped"
	TrajectoryOverdamped       TrajectoryType = "overdamped"
	TrajectoryCriticallyDamped TrajectoryType = "critically_damped"
	TrajectoryUnstable         TrajectoryType = "unstable"
	TrajectoryInsufficient     TrajectoryType = "insufficient"
)

// ClassifyTrajectory analyzes a confidence history to determine the convergence pattern.
// Underdamped: many oscillations (3+ sign changes in derivative).
// Overdamped: monotonically increasing (0 sign changes).
// Critically damped: converging within 1 oscillation (1-2 sign changes).
// Unstable: final value lower than first (diverging).
// Insufficient: fewer than 3 data points.
func ClassifyTrajectory(history []float64) TrajectoryType {
	if len(history) < 3 {
		return TrajectoryInsufficient
	}

	if history[len(history)-1] < history[0] {
		return TrajectoryUnstable
	}

	signChanges := 0
	prevDelta := history[1] - history[0]
	for i := 2; i < len(history); i++ {
		delta := history[i] - history[i-1]
		if (prevDelta > 0 && delta < 0) || (prevDelta < 0 && delta > 0) {
			signChanges++
		}
		if delta != 0 {
			prevDelta = delta
		}
	}

	switch {
	case signChanges >= 3:
		return TrajectoryUnderdamped
	case signChanges == 0:
		return TrajectoryOverdamped
	default:
		return TrajectoryCriticallyDamped
	}
}

// ReadOnlyContext returns a shallow copy of the context map.
// Used to snapshot context at dialectic entry so nodes cannot mutate
// the shared state during adversarial debate. The original map is never
// exposed; writes to the copy are discarded after the dialectic round.
func ReadOnlyContext(ctx map[string]any) map[string]any {
	if ctx == nil {
		return nil
	}
	snapshot := make(map[string]any, len(ctx))
	for k, v := range ctx {
		snapshot[k] = v
	}
	return snapshot
}

// StepRecord logs a completed node visit.
type StepRecord struct {
	Node      string `json:"node"`
	Outcome   string `json:"outcome"`
	EdgeID    string `json:"edge_id"`
	Timestamp string `json:"timestamp"`
}

// ProcessWalker is a default Walker that delegates Handle to node.Process().
// Use when all nodes are transformer-backed and no custom walker logic is needed.
type ProcessWalker struct {
	identity AgentIdentity
	state    *WalkerState
}

// NewProcessWalker creates a Walker that delegates to node.Process().
func NewProcessWalker(id string) *ProcessWalker {
	return &ProcessWalker{
		identity: AgentIdentity{PersonaName: id},
		state:    NewWalkerState(id),
	}
}

// NewProcessWalkerWithIdentity creates a ProcessWalker with a pre-built identity.
func NewProcessWalkerWithIdentity(id *AgentIdentity, stateID string) *ProcessWalker {
	return &ProcessWalker{
		identity: *id,
		state:    NewWalkerState(stateID),
	}
}

func (w *ProcessWalker) Identity() AgentIdentity       { return w.identity }
func (w *ProcessWalker) SetIdentity(id *AgentIdentity) { w.identity = *id }
func (w *ProcessWalker) State() *WalkerState           { return w.state }

func (w *ProcessWalker) Handle(ctx context.Context, node Node, nc NodeContext) (Artifact, error) {
	return node.Process(ctx, nc)
}
