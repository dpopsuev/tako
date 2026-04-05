package agentport

import "github.com/dpopsuev/troupe/signal"

// Type aliases — definitions live in troupe/signal.
type (
	Signal       = signal.Signal
	Performative = signal.Performative
	Bus          = signal.Bus
	MemBus       = signal.MemBus
	DurableBus   = signal.DurableBus
)

// Performative constants.
const (
	Inform    = signal.Inform
	Request   = signal.Request
	Confirm   = signal.Confirm
	Refuse    = signal.Refuse
	Handoff   = signal.Handoff
	Directive = signal.Directive
)

// Signal event constants.
const (
	EventWorkerStarted  = signal.EventWorkerStarted
	EventWorkerStopped  = signal.EventWorkerStopped
	EventWorkerStart    = signal.EventWorkerStart
	EventWorkerDone     = signal.EventWorkerDone
	EventWorkerError    = signal.EventWorkerError
	EventShouldStop     = signal.EventShouldStop
	EventBudgetUpdate   = signal.EventBudgetUpdate
	EventZoneShift      = signal.EventZoneShift
	EventDispatchRouted = signal.EventDispatchRouted
	EventHookExecuted   = signal.EventHookExecuted
	EventVetoApplied    = signal.EventVetoApplied
)

// Signal meta key constants.
const (
	MetaKeyWorkerID       = signal.MetaKeyWorkerID
	MetaKeyError          = signal.MetaKeyError
	MetaKeyUsed           = signal.MetaKeyUsed
	MetaKeyFromZone       = signal.MetaKeyFromZone
	MetaKeyToZone         = signal.MetaKeyToZone
	MetaKeyMode           = signal.MetaKeyMode
	MetaKeyBytes          = signal.MetaKeyBytes
	MetaKeyInFlight       = signal.MetaKeyInFlight
	MetaKeyVia            = signal.MetaKeyVia
	MetaKeyPromptPath     = signal.MetaKeyPromptPath
	MetaKeyDispatchReason = signal.MetaKeyDispatchReason
	MetaKeyQueueDepth     = signal.MetaKeyQueueDepth
	MetaKeyHookName       = signal.MetaKeyHookName
	MetaKeyHookPhase      = signal.MetaKeyHookPhase
)

// Agent name constants.
const (
	AgentWorker     = signal.AgentWorker
	AgentSupervisor = signal.AgentSupervisor
	AgentServer     = signal.AgentServer
	AgentMediator   = signal.AgentMediator
)

// Constructors.
var (
	NewMemBus     = signal.NewMemBus
	NewDurableBus = signal.NewDurableBus
)

// HealthSummary is a stub for removed Supervisor health monitoring.
// TODO(troupe): replace with Troupe Hook-based health in Phase 3.
type HealthSummary struct {
	QueueDepth int              `json:"queue_depth"`
	Workers    []WorkerSnapshot `json:"workers"`
}

// WorkerSnapshot captures a point-in-time worker state.
type WorkerSnapshot struct {
	WorkerID   string `json:"worker_id"`
	State      string `json:"state"`
	ErrorCount int    `json:"error_count"`
	LastError  string `json:"last_error,omitempty"`
}
