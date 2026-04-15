package engine

import (
	"context"
	"fmt"

	"github.com/dpopsuev/origami/circuit"
	"github.com/dpopsuev/origami/engine/gate"
)

// HITLResult is returned after a HITL walk step completes. It indicates
// whether the walk was interrupted (awaiting human input) or completed.
type HITLResult struct {
	PromptPath  string
	CurrentStep string
	IsDone      bool
	Explanation string
}

// CheckpointInspection contains structured metadata about a checkpointed walker
// for HITL inspection — current node, status, interrupt data, and history.
type CheckpointInspection struct {
	WalkerID      string               `json:"walker_id"`
	CurrentNode   string               `json:"current_node"`
	Status        string               `json:"status"`
	InterruptData map[string]any       `json:"interrupt_data,omitempty"`
	History       []circuit.StepRecord `json:"history"`
	LoopCounts    map[string]int       `json:"loop_counts,omitempty"`
}

// InspectCheckpoint loads a checkpoint and returns structured metadata
// for HITL inspection. Returns an error if the checkpoint does not exist.
func InspectCheckpoint(cp circuit.Checkpointer, walkerID string) (*CheckpointInspection, error) {
	state, err := cp.Load(walkerID)
	if err != nil {
		return nil, fmt.Errorf("load checkpoint %s: %w", walkerID, err)
	}
	if state == nil {
		return nil, fmt.Errorf("checkpoint %s: %w", walkerID, ErrWalkerNotFound)
	}

	var interruptData map[string]any
	if data, ok := state.Context["interrupt_data"].(map[string]any); ok {
		interruptData = data
	}

	return &CheckpointInspection{
		WalkerID:      state.ID,
		CurrentNode:   state.CurrentNode,
		Status:        state.Status,
		InterruptData: interruptData,
		History:       state.History,
		LoopCounts:    state.LoopCounts,
	}, nil
}

// LoadCheckpointState loads a WalkerState from a JSON checkpoint directory.
// Returns nil, nil if no checkpoint exists for the given walker ID.
func LoadCheckpointState(checkpointDir, walkerID string) (*circuit.WalkerState, error) {
	cp, err := NewJSONCheckpointer(checkpointDir)
	if err != nil {
		return nil, err
	}
	state, err := cp.Load(walkerID)
	if err != nil {
		return nil, nil
	}
	return state, nil
}

// BuildHITLResult interprets a walker's state after a walk to produce an
// HITLResult. If the walk was interrupted, it extracts the prompt path and
// step from the interrupt data. If the walk completed, it returns IsDone.
func BuildHITLResult(walker circuit.Walker, walkErr error) (*HITLResult, error) {
	state := walker.State()

	if state.Status == walkStatusInterrupted {
		step := state.CurrentNode
		promptPath := ""
		if data, ok := state.Context["interrupt_data"].(map[string]any); ok {
			promptPath, _ = data["prompt_path"].(string)
			if s, ok := data["step"].(string); ok {
				step = s
			}
		}
		return &HITLResult{
			PromptPath:  promptPath,
			CurrentStep: step,
			Explanation: fmt.Sprintf("generated prompt for %s", step),
		}, nil
	}

	if walkErr != nil {
		return nil, fmt.Errorf("walk: %w", walkErr)
	}

	return &HITLResult{
		IsDone:      true,
		CurrentStep: "DONE",
		Explanation: "circuit complete",
	}, nil
}

// RestoreWalkerState applies a previously checkpointed state onto a new
// walker. Returns the node to resume from ("" if no checkpoint).
func RestoreWalkerState(walker circuit.Walker, loaded *circuit.WalkerState) string {
	if loaded == nil {
		return ""
	}
	state := walker.State()
	for k, v := range loaded.LoopCounts {
		state.LoopCounts[k] = v
	}
	state.Status = loaded.Status
	state.CurrentNode = loaded.CurrentNode
	state.History = loaded.History
	state.Outputs = loaded.Outputs
	return loaded.CurrentNode
}

// --- Approval gate resume logic ---

// ResumeFromGate resumes a walk that was interrupted at a gated node.
//
// If the gate was rejected: injects the rejection comment into the walker's
// context as "rejection_feedback" and re-walks from the gated node (the node
// re-executes with feedback visible). The gate will park the output again,
// returning ErrWalkInterrupted.
//
// If the gate was approved: resumes the walk from the edges after the gated
// node (walks the successor node).
//
// If the gate is still pending: returns gate.ErrApprovalStillPending.
func ResumeFromGate(ctx context.Context, g Graph, walker circuit.Walker, store gate.ApprovalStore) error {
	state := walker.State()
	if state.Status != walkStatusInterrupted || state.CurrentNode == "" {
		return gate.ErrNoGatedNode
	}

	gatedNode := state.CurrentNode

	// Build the approval ID matching the format used by parkForApproval.
	gateAttemptKey := "_gate_attempt:" + gatedNode
	attempt := 0
	if v, ok := state.Context[gateAttemptKey]; ok {
		if n, ok := v.(int); ok {
			attempt = n
		}
	}
	if attempt == 0 {
		// Fallback: no attempt counter means pre-retry code parked with the old format.
		attempt = 1
	}
	itemID := fmt.Sprintf("%s:%s:%d", state.ID, gatedNode, attempt)

	item, err := store.Get(ctx, itemID)
	if err != nil {
		return fmt.Errorf("resume gate: %w", err)
	}

	switch item.Status {
	case gate.ApprovalPending:
		return fmt.Errorf("%w: %s", gate.ErrApprovalStillPending, itemID)

	case gate.ApprovalRejected:
		// Inject rejection feedback into walker context for the retry.
		if item.Decision != nil && item.Decision.Comment != "" {
			state.Context[gate.ContextKeyRejectionFeedback] = item.Decision.Comment
		}
		// Reset walker status so Walk can proceed.
		state.Status = walkStatusRunning
		return g.Walk(ctx, walker, gatedNode)

	case gate.ApprovalApproved:
		// Clear any prior rejection feedback.
		delete(state.Context, gate.ContextKeyRejectionFeedback)
		state.Status = walkStatusRunning

		// Find the successor node and resume from there.
		edges := g.EdgesFrom(gatedNode)
		if len(edges) == 0 {
			// Terminal gated node — walk is done.
			state.Status = walkStatusDone
			return nil
		}

		nextNode := edges[0].To()
		// Check if the successor is the terminal pseudo-node (e.g. _done).
		// Pseudo-nodes don't exist in the graph's node index.
		if _, exists := g.NodeByName(nextNode); !exists {
			state.Status = walkStatusDone
			return nil
		}
		return g.Walk(ctx, walker, nextNode)

	default:
		return fmt.Errorf("%w: %q for %s", gate.ErrUnexpectedApprovalStatus, item.Status, itemID)
	}
}
