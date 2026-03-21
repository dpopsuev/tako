package mcp

import (
	"context"
	"fmt"

	framework "github.com/dpopsuev/origami"
	"github.com/dpopsuev/origami/dispatch"
)

// FieldDef describes a single field in a step's artifact schema.
type FieldDef struct {
	Name     string // field name, e.g. "confidence"
	Type     string // type hint: "string", "bool", "float", "object", "array"
	Required bool   // if true, submit_step rejects artifacts missing this field
	Desc     string // optional human-readable description
}

// StepSchema declares what a single circuit step expects in its artifact.
// Used for runtime validation in submit_step and to auto-generate worker
// prompt step-schema tables.
type StepSchema struct {
	Name string     // e.g. "F0_RECALL", "scan"
	Defs []FieldDef // structured field definitions for runtime validation
}

// ValidateFields checks that fields satisfies the schema's Defs.
func (s StepSchema) ValidateFields(fields map[string]any) error {
	for _, def := range s.Defs {
		v, ok := fields[def.Name]
		if !ok && def.Required {
			return fmt.Errorf("step %s: missing required field %q", s.Name, def.Name)
		}
		if ok && v == nil && def.Required {
			return fmt.Errorf("step %s: field %q is null", s.Name, def.Name)
		}
	}
	return nil
}

// ExtraParamDef describes one domain-specific parameter inside the
// start_circuit "extra" field. Domains register these so the MCP schema
// tells callers exactly what keys are expected.
type ExtraParamDef struct {
	Name        string   // JSON key inside extra (e.g. "scenario")
	Type        string   // JSON Schema type: "string", "integer", "boolean", "object"
	Description string   // Human-readable description shown in MCP schema
	Required    bool     // If true, start_circuit rejects calls missing this key
	Enum        []string // If non-empty, allowed values (e.g. ["offline","online"])
}

// SessionObserver receives lifecycle events from the circuit server.
// When set on CircuitConfig.Observer, the framework auto-wires
// OnStepDispatched/OnStepCompleted/OnCircuitDone/OnSessionEnd callbacks.
// Consumer-set callbacks compose (explicit callbacks still fire).
type SessionObserver interface {
	OnStepDispatched(caseID, step string)
	OnStepCompleted(caseID, step string, dispatchID int64)
	OnCircuitDone()
	OnSessionEnd()
}

