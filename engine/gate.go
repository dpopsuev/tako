package engine

// Category: Execution — approval gate for write nodes.

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/dpopsuev/origami/circuit"
)

// GateApproval is the gate value that triggers engine-enforced approval.
const GateApproval = "approval"

// ApprovalStatus tracks the lifecycle of an approval item.
type ApprovalStatus string

const (
	ApprovalPending  ApprovalStatus = "pending"
	ApprovalApproved ApprovalStatus = "approved"
	ApprovalRejected ApprovalStatus = "rejected"
)

// ApprovalItem represents a node output parked for human review.
type ApprovalItem struct {
	ID         string          `json:"id"`
	CircuitRun string          `json:"circuit_run"`
	NodeName   string          `json:"node_name"`
	Output     json.RawMessage `json:"output"`
	ParkedAt   time.Time       `json:"parked_at"`
	Status     ApprovalStatus  `json:"status"`
	Decision   *Decision       `json:"decision,omitempty"`
}

// Decision is the human's verdict on a parked approval item.
type Decision struct {
	Status   ApprovalStatus `json:"status"`
	Comment  string         `json:"comment,omitempty"`
	Operator string         `json:"operator"`
}

// ApprovalStore persists pending approval items durably.
// Implementations: MemoryApprovalStore (test), SQLiteApprovalStore (production).
type ApprovalStore interface {
	// Park stores a node output for human review.
	Park(ctx context.Context, item ApprovalItem) error

	// Get retrieves an approval item by ID.
	Get(ctx context.Context, id string) (*ApprovalItem, error)

	// List returns all items matching the given status.
	List(ctx context.Context, status ApprovalStatus) ([]ApprovalItem, error)

	// Resolve records the human's decision on a pending item.
	Resolve(ctx context.Context, id string, decision Decision) error
}

// Notifier sends notifications when items are parked for approval.
// Implementations: StubNotifier (test), SlackNotifier, WebhookNotifier.
type Notifier interface {
	Notify(ctx context.Context, item ApprovalItem) error
}

// ContextKeyRejectionFeedback is the walker context key where rejection
// comments are injected before retrying a gated node.
const ContextKeyRejectionFeedback = "rejection_feedback"

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
// If the gate is still pending: returns ErrApprovalStillPending.
func ResumeFromGate(ctx context.Context, g Graph, walker circuit.Walker, store ApprovalStore) error {
	state := walker.State()
	if state.Status != walkStatusInterrupted || state.CurrentNode == "" {
		return ErrNoGatedNode
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
	case ApprovalPending:
		return fmt.Errorf("%w: %s", ErrApprovalStillPending, itemID)

	case ApprovalRejected:
		// Inject rejection feedback into walker context for the retry.
		if item.Decision != nil && item.Decision.Comment != "" {
			state.Context[ContextKeyRejectionFeedback] = item.Decision.Comment
		}
		// Reset walker status so Walk can proceed.
		state.Status = walkStatusRunning
		return g.Walk(ctx, walker, gatedNode)

	case ApprovalApproved:
		// Clear any prior rejection feedback.
		delete(state.Context, ContextKeyRejectionFeedback)
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
		return fmt.Errorf("%w: %q for %s", ErrUnexpectedApprovalStatus, item.Status, itemID)
	}
}
