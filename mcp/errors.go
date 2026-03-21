package mcp

import "errors"

// Sentinel errors returned by CircuitServer tool handlers.
var (
	ErrNoActiveSession    = errors.New("no active session; call start_circuit first to create one")
	ErrDispatchIDRequired = errors.New("dispatch_id is required (got 0); did you submit after available=false?")
	ErrStepRequired       = errors.New("step is required")
	ErrEventRequired      = errors.New("event is required")
	ErrAgentRequired      = errors.New("agent is required")
)
