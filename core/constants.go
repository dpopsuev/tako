package core

// Walker context keys (underscore-prefixed, stored in WalkerState.Context).
const (
	ContextKeyTraceID = "_trace_id"
)

// Extra param keys (used in start_circuit extra map and mediator routing).
const (
	ExtraKeyCircuitType = "circuit_type"
	ExtraKeyTraceID     = "trace_id"
)

// TraceEvent metadata keys used in delegation event annotation.
const (
	TraceMetaDelegation = "delegation"
	TraceMetaSource     = "source"
)

// Papercup protocol JSON field names used in MCP tool call arguments
// and response parsing. In the framework root (not mcp/) to avoid
// circular imports — mediator_delegate.go is in package framework.
const (
	ProtoKeySessionID     = "session_id"
	ProtoKeyDone          = "done"
	ProtoKeyAvailable     = "available"
	ProtoKeyStep          = "step"
	ProtoKeyDispatchID    = "dispatch_id"
	ProtoKeyPromptContent = "prompt_content"
	ProtoKeyCaseID        = "case_id"
	ProtoKeyArtifactPath  = "artifact_path"
	ProtoKeyFields        = "fields"
	ProtoKeyExtra         = "extra"
	ProtoKeyError         = "error"
	ProtoKeyStatus        = "status"
	ProtoKeyStructured    = "structured"
	ProtoKeyTimeoutMS     = "timeout_ms"
)
