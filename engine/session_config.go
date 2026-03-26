package engine

import (
	"context"
	"io/fs"

	"github.com/dpopsuev/origami/agentport"
	"github.com/dpopsuev/origami/circuit"
)

// SessionConfig is the domain-specific configuration returned by a
// consumer's CreateSession hook. It contains ONLY domain data —
// no infrastructure types (no dispatcher, no signal bus).
//
// The framework receives this and wires infrastructure (dispatch,
// bus, relayer, observer) internally based on the board's bind:
// declarations.
type SessionConfig struct {
	// CircuitDef is the parsed circuit definition for this run.
	CircuitDef *circuit.CircuitDef

	// Cases are the batch cases to process.
	Cases []BatchCase

	// Transformers maps node names to their transformer implementations.
	Transformers TransformerRegistry

	// Extractors maps node names to their extractor implementations.
	Extractors ExtractorRegistry

	// Hooks are the lifecycle hooks for the circuit run.
	Hooks HookRegistry

	// RunOptions are additional engine.Run options the consumer wants.
	RunOptions []RunOption

	// Meta carries domain metadata back to the framework
	// (e.g., total cases, scenario name).
	Meta SessionMeta

	// RunFunc, when set, replaces the bridge's default BatchWalk execution.
	// Used by calibration consumers that need the full calibrate.Run()
	// pipeline (load → walk → collect → score → report). When nil,
	// the bridge builds a RunFunc from Cases/Transformers/CircuitDef.
	RunFunc func(ctx context.Context) (any, error)
}

// SessionMeta carries initial metadata from the domain session factory
// back to the start_circuit response.
type SessionMeta struct {
	TotalCases int
	Scenario   string
}

// SessionHooks is the consumer's interface to the framework.
// Fold-generated code calls these; the consumer implements them.
//
// Unlike the old SchematicHooks, CreateSession does NOT receive
// dispatcher or signal bus — the framework wires those internally.
type SessionHooks struct {
	// CreateSession sets up domain-specific config for a circuit run.
	// Returns domain config only. The framework handles infrastructure.
	CreateSession func(ctx context.Context, params SessionParams) (*SessionConfig, error)

	// FormatReport converts domain-specific run result into
	// human-readable text and optional structured data.
	FormatReport func(result any) (formatted string, structured any, err error)
}

// SessionParams are the parsed parameters from a start_circuit tool call.
// Domain-specific fields live in Extra.
type SessionParams struct {
	Parallel int
	Force    bool
	Extra    map[string]any

	// DomainFS is the domain data filesystem (scenarios, prompts, etc.).
	// The framework injects this from CircuitConfig.DomainFS.
	DomainFS fs.FS

	// StateDir is the base directory for persistent run data.
	StateDir string

	// Observer is set by the framework for tracing. Consumers forward
	// it to RunOptions via engine.WithRunObserver(params.Observer).
	Observer circuit.WalkObserver

	// Dispatcher is the framework-created MuxDispatcher for LLM dispatch.
	// Consumers that need to create domain-specific transformers (e.g.,
	// prompt-filling transformers) use this to wire dispatch.
	// Nil when no dispatch is needed (e.g., stub/heuristic backends).
	Dispatcher agentport.Dispatcher

	// Relayer wraps the Dispatcher as a PromptRelayer for sub-circuit
	// delegation via MCPCircuitTransformer. Set by the bridge.
	Relayer PromptRelayer
}
