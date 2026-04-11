package circuit

// Log component names — used as slog "component" field values.
// Each subsystem has one name for consistent filtering.
const (
	// Core
	LogComponentWalk      = "walk"
	LogComponentDSL       = "dsl"
	LogComponentCalibrate = "calibrate"
	LogComponentBatch     = "batch_walk"
	LogComponentTransform = "transformer"
	LogComponentBuild     = "build"
	LogComponentRegistry  = "registry"

	// Dispatch
	LogComponentMuxDispatch   = "mux_dispatch"
	LogComponentFileDispatch  = "file_dispatch"
	LogComponentBatchDispatch = "batch_dispatch"

	// MCP / Session
	LogComponentCircuitSession = "circuit_session"
	LogComponentSignalBus      = "signal_bus"

	// Ingest / Curate
	LogComponentDatasetPipeline = "dataset_pipeline"

	// Instrument
	LogComponentInstrument = "instrument"
)

// Log event names — used as slog msg values.
// Each decision point has one name for consistent grep/search.
const (
	// Walk events
	LogNodeEnter        = "node enter"
	LogNodeExit         = "node exit"
	LogEdgeTaken        = "edge taken"
	LogEdgeEvaluated    = "edge evaluated"
	LogEdgeNoMatch      = "no matching edge"
	LogLoopIncremented  = "loop incremented"
	LogWalkComplete     = "walk complete"
	LogWalkError        = "walk error"
	LogDelegateStart    = "delegate start"
	LogDelegateComplete = "delegate complete"

	// DSL events
	LogOverlayMerge         = "overlay merge"
	LogOverlayMergeComplete = "overlay merge complete"
	LogSubCircuitLoaded     = "sub-circuit loaded"
	LogSubCircuitSkipped    = "skipping sub-circuit with unresolved import"

	// Calibrate events
	LogRunStart        = "calibration run start"
	LogCaseComplete    = "case complete"
	LogAllCasesFailed  = "all cases failed"
	LogStartingRun     = "starting run"
	LogPartialFailures = "partial failures"
	LogUnvisitedNodes  = "declared nodes never visited by any case"

	// Build / diagnostics
	LogTopologySkipped         = "topology validator not registered, skipping validation"
	LogResolveCircuitHandler   = "resolve circuit handler"
	LogCircuitHandlerLocal     = "circuit handler resolved locally"
	LogCircuitHandlerMediator  = "circuit handler delegating to mediator"
	LogMergeComponents         = "merge components"
	LogUnreferencedHook        = "unreferenced hook"
	LogMissingHookRefs         = "missing hook references"
	LogCircuitMediatorFallback = "circuit node falling back to mediator delegation (no local circuit)"

	// Cache
	LogCacheHit  = "cache hit"
	LogCacheMiss = "cache miss"

	// HITL
	LogInspectCheckpoint = "inspect checkpoint"
	LogResumeWalk        = "resume walk"

	// Worker lifecycle
	LogWorkersSpawned    = "workers spawned"
	LogWorkerSpawnFailed = "worker spawn failed"

	// Runner / hooks
	LogSchemaValidationFailed = "artifact schema validation failed"
	LogBeforeHookNotFound     = "before-hook not found"
	LogBeforeHookError        = "before-hook error"
	LogHookNotFound           = "hook not found"
	LogHookError              = "hook error"

	// Transformer
	LogInputResolutionFailed = "input resolution failed"
	LogTransformerExecuting  = "transformer executing"
	LogTransformerFailed     = "transformer failed"
	LogTransformerCompleted  = "transformer completed"

	// Engine / run
	LogCheckpointRemoveFailed = "failed to remove checkpoint after successful walk"
	LogCaseWalkFailed         = "case walk failed"

	// Mediator
	LogMediatorDelegateStart     = "mediator delegate start"
	LogMediatorSessionStarted    = "mediator delegate session started"
	LogMediatorRelayChild        = "mediator relay child prompt"
	LogMediatorDelegateComplete  = "mediator delegate complete"
	LogMediatorStateDirFailed    = "failed to create mediator state dir, tracing disabled"
	LogMediatorTraceFailed       = "failed to create mediator trace recorder"
	LogSessionAffinityRegistered = "session affinity registered"

	// Dispatch — file
	LogDispatchBegin        = "dispatch begin"
	LogRemoveStaleArtifact  = "removing stale artifact before dispatch"
	LogSignalWritten        = "signal written"
	LogSignalWaiting        = "signal.json written, waiting for artifact"
	LogTimeoutReached       = "timeout reached"
	LogResponderError       = "responder reported error via signal"
	LogPollArtifactNotFound = "poll: artifact not found"
	LogPollArtifactFound    = "poll: artifact file found"
	LogPollInvalidJSON      = "poll: invalid JSON (possible partial write)"
	LogPollStaleArtifact    = "poll: stale artifact (dispatch_id mismatch)"
	LogArtifactValidated    = "artifact validated"
	LogArtifactAccepted     = "artifact validated and accepted"
	LogMarkDoneReadFailed   = "mark-done: cannot read signal"
	LogMarkDoneParseFailed  = "mark-done: cannot parse signal"
	LogMarkDone             = "mark-done"

	// Dispatch — CLI worker
	LogCLIExecFailed = "CLI execution failed"
	LogStepComplete  = "step complete"
	LogCLIExec       = "CLI exec"

	// Dispatch — ACP worker
	LogReadPromptFile      = "read prompt file"
	LogRoutingToCollective = "routing to collective"
	LogNoWorkersAvailable  = "no workers available"
	LogACPAgentFailed      = "ACP agent failed"

	// Dispatch — network
	LogNetworkServerStarted = "network server started"
	LogGetNextStepFailed    = "get next step failed"
	LogSubmitArtifactFailed = "submit artifact failed"

	// Dispatch — batch
	LogBatchDispatchBegin    = "batch dispatch begin"
	LogManifestWritten       = "manifest written"
	LogBudgetStatusFailed    = "failed to write budget status"
	LogBatchDispatchComplete = "batch dispatch complete"

	// Dispatch — mux
	LogMuxRegistered         = "mux dispatch registered"
	LogMuxCanceledSending    = "mux dispatch canceled while sending prompt"
	LogMuxAbortedSending     = "mux dispatch aborted while sending prompt"
	LogDispatchRoundTrip     = "dispatch round-trip"
	LogDispatchTimeout       = "dispatch timeout"
	LogMuxCanceledWaiting    = "mux dispatch canceled while waiting for artifact"
	LogDoubleSubmit          = "double submit detected"
	LogSubmitUnknownDispatch = "submit for unknown dispatch ID"
	LogMuxArtifactRouted     = "mux artifact routed"
	LogMuxAbort              = "mux dispatcher abort"

	// MCP — circuit server
	LogStepSchemasRegistered = "step schemas registered"
	LogRunDirFailed          = "failed to create run dir, tracing disabled"
	LogTraceRecorderFailed   = "failed to create trace recorder"
	LogConfigNameEmpty       = "CircuitConfig.Name is empty; affects logging and state directory naming"
	LogConfigSchemasEmpty    = "CircuitConfig.StepSchemas is empty; submit_step will reject all steps"
	LogConfigStateDirEmpty   = "CircuitConfig.StateDir is empty; walker tracing disabled — set StateDir to enable trace recording"
	LogReplacingSession      = "replacing completed/aborted session"
	LogForceReplacingSession = "force-replacing active session"
	LogCircuitSessionFailed  = "circuit session failed"
	LogCircuitSessionStarted = "circuit session started"
	LogGetNextStepError      = "get_next_step error"
	LogCircuitComplete       = "circuit complete"
	LogStepDispatched        = "step dispatched to worker"
	LogUnderCapacity         = "under capacity"
	LogCapacityGateAdvisory  = "capacity gate advisory on submit_step"
	LogStepSchemaFailed      = "step schema validation failed"
	LogStepArtifactAccepted  = "step artifact accepted"
	LogReportGeneratedError  = "report generated with error"
	LogReportGenerated       = "report generated"
	LogEmitSignalNoEvent     = "emit_signal rejected: empty event field"
	LogEmitSignalNoAgent     = "emit_signal rejected: empty agent field"
	LogWorkerRegistered      = "worker registered"
	LogSignalEmitted         = "signal emitted"

	// MCP — circuit session
	LogActivityReset       = "activity reset"
	LogTTLWatchdog         = "TTL watchdog triggered, aborting session"
	LogCircuitRunFailed    = "circuit run failed"
	LogCircuitRunComplete  = "circuit run complete"
	LogMarshalReportFailed = "failed to marshal report"
	LogWriteReportFailed   = "failed to write report.json"
	LogReportWritten       = "report written"
	LogWriteRunFailed      = "failed to write run.json"
	LogRunRecordWritten    = "run record written"
	LogGetNextStepTimeout  = "get_next_step timed out"
	LogStepDelivered       = "step delivered"

	// MCP — session bridge
	LogCollectiveSpawnFailed = "collective spawn failed, falling back to single-agent dispatch"
	LogACPDispatchError      = "ACP worker dispatch error"

	// Health — component health tracking
	LogComponentHealthCheck   = "component health check"
	LogComponentHealthy       = "component healthy"
	LogComponentUnhealthy     = "component unhealthy"
	LogComponentHealthSkipped = "component has no health checker"
	LogAllComponentsHealthy   = "all components healthy"

	// Fold — contract validation
	LogComponentFold              = "fold"
	LogSocketContractNotSatisfied = "socket contract not satisfied"
	LogSymbolNotExported          = "declared symbol not exported"
	LogSocketContractsValidated   = "socket contracts validated"
	LogExportValidationComplete   = "export validation complete"

	// DSL — graph validation
	LogUndefinedNodeRef      = "edge references undefined node"
	LogUndefinedStartNode    = "start references undefined node"
	LogUndefinedDoneNode     = "done references undefined node"
	LogCircuitGraphValidated = "circuit graph validated"

	// CLI commands
	LogRunningCircuit = "running circuit"
	LogCircuitDone    = "circuit completed"
	LogLSPStarted     = "origami-lsp started"
	LogConnected      = "connected"
	LogCircuitStarted = "circuit started"
	LogWorkerFailed   = "worker failed"
	LogAllWorkersDone = "all workers done"
	LogCircuitDoneErr = "circuit done with error"
	LogProcessing     = "processing"
	LogCLIFailed      = "CLI failed"
	LogSubmitRejected = "submit_step rejected"
	LogSubmitted      = "submitted"

	// Ingest
	LogIngestComplete       = "ingest complete"
	LogVerifierFailed       = "verifier failed"
	LogCandidateNotVerified = "candidate not verified"
	LogVerificationComplete = "verification complete"
	LogPromotionComplete    = "promotion complete"

	// Curate
	LogSourceFetchFailed = "source fetch failed"
	LogExtractorFailed   = "extractor failed"
	LogRecordPromoted    = "record promoted"

	// Toolkit
	LogTuningParseFailed = "failed to parse tuning-quickwins YAML"
	LogTuningQWSkipped   = "tuning QW skipped (not yet implemented)"
	LogTuningQWFailed    = "tuning QW apply failed"

	// Instrument
	LogInstrumentDispatching = "instrument dispatching"
	LogInstrumentCompleted   = "instrument completed"
	LogInstrumentFailed      = "instrument failed"

	// Tune — preflight verification
	LogTuneAllStarted   = "tune all started"
	LogTuneAllCompleted = "tune all completed"
	LogTuneStarted      = "tune started"
	LogTuneCompleted    = "tune completed"
	LogTuneFailed       = "tune failed"
)

