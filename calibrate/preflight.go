package calibrate

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/dpopsuev/origami/circuit"
	"github.com/dpopsuev/origami/engine"
)

// PreflightReport captures structured diagnostics from a preflight check.
// On success all phases appear in Passed with nil error. On partial failure
// the report contains both Passed entries and the failing PreflightErrors.
type PreflightReport struct {
	Passed   []string
	Warnings []string
	Errors   []PreflightError
	Elapsed  time.Duration
}

// PreflightError describes a single failure during preflight.
type PreflightError struct {
	Phase  string // "validate", "components", "build", "walk"
	Detail string
}

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
// Returns a structured PreflightReport with Passed/Warnings/Errors.
// On fatal error the report is partial and error is non-nil.
func Preflight(ctx context.Context, cfg *HarnessConfig) (*PreflightReport, error) {
	start := time.Now()
	report := &PreflightReport{}

	if cfg.CircuitDef == nil {
		report.Errors = append(report.Errors, PreflightError{
			Phase:  "validate",
			Detail: "CircuitDef is required",
		})
		report.Elapsed = time.Since(start)
		return report, fmt.Errorf("preflight: CircuitDef is required")
	}

	// Step 1: Validate the CircuitDef (YAML structure, referential integrity).
	if err := cfg.CircuitDef.Validate(); err != nil {
		report.Errors = append(report.Errors, PreflightError{
			Phase:  "validate",
			Detail: err.Error(),
		})
		report.Elapsed = time.Since(start)
		return report, fmt.Errorf("preflight: circuit definition invalid: %w", err)
	}
	report.Passed = append(report.Passed, "validate")

	// Merge components into shared registries (same as Run does).
	shared := cfg.Shared
	if shared == nil {
		shared = &engine.GraphRegistries{}
	}
	if len(cfg.Components) > 0 {
		merged, err := engine.MergeComponents(shared, cfg.Components...)
		if err != nil {
			report.Errors = append(report.Errors, PreflightError{
				Phase:  "components",
				Detail: err.Error(),
			})
			report.Elapsed = time.Since(start)
			return report, fmt.Errorf("preflight: merge components: %w", err)
		}
		shared = merged
	}
	report.Passed = append(report.Passed, "components")

	// Step 2: Build the graph (handler resolution, transformer lookup, edge compilation).
	runner, err := engine.NewRunnerWith(cfg.CircuitDef, shared)
	if err != nil {
		report.Errors = append(report.Errors, PreflightError{
			Phase:  "build",
			Detail: err.Error(),
		})
		report.Elapsed = time.Since(start)
		return report, fmt.Errorf("preflight: build graph: %w", err)
	}
	report.Passed = append(report.Passed, "build")

	// TSK-206: Mediator connectivity check (warning only).
	if shared.MediatorEndpoint != "" {
		checkMediatorHealth(report, shared.MediatorEndpoint)
	}

	// Step 3: Walk the start node with a stub walker that cancels the
	// context after handling the first node. This verifies the start node
	// can be entered and its handler chain (validation, hooks) runs.
	probeCtx, probeCancel := context.WithCancel(ctx)
	defer probeCancel()

	stub := &preflightWalker{
		state:  circuit.NewWalkerState("preflight"),
		cancel: probeCancel,
	}
	walkErr := runner.Walk(probeCtx, stub, cfg.CircuitDef.Start)

	// The stub walker cancels the context after the first node, causing
	// the walk loop to return context.Canceled on the next iteration.
	// That is the expected outcome. Any other error (except Interrupt)
	// means the start node itself is broken.
	if walkErr != nil && !errors.Is(walkErr, context.Canceled) && !engine.IsInterrupt(walkErr) {
		report.Errors = append(report.Errors, PreflightError{
			Phase:  "walk",
			Detail: walkErr.Error(),
		})
		report.Elapsed = time.Since(start)
		return report, fmt.Errorf("preflight: start node walk: %w", walkErr)
	}
	report.Passed = append(report.Passed, "walk")

	report.Elapsed = time.Since(start)
	return report, nil
}

func checkMediatorHealth(report *PreflightReport, endpoint string) {
	healthURL := strings.TrimSuffix(endpoint, "/mcp") + "/healthz"
	client := &http.Client{Timeout: 2 * time.Second}
	resp, hErr := client.Get(healthURL)
	if hErr != nil {
		report.Warnings = append(report.Warnings,
			fmt.Sprintf("mediator at %s may be unreachable: %v", endpoint, hErr))
		return
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		report.Warnings = append(report.Warnings,
			fmt.Sprintf("mediator at %s returned status %d", endpoint, resp.StatusCode))
		return
	}
	report.Passed = append(report.Passed, "mediator connectivity")
}

// preflightWalker is a stub Walker that returns a minimal artifact on Handle
// and then cancels the walk context, causing a clean exit after the first node.
type preflightWalker struct {
	identity circuit.AgentIdentity
	state    *circuit.WalkerState
	cancel   context.CancelFunc
}

func (w *preflightWalker) Identity() circuit.AgentIdentity      { return w.identity }
func (w *preflightWalker) SetIdentity(id *circuit.AgentIdentity)  { w.identity = *id }
func (w *preflightWalker) State() *circuit.WalkerState           { return w.state }

func (w *preflightWalker) Handle(_ context.Context, _ circuit.Node, _ circuit.NodeContext) (circuit.Artifact, error) {
	// Cancel the context so the walk loop exits cleanly after this node.
	w.cancel()
	return &preflightArtifact{}, nil
}

// preflightArtifact is a minimal Artifact returned by the preflight walker.
type preflightArtifact struct{}

func (a *preflightArtifact) Type() string       { return "preflight" }
func (a *preflightArtifact) Confidence() float64 { return 1.0 }
func (a *preflightArtifact) Raw() any            { return nil }
