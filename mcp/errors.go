package mcp

import "errors"

// Sentinel errors returned by CircuitServer tool handlers.
var (
	ErrNoActiveSession    = errors.New("no active session; call start_circuit first to create one")
	ErrDispatchIDRequired = errors.New("dispatch_id is required (got 0); did you submit after available=false?")
	ErrStepRequired       = errors.New("step is required")
	ErrEventRequired      = errors.New("event is required")
	ErrAgentRequired      = errors.New("agent is required")
	// ErrUnknownStep is returned for: unknown step
	ErrUnknownStep = errors.New("unknown step")

	// ErrCircuitTypeIsRequiredInExtraWhenMultipleTypesAreRegi is returned for: circuit_type is required in extra when multiple types are registered (available
	ErrCircuitTypeIsRequiredInExtraWhenMultipleTypesAreRegi = errors.New("circuit_type is required in extra when multiple types are registered (available")

	// ErrUnknownCircuitType is returned for: unknown circuit_type
	ErrUnknownCircuitType = errors.New("unknown circuit_type")

	// ErrUnknownCircuitAction is returned for: unknown circuit action
	ErrUnknownCircuitAction = errors.New("unknown circuit action")

	// ErrUnknownSignalAction is returned for: unknown signal action
	ErrUnknownSignalAction = errors.New("unknown signal action")

	// ErrUnknownMetric is returned for: unknown confusion metric
	ErrUnknownMetric = errors.New("unknown confusion metric")

	// ErrACircuitSessionIsAlreadyRunningId is returned for: a circuit session is already running (id=
	ErrACircuitSessionIsAlreadyRunningId = errors.New("a circuit session is already running (id=")

	// ErrNoResultAvailable is returned for: no result available
	ErrNoResultAvailable = errors.New("no result available")

	// ErrCaseIdIsRequiredForDetailAction is returned for: case_id is required for detail action
	ErrCaseIdIsRequiredForDetailAction = errors.New("case_id is required for detail action")

	// ErrCaseId is returned for: case_id
	ErrCaseId = errors.New("case_id")

	// ErrSessionId is returned for: session_id
	ErrSessionId = errors.New("session_id")

	// ErrUnknownTraceAction is returned for: unknown trace action
	ErrUnknownTraceAction = errors.New("unknown trace action")

	// ErrNoTraceDataStateDirNotConfiguredOrRunNotFound is returned for: no trace data: StateDir not configured or run not found
	ErrNoTraceDataStateDirNotConfiguredOrRunNotFound = errors.New("no trace data: StateDir not configured or run not found")

	// ErrNoReportDataStateDirNotConfiguredOrRunNotFound is returned for: no report data: StateDir not configured or run not found
	ErrNoReportDataStateDirNotConfiguredOrRunNotFound = errors.New("no report data: StateDir not configured or run not found")

	// ErrStateDirNotConfigured is returned for: StateDir not configured
	ErrStateDirNotConfigured = errors.New("StateDir not configured")

	// ErrCapacityGate is returned when the capacity gate detects insufficient workers.
	ErrCapacityGate = errors.New("capacity gate: insufficient concurrent workers")

	// ErrSessionTTLExpired is returned when a session's TTL is exceeded.
	ErrSessionTTLExpired = errors.New("session TTL expired: no activity")

	// ErrCheckpointerNotConfigured is returned when inspect/resume is called without a Checkpointer.
	ErrCheckpointerNotConfigured = errors.New("checkpointer not configured; set CircuitConfig.Checkpointer for HITL")

	// ErrWalkerIDRequired is returned when inspect/resume is called without a walker_id.
	ErrWalkerIDRequired = errors.New("walker_id is required")

	// ErrPromptNameRequired is returned when a prompt action requires a name but none was given.
	ErrPromptNameRequired = errors.New("prompt name is required")

	// ErrUnknownPromptAction is returned for unrecognized prompt tool actions.
	ErrUnknownPromptAction = errors.New("unknown prompt action")

	// ErrUnknownResourceAction is returned for unrecognized resource tool actions.
	ErrUnknownResourceAction = errors.New("unknown resource action")

	// ErrResourceKindRequired is returned when a resource action requires a kind.
	ErrResourceKindRequired = errors.New("kind is required")

	// ErrResourceNameRequired is returned when a resource action requires a name.
	ErrResourceNameRequired = errors.New("name is required")

	// ErrResourceNotFound is returned when a resource cannot be found by kind+name.
	ErrResourceNotFound = errors.New("resource not found")

	// ErrResourceFilesRequired is returned when diff action is missing file paths.
	ErrResourceFilesRequired = errors.New("file_a and file_b are required for diff")

	// ErrUnknownApprovalAction is returned for unrecognized approval tool actions.
	ErrUnknownApprovalAction = errors.New("unknown approval action")

	// ErrApprovalIDRequired is returned when an approval action requires an ID.
	ErrApprovalIDRequired = errors.New("id is required for approval action")

	// ErrUnknownInstrumentAction is returned for unrecognized instrument tool actions.
	ErrUnknownInstrumentAction = errors.New("unknown instrument action")

	// ErrUnknownOperatorAction is returned for unrecognized operator tool actions.
	ErrUnknownOperatorAction = errors.New("unknown operator action")

	// ErrUnknownBudgetAction is returned for unrecognized budget tool actions.
	ErrUnknownBudgetAction = errors.New("unknown budget action")
)
