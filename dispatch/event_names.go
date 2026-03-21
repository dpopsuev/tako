package dispatch

// Signal event name constants for cross-package bus.Emit calls.
// Using constants prevents typo-induced mismatches between emitters
// and listeners (e.g., SupervisorTracker.Process).
const (
	EventWorkerStarted = "worker_started"
	EventWorkerStopped = "worker_stopped"
	EventWorkerStart   = "start"
	EventWorkerDone    = "done"
	EventWorkerError   = "error"
	EventShouldStop    = "should_stop"
	EventBudgetUpdate  = "budget_update"
	EventZoneShift     = "zone_shift"
)

// Signal meta key constants used in bus.Emit meta maps and read by
// SupervisorTracker.Process and other signal consumers.
const (
	MetaKeyWorkerID = "worker_id"
	MetaKeyError    = "error"
	MetaKeyUsed     = "used"
	MetaKeyFromZone = "from_zone"
	MetaKeyToZone   = "to_zone"
)

// Agent name constants used as the agent parameter in bus.Emit calls.
const (
	AgentWorker     = "worker"
	AgentSupervisor = "supervisor"
	AgentServer     = "server"
)
