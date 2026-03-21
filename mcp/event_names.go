package mcp

// Signal event name constants emitted by CircuitServer and CircuitSession.
const (
	EventSessionStarted    = "session_started"
	EventSessionDone       = "session_done"
	EventSessionError      = "session_error"
	EventCircuitDone       = "circuit_done"
	EventStepReady         = "step_ready"
	EventArtifactSubmitted = "artifact_submitted"
)
