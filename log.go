package framework

// Log component names — used as slog "component" field values.
// Each subsystem has one name for consistent filtering.
const (
	LogComponentWalk      = "walk"
	LogComponentDSL       = "dsl"
	LogComponentCalibrate = "calibrate"
	LogComponentBatch     = "batch_walk"
	LogComponentTransform = "transformer"
)

// Log event names — used as slog msg values.
// Each decision point has one name for consistent grep/search.
const (
	// Walk events
	LogNodeEnter       = "node enter"
	LogNodeExit        = "node exit"
	LogEdgeTaken       = "edge taken"
	LogEdgeNoMatch     = "no matching edge"
	LogLoopIncremented = "loop incremented"
	LogWalkComplete    = "walk complete"
	LogWalkError       = "walk error"
	LogDelegateStart   = "delegate start"
	LogDelegateComplete = "delegate complete"

	// DSL events
	LogOverlayMerge         = "overlay merge"
	LogOverlayMergeComplete = "overlay merge complete"
	LogSubCircuitLoaded     = "sub-circuit loaded"

	// Calibrate events
	LogRunStart       = "calibration run start"
	LogCaseComplete   = "case complete"
	LogAllCasesFailed = "all cases failed"
)

// Log field keys — used as slog attribute keys.
// Consistent naming across all subsystems.
const (
	LogKeyComponent = "component"
	LogKeyNode      = "node"
	LogKeyEdge      = "edge"
	LogKeyFrom      = "from"
	LogKeyTo        = "to"
	LogKeyWalker    = "walker"
	LogKeyElapsed   = "elapsed_ms"
	LogKeyLoop      = "loop"
	LogKeyShortcut  = "shortcut"
	LogKeyCount     = "count"
	LogKeyError     = "error"
	LogKeyCaseID    = "case_id"
	LogKeyCircuit   = "circuit"
)