// CircuitConfig is the domain-injection entry point. Implementations register
// three hooks (session creation, step schemas, report formatting) and the
// generic CircuitServer handles all protocol mechanics.
type CircuitConfig struct {
	Name    string // server implementation name (e.g. "asterisk", "achilles")
	Version string // server version (e.g. "dev")

	// StepSchemas declares the artifact schema for each circuit step.
	// The worker prompt auto-generates a step-schema table from these.
	StepSchemas []StepSchema

	// ExtraParamDefs declares the domain-specific parameters that callers
	// should pass in start_circuit's "extra" field. These are rendered into
	// the MCP tool schema so agents know what to provide.
	ExtraParamDefs []ExtraParamDef

	// WorkerPreamble is domain-specific instruction text prepended to the
	// auto-generated worker prompt. For example: "You are an Asterisk
	// calibration worker."
	WorkerPreamble string

	// CreateSession wires up a domain-specific circuit run. It receives
	// the parsed start parameters, a pre-created MuxDispatcher, and the
	// session's SignalBus for domain-specific observability signals.
	// Returns a RunFunc (executed in a goroutine), initial metadata
	// (total_cases, scenario name), and an error.
	CreateSession func(ctx context.Context, params StartParams, disp *dispatch.MuxDispatcher, bus *dispatch.SignalBus) (RunFunc, SessionMeta, error)

	// FormatReport converts domain-specific run result into human-readable
	// text and optional structured data. Called by get_report.
	FormatReport func(result any) (formatted string, structured any, err error)

	// DefaultGetNextStepTimeout is the server-side timeout for get_next_step
	// when the caller doesn't specify timeout_ms. Defaults to 10s if zero.
	DefaultGetNextStepTimeout int // milliseconds

	// DefaultSessionTTL is the inactivity TTL for sessions. When no
	// artifact submission arrives for this duration, the session aborts.
	// Defaults to 300s (5min) if zero.
	DefaultSessionTTL int // milliseconds

	// MaxSessionDuration is the absolute maximum wall-clock duration for a
	// session. When set, runCtx gets a deadline so sessions cannot run
	// forever regardless of activity. Zero means no limit (backward compatible).
	MaxSessionDuration int // milliseconds

	// OnStepDispatched is called after a step is dispatched to a worker
	// via get_next_step. Nil is safe (no-op).
	OnStepDispatched func(caseID, step string)

	// OnStepCompleted is called after a worker submits an artifact via
	// submit_step. Nil is safe (no-op).
	OnStepCompleted func(caseID, step string, dispatchID int64)

	// OnCircuitDone is called once when get_next_step returns done=true
	// for the first time. Use it to emit WalkComplete to the Kami store
	// so observers (Sumi) see the circuit as completed. Nil is safe.
	OnCircuitDone func()

	// OnSessionEnd is called when a session terminates for any reason:
	// successful completion, abort, force-replace, or TTL expiry. Use it
	// to clear stale walkers from the Kami store. Nil is safe.
	OnSessionEnd func()

	// GatewayEndpoint is the MCP gateway URL that workers connect to
	// (e.g. "http://localhost:9000/mcp"). When set, WorkerPrompt()
	// includes a Connection section so Task subagents — which don't
	// inherit project-level MCP configs — know where to connect.
	GatewayEndpoint string

	// StateDir is the base directory for persistent run data (traces,
	// artifacts, reports). When set, each session creates a run directory
	// at {StateDir}/runs/{session-id}/ with trace.jsonl, report.json,
	// and artifacts. When empty, tracing is disabled.
	StateDir string

	// Preflight is called before CreateSession to validate circuit
	// configuration. When set, start_circuit fails fast if the preflight
	// detects issues (e.g., missing transformers, broken edge conditions).
	// Defaults to nil (no preflight). Set by calibrate.WithPreflight().
	Preflight func(ctx context.Context) error

	// Observer receives lifecycle events (step dispatched/completed,
	// circuit done, session end). When set, the framework auto-wires
	// the four On* callbacks. Consumer-set callbacks compose — both
	// the observer and any explicit callback fire.
	Observer SessionObserver
}

// FindSchema returns the StepSchema for the given step name, or an error
// listing valid step names. Used by the submit_step handler.
func (c *CircuitConfig) FindSchema(step string) (StepSchema, error) {
	var names []string
	for _, s := range c.StepSchemas {
		if s.Name == step {
			return s, nil
		}
		names = append(names, s.Name)
	}
	return StepSchema{}, fmt.Errorf("unknown step %q; valid steps: %v", step, names)
}

// RunFunc is the goroutine body that runs the domain circuit. It receives
// a context (cancelled on session abort) and returns the domain result
// plus any error.
type RunFunc func(ctx context.Context) (result any, err error)

// SessionMeta carries initial metadata from the domain session factory
// back to the start_circuit response.
type SessionMeta struct {
	TotalCases int
	Scenario   string
}

// StartParams are the parsed parameters from a start_circuit tool call.
// Domain-specific fields live in Extra.
type StartParams struct {
	Parallel int
	Force    bool
	Extra    map[string]any // domain-specific params (scenario, backend, rp_base_url, etc.)

	// Observer is set by the framework when tracing is enabled (StateDir != "").
	// Domain CreateSession implementations should forward this to
	// HarnessConfig.Observer so walker-level debug events flow to the trace.
	// Consumers never set this — the framework auto-wires it.
	Observer framework.WalkObserver
}
