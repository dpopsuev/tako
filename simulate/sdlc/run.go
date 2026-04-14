package sdlc

import (
	"context"
	"fmt"
	"io/fs"
	"os"

	"github.com/dpopsuev/origami/circuit"
	"github.com/dpopsuev/origami/engine"
	"github.com/dpopsuev/origami/engine/trace"
)

// CircuitPath is the default path to the SDLC circuit YAML within DomainFS.
// Changed to v2 by TSK-772 after sub-circuit delegation is wired.
const CircuitPath = "circuits/sdlc.yaml"

// LoadCircuit reads and parses the SDLC circuit from a filesystem.
// The domainFS is typically os.DirFS(repoPath) set by the serve command.
func LoadCircuit(domainFS fs.FS) (*circuit.CircuitDef, error) {
	data, err := fs.ReadFile(domainFS, CircuitPath)
	if err != nil {
		return nil, fmt.Errorf("read circuit %s: %w", CircuitPath, err)
	}
	return circuit.LoadCircuit(data)
}

// RunConfig configures an SDLC circuit run.
type RunConfig struct {
	// Transformers is the registry of node handlers (stubs or real instruments).
	Transformers engine.TransformerRegistry

	// Recorder captures station-level events. When nil, a default is created.
	Recorder *trace.FlightRecorder

	// Parallel controls concurrent case execution. Default 1.
	Parallel int

	// DomainFS is the filesystem to load circuit YAML from.
	// When nil, defaults to os.DirFS(".").
	DomainFS fs.FS
}

// RunResult holds the output of an SDLC circuit run.
type RunResult struct {
	// WalkResults contains per-case walk results.
	WalkResults []engine.BatchWalkResult

	// Recorder is the FlightRecorder that captured the run.
	Recorder *trace.FlightRecorder
}

// Deprecated: Run uses engine.BatchWalk directly, bypassing MCP dispatch.
// Use operator.MCPActor with SessionFactory() for the production path.
// Kept for test harness use only.
func Run(ctx context.Context, cfg RunConfig) (*RunResult, error) {
	domainFS := cfg.DomainFS
	if domainFS == nil {
		domainFS = os.DirFS(".")
	}
	def, err := LoadCircuit(domainFS)
	if err != nil {
		return nil, fmt.Errorf("load sdlc circuit: %w", err)
	}

	recorder := cfg.Recorder
	if recorder == nil {
		recorder = trace.NewFlightRecorder(1000)
	}

	recorder.Record("session:start", "in", "circuit=sdlc", nil, nil)

	parallel := cfg.Parallel
	if parallel <= 0 {
		parallel = 1
	}

	shared := &engine.GraphRegistries{
		Transformers: cfg.Transformers,
	}

	// Single case — the SDLC circuit operates on one codebase per run.
	cases := []engine.BatchCase{
		{ID: "sdlc-run", Context: map[string]any{}},
	}

	recorder.Record("circuit:build", "in", "nodes=9", nil, nil)

	bwCfg := engine.BatchWalkConfig{
		Def:      def,
		Shared:   shared,
		Cases:    cases,
		Parallel: parallel,
		Observer: recorder,
	}

	results := engine.BatchWalk(ctx, bwCfg)

	recorder.Record("circuit:complete", "out", fmt.Sprintf("cases=%d", len(results)), nil, nil)

	return &RunResult{
		WalkResults: results,
		Recorder:    recorder,
	}, nil
}
