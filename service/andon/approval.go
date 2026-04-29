package andon

import (
	"context"
	"encoding/json"
	"time"
)

const GateApproval = "approval"

type ApprovalStatus string

const (
	ApprovalPending  ApprovalStatus = "pending"
	ApprovalApproved ApprovalStatus = "approved"
	ApprovalRejected ApprovalStatus = "rejected"
)

type Comment struct {
	Text     string    `json:"text"`
	Operator string    `json:"operator"`
	At       time.Time `json:"at"`
}

type ApprovalItem struct {
	ID       string          `json:"id"`
	Station  string          `json:"station"`
	Agent    string          `json:"agent"`
	Priority string          `json:"priority,omitempty"`
	SpecID   string          `json:"spec_id,omitempty"`
	Output   json.RawMessage `json:"output"`
	ParkedAt time.Time       `json:"parked_at"`
	Status   ApprovalStatus  `json:"status"`
	Decision *Decision       `json:"decision,omitempty"`
	Comments []Comment       `json:"comments,omitempty"`
}

type Decision struct {
	Status   ApprovalStatus `json:"status"`
	Comment  string         `json:"comment,omitempty"`
	Operator string         `json:"operator"`
}

type ApprovalStore interface {
	Park(ctx context.Context, item ApprovalItem) error
	Get(ctx context.Context, id string) (*ApprovalItem, error)
	List(ctx context.Context, status ApprovalStatus) ([]ApprovalItem, error)
	Resolve(ctx context.Context, id string, decision Decision) error
	AddComment(ctx context.Context, id string, comment Comment) error
}

type Notifier interface {
	Notify(ctx context.Context, item ApprovalItem) error
}
