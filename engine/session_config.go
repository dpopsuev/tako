package engine

import (
	"context"
	"io/fs"

	"github.com/dpopsuev/battery/tool"

	"github.com/dpopsuev/origami/agentport"
	"github.com/dpopsuev/origami/circuit"
	"github.com/dpopsuev/origami/prompt"
	"github.com/dpopsuev/origami/resource"
	"github.com/dpopsuev/origami/toolkit"
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

	// Preflight validates session prerequisites before the run starts.
	// Called after CreateSession but before the RunFunc goroutine launches.
	// Return an error to fail-fast with a clear message (e.g., missing
	// credentials, unreachable endpoints, invalid config).
	//
	// Consumers should always set this, even if just to return nil.
	// The framework logs a warning when Preflight is nil.
	Preflight func(ctx context.Context) error
}

// SessionMeta carries initial metadata from the domain session factory
// back to the start_circuit response.
type SessionMeta struct {
	TotalCases int
	Scenario   string
}

// SessionFactory creates domain-specific session configurations.
// This is the Layer 2 consumer's single entry point to the framework,
// following the net/http Handler pattern (one interface, one method).
type SessionFactory interface {
	CreateSession(ctx context.Context, params *SessionParams) (*SessionConfig, error)
}

// SessionFactoryFunc adapts a plain function as a SessionFactory,
// just as http.HandlerFunc adapts a function as http.Handler.
type SessionFactoryFunc func(ctx context.Context, params *SessionParams) (*SessionConfig, error)

// CreateSession implements SessionFactory.
func (f SessionFactoryFunc) CreateSession(ctx context.Context, params *SessionParams) (*SessionConfig, error) {
	return f(ctx, params)
}

// ReportFormatter is optionally implemented by SessionFactory implementations
// that can format domain-specific run results for human consumption.
type ReportFormatter interface {
	FormatReport(result any) (formatted string, structured any, err error)
}

// StepSchemaProvider is optionally implemented by SessionFactory implementations
// that declare artifact schemas for circuit steps.
type StepSchemaProvider interface {
	StepSchemas() []StepSchema
}

// StepSchema is a type alias for toolkit.StepSchema, re-exported here
// so consumers don't need to import toolkit/ just for step schemas.
type StepSchema = toolkit.StepSchema

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

	// PromptStore is the framework-injected prompt store. Consumers use
	// this to load/edit prompts at runtime instead of fs.ReadFile.
	// Nil when no PromptStore is configured.
	PromptStore prompt.Store

	// ResourceRegistry is the framework-injected kind registry. Consumers
	// use this to load/validate/discover any registered resource kind.
	// Nil when no ResourceRegistry is configured.
	ResourceRegistry *resource.KindRegistry

	// SubCircuitResolvers maps schematic names to their embedded circuit YAML.
	// Consumers with custom RunFunc use this to load sub-circuit definitions
	// (e.g., GND within RCA) via circuit.LoadSubCircuitsFromFS(domainFS, resolvers).
	SubCircuitResolvers map[string]circuit.AssetResolver

	// Tools is the typed tool registry from fold-generated code.
	// Consumers read connector dependencies via Tools.Get(name)
	// instead of type-asserting from Extra. Nil when no tools are wired.
	Tools *tool.Registry
}
