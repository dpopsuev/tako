package framework

// Walker context keys (underscore-prefixed, stored in WalkerState.Context).
const (
	ContextKeyTraceID = "_trace_id"
)

// Extra param keys (used in start_circuit extra map and mediator routing).
const (
	ExtraKeyCircuitType = "circuit_type"
	ExtraKeyTraceID     = "trace_id"
)
