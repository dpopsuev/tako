package dispatch

import (
	"context"
	"fmt"
	"log/slog"
	"os"

	"github.com/dpopsuev/origami/agentport"
	"github.com/dpopsuev/origami/circuit"
)

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
	broker     agentport.Broker
	bus        agentport.Bus
	role       string
	workers    int
	collective agentport.Actor
	log        *slog.Logger
}

// ACPWorkerOption configures an ACPWorkerDispatcher.
type ACPWorkerOption func(*ACPWorkerDispatcher)

// WithACPWorkerLogger sets the logger.
func WithACPWorkerLogger(l *slog.Logger) ACPWorkerOption {
	return func(d *ACPWorkerDispatcher) { d.log = l }
}

// WithACPWorkerBus attaches a signal bus for lifecycle events.
func WithACPWorkerBus(bus agentport.Bus) ACPWorkerOption {
	return func(d *ACPWorkerDispatcher) { d.bus = bus }
}

// WithACPWorkerCollective routes hard steps (investigate, review) through
// a dialectic collective instead of a single agent.
func WithACPWorkerCollective(c agentport.Actor) ACPWorkerOption {
	return func(d *ACPWorkerDispatcher) { d.collective = c }
}

// NewACPWorkerDispatcher creates a dispatcher that runs N ACP agent workers.
// Each worker pulls prompts from the MuxDispatcher, asks the agent via
// Staff.AskRole, and submits the response back.
func NewACPWorkerDispatcher(mux *MuxDispatcher, broker agentport.Broker, role string, workers int, opts ...ACPWorkerOption) *ACPWorkerDispatcher {
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
	d.emit(agentport.EventWorkerStarted, "", "", map[string]string{agentport.MetaKeyWorkerID: workerID})
	defer d.emit(agentport.EventWorkerStopped, "", "", map[string]string{agentport.MetaKeyWorkerID: workerID})

	for {
		dc, err := d.mux.GetNextStep(ctx)
		if err != nil {
			if ctx.Err() != nil {
				return nil
			}
			return fmt.Errorf("get_next_step: %w", err)
		}

		d.emit(agentport.EventWorkerStart, dc.CaseID, dc.Step, map[string]string{agentport.MetaKeyWorkerID: workerID})

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

		// Route hard steps through collective debate, others to single agent.
		var response string
		if d.collective != nil && collectiveSteps[dc.Step] {
			d.log.InfoContext(ctx, circuit.LogRoutingToCollective, slog.Any(circuit.LogKeyStep, dc.Step), slog.Any(circuit.LogKeyCaseID, dc.CaseID))
			response, err = d.collective.Perform(ctx, prompt)
		} else {
			configs, pickErr := d.broker.Pick(ctx, agentport.BrokerPrefs{Role: d.role, Count: 1})
			if pickErr != nil || len(configs) == 0 {
				d.log.ErrorContext(ctx, circuit.LogNoWorkersAvailable, slog.Any(circuit.LogKeyRole, d.role))
				continue
			}
			actor, spawnErr := d.broker.Spawn(ctx, configs[0])
			if spawnErr != nil {
				d.log.ErrorContext(ctx, circuit.LogNoWorkersAvailable, slog.Any(circuit.LogKeyRole, d.role), slog.Any(circuit.LogKeyError, spawnErr))
				continue
			}
			response, err = actor.Perform(ctx, prompt)
		}
		if err != nil {
			d.emit(agentport.EventWorkerError, dc.CaseID, dc.Step, map[string]string{
				agentport.MetaKeyWorkerID: workerID,
				agentport.MetaKeyError:    err.Error(),
			})
			d.log.ErrorContext(ctx, circuit.LogACPAgentFailed, slog.Any(circuit.LogKeyWorker, workerID), slog.Any(circuit.LogKeyCaseID, dc.CaseID), slog.Any(circuit.LogKeyStep, dc.Step), slog.Any(circuit.LogKeyError, err))
			continue
		}

		if err := d.mux.SubmitArtifact(ctx, dc.DispatchID, []byte(response)); err != nil {
			d.emit(agentport.EventWorkerError, dc.CaseID, dc.Step, map[string]string{
				agentport.MetaKeyWorkerID: workerID,
				agentport.MetaKeyError:    err.Error(),
			})
			return fmt.Errorf("submit_artifact dispatch_id=%d: %w", dc.DispatchID, err)
		}

		d.emit(agentport.EventWorkerDone, dc.CaseID, dc.Step, map[string]string{
			agentport.MetaKeyWorkerID: workerID,
			agentport.MetaKeyBytes:    fmt.Sprintf("%d", len(response)),
		})

		d.log.InfoContext(ctx, circuit.LogStepComplete, slog.Any(circuit.LogKeyWorker, workerID), slog.Any(circuit.LogKeyCaseID, dc.CaseID), slog.Any(circuit.LogKeyStep, dc.Step), slog.Any(circuit.LogKeyBytes, len(response)))
	}
}

func (d *ACPWorkerDispatcher) emit(event, caseID, step string, meta map[string]string) {
	if d.bus != nil {
		d.bus.Emit(&agentport.Signal{
			Event:  event,
			Agent:  agentport.AgentWorker,
			CaseID: caseID,
			Step:   step,
			Meta:   meta,
		})
	}
}

func readPromptFile(path string) ([]byte, error) {
	return os.ReadFile(path)
}
