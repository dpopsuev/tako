package calibrate

import (
	"context"
	"fmt"

	framework "github.com/dpopsuev/origami"
)

// Preflight performs a lightweight validation of a circuit configuration
// before running a full calibration. It catches handler resolution failures,
// missing transformers, broken edge conditions, and unresolved circuit
// references in ~2 seconds instead of discovering them after a full run.
//
// Steps:
//  1. Validate the CircuitDef (syntax, references)
//  2. Build the graph (handler resolution, transformer lookup, edge compilation)
//  3. Walk the start node with a stub walker (exits after first node)
//
// Returns nil if preflight passes, or an error describing the failure point.
func Preflight(ctx context.Context, cfg HarnessConfig) error {
	if cfg.CircuitDef == nil {
		return fmt.Errorf("preflight: CircuitDef is required")
	}

	// Step 1: Validate the CircuitDef (YAML structure, referential integrity).
	if err := cfg.CircuitDef.Validate(); err != nil {
		return fmt.Errorf("preflight: circuit definition invalid: %w", err)
	}

	// Merge components into shared registries (same as Run does).
	shared := cfg.Shared
	if len(cfg.Components) > 0 {
		merged, err := framework.MergeComponents(shared, cfg.Components...)
		if err != nil {
			return fmt.Errorf("preflight: merge components: %w", err)
		}
		shared = merged
	}

	// Step 2: Build the graph (handler resolution, transformer lookup, edge compilation).
	runner, err := framework.NewRunnerWith(cfg.CircuitDef, shared)
	if err != nil {
		return fmt.Errorf("preflight: build graph: %w", err)
	}

	// Step 3: Walk the start node with a stub walker that cancels the
	// context after handling the first node. This verifies the start node
	// can be entered and its handler chain (validation, hooks) runs.
	probeCtx, probeCancel := context.WithCancel(ctx)
	defer probeCancel()

	stub := &preflightWalker{
		state:  framework.NewWalkerState("preflight"),
		cancel: probeCancel,
	}
	walkErr := runner.Walk(probeCtx, stub, cfg.CircuitDef.Start)

	// The stub walker cancels the context after the first node, causing
	// the walk loop to return context.Canceled on the next iteration.
	// That is the expected outcome. Any other error (except Interrupt)
	// means the start node itself is broken.
	if walkErr != nil && walkErr != context.Canceled && !framework.IsInterrupt(walkErr) {
		return fmt.Errorf("preflight: start node walk: %w", walkErr)
	}

	return nil
}

// preflightWalker is a stub Walker that returns a minimal artifact on Handle
// and then cancels the walk context, causing a clean exit after the first node.
type preflightWalker struct {
	identity framework.AgentIdentity
	state    *framework.WalkerState
	cancel   context.CancelFunc
}

func (w *preflightWalker) Identity() framework.AgentIdentity      { return w.identity }
func (w *preflightWalker) SetIdentity(id framework.AgentIdentity)  { w.identity = id }
func (w *preflightWalker) State() *framework.WalkerState           { return w.state }

func (w *preflightWalker) Handle(_ context.Context, _ framework.Node, _ framework.NodeContext) (framework.Artifact, error) {
	// Cancel the context so the walk loop exits cleanly after this node.
	w.cancel()
	return &preflightArtifact{}, nil
}

// preflightArtifact is a minimal Artifact returned by the preflight walker.
type preflightArtifact struct{}

func (a *preflightArtifact) Type() string       { return "preflight" }
func (a *preflightArtifact) Confidence() float64 { return 1.0 }
func (a *preflightArtifact) Raw() any            { return nil }