// Log field keys — used as slog attribute keys.
// Consistent naming across all subsystems.
const (
	// --- Core (shared across subsystems) ---
	LogKeyComponent  = "component"
	LogKeyNode       = "node"
	LogKeyEdge       = "edge"
	LogKeyFrom       = "from"
	LogKeyTo         = "to"
	LogKeyWalker     = "walker"
	LogKeyElapsed    = "elapsed_ms"
	LogKeyLoop       = "loop"
	LogKeyShortcut   = "shortcut"
	LogKeyCount      = "count"
	LogKeyError      = "error"
	LogKeyCaseID     = "case_id"
	LogKeyCircuit    = "circuit"
	LogKeyExpression = "expression"
	LogKeyResult     = "result"

	// --- Dispatch ---
	LogKeyDispatchID       = "dispatch_id"
	LogKeyStep             = "step"
	LogKeyStatus           = "status"
	LogKeyBytes            = "bytes"
	LogKeyPendingCount     = "pending_count"
	LogKeyActiveDispatches = "active_dispatches"
	LogKeyArtifactBytes    = "artifact_bytes"
	LogKeyTimeout          = "timeout"
	LogKeyPhase            = "phase"
	LogKeyBatchID          = "batch_id"
	LogKeySignals          = "signals"
	LogKeyPoll             = "poll"
	LogKeyArtifactPath     = "artifact_path"
	LogKeySignalPath       = "signal_path"
	LogKeyLatency          = "latency_ms"
	LogKeyPolls            = "polls"
	LogKeyBadReadStreak    = "bad_read_streak"
	LogKeyMax              = "max"
	LogKeyWant             = "want"
	LogKeyGot              = "got"
	LogKeyStaleStreak      = "stale_streak"

	// --- Worker ---
	LogKeyWorkerID = "worker_id"
	LogKeyWorker   = "worker"
	LogKeyRole     = "role"

	// --- Session / MCP ---
	LogKeySessionID  = "session_id"
	LogKeyOldID      = "old_id"
	LogKeyTotal      = "total"
	LogKeyTotalCases = "total_cases"
	LogKeyScenario   = "scenario"
	LogKeyParallel   = "parallel"
	LogKeyParams     = "params"
	LogKeyInFlight   = "in_flight"
	LogKeyDesired    = "desired"
	LogKeyDeficit    = "deficit"
	LogKeyDetail     = "detail"

	// --- Build / diagnostics ---
	LogKeyDiagnostic       = "diagnostic"
	LogKeyHook             = "hook"
	LogKeyHandler          = "handler"
	LogKeyComponents       = "components"
	LogKeyMediatorEndpoint = "mediator_endpoint"
	LogKeyTopology         = "topology"
	LogKeyCircuitsNil      = "circuits_nil"
	LogKeyCircuitsCount    = "circuits_count"
	LogKeyBaseCircuits     = "base_circuits"
	LogKeyMissing          = "missing"
	LogKeyMissingCount     = "missing_count"
	LogKeyDeclaredCount    = "declared_count"
	LogKeyAvailable        = "available"

	// --- Transformer / mediator ---
	LogKeyTransformer     = "transformer"
	LogKeyInputExpr       = "input_expr"
	LogKeyHasInput        = "has_input"
	LogKeyHasPrompt       = "has_prompt"
	LogKeyCircuitType     = "circuit_type"
	LogKeyEndpoint        = "endpoint"
	LogKeyHasRelayer      = "has_relayer"
	LogKeyChildStep       = "child_step"
	LogKeyChildCaseID     = "child_case_id"
	LogKeyChildDispatchID = "child_dispatch_id"
	LogKeyBackend         = "backend"

	// --- Ingest / curate ---
	LogKeySource     = "source"
	LogKeyRecord     = "record"
	LogKeyRecordID   = "record_id"
	LogKeyFields     = "fields"
	LogKeyVerifier   = "verifier"
	LogKeyCandidate  = "candidate"
	LogKeyReason     = "reason"
	LogKeyDiscovered = "discovered"
	LogKeyMatched    = "matched"
	LogKeyDuplicates = "duplicates"
	LogKeyVerified   = "verified"
	LogKeyPromoted   = "promoted"
	LogKeyExtractor  = "extractor"

	// --- Signal bus ---
	LogKeyEvent = "event"
	LogKeyAgent = "agent"
	LogKeyMode  = "mode"
	LogKeyIndex = "index"

	// --- Cache ---
	LogKeyCacheKey = "cache_key"

	// --- Observer / misc ---
	LogKeyPath           = "path"
	LogKeyName           = "name"
	LogKeyNames          = "names"
	LogKeyFirstError     = "first_error"
	LogKeyStepsCompleted = "steps_completed"
	LogKeyAddr           = "addr"
	LogKeyTransport      = "transport"
	LogKeyElapsedDur     = "elapsed"
	LogKeyWait           = "wait"
	LogKeyTTL            = "ttl"
	LogKeyStale          = "stale"
	LogKeyGap            = "gap"
	LogKeyMeta           = "meta"
	LogKeyRun            = "run"
	LogKeyRunDir         = "run_dir"
	LogKeyStateDir       = "state_dir"
	LogKeyFailed         = "failed"
	LogKeyWalkerID       = "walker_id"
	LogKeyQW             = "qw"
	LogKeyImport         = "import"
	LogKeyNodes          = "nodes"
	LogKeyBase           = "base"
	LogKeyBaseNodes      = "base_nodes"
	LogKeyMergedNodes    = "merged_nodes"
	LogKeyMergedEdges    = "merged_edges"
	LogKeyOverlayNodes   = "overlay_nodes"
	LogKeyOverlayEdges   = "overlay_edges"
	LogKeyStart          = "start"
	LogKeyDone           = "done"

	// --- Embedding / memory ---
	LogKeyNamespace  = "namespace"
	LogKeyQuery      = "query"
	LogKeySimilarity = "similarity"
	LogKeyResults    = "results"

	// --- Instrument ---
	LogKeyInstrument   = "instrument"
	LogKeyAction       = "action"
	LogKeyDispatchMode = "dispatch_mode"
	LogKeyCommand      = "command"
)
