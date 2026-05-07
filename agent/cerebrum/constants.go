package cerebrum

import "errors"

// EventKind identifies the type of event flowing through buses.
type EventKind string

func (k EventKind) String() string { return string(k) }

const (
	EventOrgan       EventKind = "organ"
	EventOrganResult EventKind = "organ.result"
	EventOrganError  EventKind = "organ.error"
	EventApprovalHITL EventKind = "approval.hitl"

	EventSensoryAlarm     EventKind = "sensory.alarm"
	EventSensoryEmergency EventKind = "sensory.emergency"
	EventSensoryTimer     EventKind = "sensory.timer"
	EventSensoryWarning   EventKind = "sensory.warning"

	EventMotorExecute    EventKind = "motor.execute"
	EventMotorDeniedPhase EventKind = "motor.denied.phase"
	EventMotorDeniedHITL EventKind = "motor.denied.hitl"
	EventMotorPendingHITL EventKind = "motor.pending.hitl"
)

// MessageRole identifies LLM message roles.
type MessageRole = string

const (
	RoleUser      MessageRole = "user"
	RoleAssistant MessageRole = "assistant"
	RoleTool      MessageRole = "tool"
)

// ThinkingLevel maps cognitive gears to LLM thinking budgets.
type ThinkingLevel = string

const (
	ThinkingMinimal ThinkingLevel = "minimal"
	ThinkingMedium  ThinkingLevel = "medium"
	ThinkingHigh    ThinkingLevel = "high"
)

// StepStatus tracks PipeStep execution state.
type StepStatus = string

const (
	StepPending  StepStatus = "pending"
	StepReady    StepStatus = "ready"
	StepRunning  StepStatus = "running"
	StepComplete StepStatus = "complete"
	StepFailed   StepStatus = "failed"
	StepSkipped  StepStatus = "skipped"
)

// Sentinel errors for pipe execution.
var (
	ErrRunNotFound  = errors.New("run not found")
	ErrStepNotFound = errors.New("step not found")
	ErrPipeExists   = errors.New("pipe already exists")
)

// toolOutputMax is the maximum character length for tool output before truncation.
const toolOutputMax = 2000
