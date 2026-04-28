package dispatch

import (
	"context"
	"fmt"
	"log/slog"
	"os"

	"github.com/dpopsuev/tako/circuit"
	"github.com/dpopsuev/tangle"
	"github.com/dpopsuev/tangle/broker"
	"github.com/dpopsuev/tangle/collective"
	"github.com/dpopsuev/tangle/signal"
)

var errNoActors = fmt.Errorf("no actors available")

// Steps where dialectic collective debate improves quality.
// These are scored by M1 (defect_type), M8 (convergence), M15 (component).
var collectiveSteps = map[string]bool{
	"investigate": true,
	"review":      true,
}

// ACPWorkerDispatcher runs N ACP agent workers that pull steps from a
// MuxDispatcher, ask an agent via the facade Staff, and submit the
// response back. Same competing-consumer pattern as CLIWorkerDispatcher
// but using bugle/acp agents instead of raw CLI subprocesses.
//
// When a collective is configured, hard steps (investigate, review)
// are routed through a dialectic debate instead of a single agent.
type ACPWorkerDispatcher struct {
	mux        *MuxDispatcher
	broker     troupe.Broker
	bus        signal.Bus
	role       string
	workers    int
	collective troupe.Actor
	strategies map[string]collective.CollectiveStrategy // step → strategy (scatter, race, etc.)
	hooks      []broker.Hook
	log        *slog.Logger
}

// ACPWorkerOption configures an ACPWorkerDispatcher.
type ACPWorkerOption func(*ACPWorkerDispatcher)

// WithACPWorkerLogger sets the logger.
func WithACPWorkerLogger(l *slog.Logger) ACPWorkerOption {
	return func(d *ACPWorkerDispatcher) { d.log = l }
}

// WithACPWorkerBus attaches a signal bus for lifecycle events.
func WithACPWorkerBus(bus signal.Bus) ACPWorkerOption {
	return func(d *ACPWorkerDispatcher) { d.bus = bus }
}

// WithACPWorkerCollective routes hard steps (investigate, review) through
// a dialectic collective instead of a single agent.
func WithACPWorkerCollective(c troupe.Actor) ACPWorkerOption {
	return func(d *ACPWorkerDispatcher) { d.collective = c }
}

// WithACPWorkerHooks registers lifecycle hooks (SpawnHook, PerformHook)
// that the broker will call on spawn/perform events.
func WithACPWorkerHooks(hooks ...broker.Hook) ACPWorkerOption {
	return func(d *ACPWorkerDispatcher) { d.hooks = append(d.hooks, hooks...) }
}

// WithACPWorkerStrategy registers a collective strategy for specific steps.
// When a step matches, the dispatcher spawns a collective with the given
// strategy instead of using a single agent. Use Scatter for fan-out-and-merge,
// Race for first-best, etc.
func WithACPWorkerStrategy(step string, strategy collective.CollectiveStrategy, agentCount int) ACPWorkerOption {
	return func(d *ACPWorkerDispatcher) {
		if d.strategies == nil {
			d.strategies = make(map[string]collective.CollectiveStrategy)
		}
		d.strategies[step] = strategy
	}
}

// NewACPWorkerDispatcher creates a dispatcher that runs N ACP agent workers.
// Each worker pulls prompts from the MuxDispatcher, asks the agent via
// Staff.AskRole, and submits the response back.
func NewACPWorkerDispatcher(mux *MuxDispatcher, broker troupe.Broker, role string, workers int, opts ...ACPWorkerOption) *ACPWorkerDispatcher {
	if workers < 1 {
		workers = 1
	}
	d := &ACPWorkerDispatcher{
		mux:     mux,
		broker:  broker,
		role:    role,
		workers: workers,
		log:     discardLogger(),
	}
	for _, o := range opts {
		o(d)
	}
	return d
}

// Run starts N worker goroutines and blocks until all complete.
func (d *ACPWorkerDispatcher) Run(ctx context.Context) error {
	return runWorkers(ctx, d.workers, "acp-worker", d.workerLoop)
}

