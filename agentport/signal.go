package agentport

import "github.com/dpopsuev/jericho/signal"

// Type aliases — definitions live in bugle/signal.
type (
	Signal        = signal.Signal
	Performative  = signal.Performative
	Bus           = signal.Bus
	MemBus        = signal.MemBus
	DurableBus    = signal.DurableBus
	Supervisor    = signal.Supervisor
	WorkerState   = signal.WorkerState
	HealthSummary = signal.HealthSummary

	SupervisorOption = signal.SupervisorOption
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
	NewSupervisor = signal.NewSupervisor
)

// Supervisor option constructors.
var (
	WithSilenceThreshold = signal.WithSilenceThreshold
	WithErrorThreshold   = signal.WithErrorThreshold
	WithBudgetTotal      = signal.WithBudgetTotal
)
