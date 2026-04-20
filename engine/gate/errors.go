package gate

import "errors"

var (
	// ErrApprovalNotFound is returned when an approval item does not exist.
	ErrApprovalNotFound = errors.New("approval not found")

	// ErrApprovalStillPending is returned when trying to resume a gate that hasn't been decided yet.
	ErrApprovalStillPending = errors.New("approval still pending")

	// ErrNoGatedNode is returned when ResumeFromGate is called but the walker isn't parked at a gate.
	ErrNoGatedNode = errors.New("walker is not parked at a gated node")

	// ErrUnexpectedApprovalStatus is returned when an approval item has an unrecognized status.
	ErrUnexpectedApprovalStatus = errors.New("unexpected approval status")

	// ErrApprovalNotPending is returned when trying to comment on an already-resolved item.
	ErrApprovalNotPending = errors.New("approval item is not pending")
)
