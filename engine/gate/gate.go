// Package gate defines approval gate types for engine-enforced HITL.
//
// Types live here so consumers (mcp/, testkit/, operator/) can depend on
// gate contracts without importing the full engine/ god package.
package gate

import (
	"context"
	"encoding/json"
	"time"
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

// Comment is directional feedback added to a pending approval item.
type Comment struct {
	Text     string    `json:"text"`
	Operator string    `json:"operator"`
	At       time.Time `json:"at"`
}

// ApprovalItem represents a node output parked for human review.
type ApprovalItem struct {
	ID         string          `json:"id"`
	CircuitRun string          `json:"circuit_run"`
	NodeName   string          `json:"node_name"`
	Circuit    string          `json:"circuit,omitempty"`  // sub-circuit that parked this item
	Priority   string          `json:"priority,omitempty"` // critical > high > medium > low
	SpecID     string          `json:"spec_id,omitempty"`  // Scribe artifact to validate against
	Output     json.RawMessage `json:"output"`
	ParkedAt   time.Time       `json:"parked_at"`
	Status     ApprovalStatus  `json:"status"`
	Decision   *Decision       `json:"decision,omitempty"`
	Comments   []Comment       `json:"comments,omitempty"`
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

	// AddComment appends directional feedback to a pending item.
	AddComment(ctx context.Context, id string, comment Comment) error
}

// Notifier sends notifications when items are parked for approval.
// Implementations: StubNotifier (test), SlackNotifier, WebhookNotifier.
type Notifier interface {
	Notify(ctx context.Context, item ApprovalItem) error
}

// ContextKeyRejectionFeedback is the walker context key where rejection
// comments are injected before retrying a gated node.
const ContextKeyRejectionFeedback = "rejection_feedback"

// ContextKeyGateComments is the walker context key where accumulated
// gate comments are injected before retrying a gated node.
const ContextKeyGateComments = "gate_comments"
