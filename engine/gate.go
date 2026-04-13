package engine

// Category: Execution — approval gate resume logic.

import (
	"context"
	"fmt"

	"github.com/dpopsuev/origami/circuit"
	"github.com/dpopsuev/origami/engine/gate"
)

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