func (d *ACPWorkerDispatcher) workerLoop(ctx context.Context, workerID string) error {
	d.emit(signal.EventWorkerStarted, "", "", map[string]string{signal.MetaKeyWorkerID: workerID})
	defer d.emit(signal.EventWorkerStopped, "", "", map[string]string{signal.MetaKeyWorkerID: workerID})

	// Actor pool: spawn once, reuse across steps.
	var actor troupe.Actor

	for {
		dc, err := d.mux.GetNextStep(ctx)
		if err != nil {
			if ctx.Err() != nil {
				return nil
			}
			return fmt.Errorf("get_next_step: %w", err)
		}

		d.emit(signal.EventWorkerStart, dc.CaseID, dc.Step, map[string]string{signal.MetaKeyWorkerID: workerID})

		// Build the prompt from content or file.
		prompt := dc.PromptContent
		if prompt == "" && dc.PromptPath != "" {
			data, readErr := readPromptFile(dc.PromptPath)
			if readErr != nil {
				d.log.ErrorContext(ctx, circuit.LogReadPromptFile, slog.Any(circuit.LogKeyWorker, workerID), slog.Any(circuit.LogKeyPath, dc.PromptPath), slog.Any(circuit.LogKeyError, readErr))
				continue
			}
			prompt = string(data)
		}

		// Route steps: strategy map → legacy collective → single agent.
		var response string
		if strategy, ok := d.strategies[dc.Step]; ok {
			d.log.InfoContext(ctx, "routing to collective strategy", slog.Any(circuit.LogKeyStep, dc.Step), slog.Any(circuit.LogKeyCaseID, dc.CaseID))
			response, err = d.executeStrategy(ctx, strategy, prompt)
		} else if d.collective != nil && collectiveSteps[dc.Step] {
			d.log.InfoContext(ctx, circuit.LogRoutingToCollective, slog.Any(circuit.LogKeyStep, dc.Step), slog.Any(circuit.LogKeyCaseID, dc.CaseID))
			response, err = d.collective.Perform(ctx, prompt)
		} else {
			// Ensure we have a healthy actor — spawn if needed, respawn if unhealthy.
			actor, err = d.ensureActor(ctx, actor, workerID)
			if err != nil {
				continue
			}
			response, err = actor.Perform(ctx, prompt)
			if err != nil {
				// Actor failed — kill and clear so next iteration respawns.
				_ = actor.Kill(ctx)
				actor = nil
			}
		}
		if err != nil {
			d.emit(signal.EventWorkerError, dc.CaseID, dc.Step, map[string]string{
				signal.MetaKeyWorkerID: workerID,
				signal.MetaKeyError:    err.Error(),
			})
			d.log.ErrorContext(ctx, circuit.LogACPAgentFailed, slog.Any(circuit.LogKeyWorker, workerID), slog.Any(circuit.LogKeyCaseID, dc.CaseID), slog.Any(circuit.LogKeyStep, dc.Step), slog.Any(circuit.LogKeyError, err))
			continue
		}

		if err := d.mux.SubmitArtifact(ctx, dc.DispatchID, []byte(response)); err != nil {
			d.emit(signal.EventWorkerError, dc.CaseID, dc.Step, map[string]string{
				signal.MetaKeyWorkerID: workerID,
				signal.MetaKeyError:    err.Error(),
			})
			return fmt.Errorf("submit_artifact dispatch_id=%d: %w", dc.DispatchID, err)
		}

		d.emit(signal.EventWorkerDone, dc.CaseID, dc.Step, map[string]string{
			signal.MetaKeyWorkerID: workerID,
			signal.MetaKeyBytes:    fmt.Sprintf("%d", len(response)),
		})

		d.log.InfoContext(ctx, circuit.LogStepComplete, slog.Any(circuit.LogKeyWorker, workerID), slog.Any(circuit.LogKeyCaseID, dc.CaseID), slog.Any(circuit.LogKeyStep, dc.Step), slog.Any(circuit.LogKeyBytes, len(response)))
	}
}

// ensureActor returns a healthy actor, spawning or respawning as needed.
func (d *ACPWorkerDispatcher) ensureActor(ctx context.Context, current troupe.Actor, workerID string) (troupe.Actor, error) {
	// Reuse if alive and ready.
	if current != nil && current.Ready() {
		return current, nil
	}

	// Kill stale actor.
	if current != nil {
		d.log.WarnContext(ctx, "agent not ready, respawning", slog.Any(circuit.LogKeyWorker, workerID))
		_ = current.Kill(ctx)
	}

	// Spawn fresh.
	configs, pickErr := d.broker.Pick(ctx, troupe.Preferences{Role: d.role, Count: 1})
	if pickErr != nil || len(configs) == 0 {
		d.log.ErrorContext(ctx, circuit.LogNoWorkersAvailable, slog.Any(circuit.LogKeyRole, d.role))
		return nil, fmt.Errorf("%w for role %s", errNoActors, d.role)
	}
	actor, spawnErr := d.broker.Spawn(ctx, configs[0])
	if spawnErr != nil {
		d.log.ErrorContext(ctx, circuit.LogNoWorkersAvailable, slog.Any(circuit.LogKeyRole, d.role), slog.Any(circuit.LogKeyError, spawnErr))
		return nil, fmt.Errorf("spawn failed: %w", spawnErr)
	}
	return actor, nil
}

const defaultScatterAgents = 3

// executeStrategy spawns a collective with the given strategy and runs it.
func (d *ACPWorkerDispatcher) executeStrategy(ctx context.Context, strategy collective.CollectiveStrategy, prompt string) (string, error) {
	coll, err := collective.SpawnCollective(ctx, d.broker, defaultScatterAgents, strategy)
	if err != nil {
		return "", fmt.Errorf("spawn collective: %w", err)
	}
	defer coll.Kill(ctx) //nolint:errcheck // best-effort cleanup
	return coll.Perform(ctx, prompt)
}

func (d *ACPWorkerDispatcher) emit(event, caseID, step string, meta map[string]string) {
	if d.bus != nil {
		d.bus.Emit(&signal.Signal{
			Event:  event,
			Agent:  signal.AgentWorker,
			CaseID: caseID,
			Step:   step,
			Meta:   meta,
		})
	}
}

func readPromptFile(path string) ([]byte, error) {
	return os.ReadFile(path)
}
