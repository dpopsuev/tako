package engine

// Category: DSL & Build — instrument-based node dispatch.

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"time"

	"github.com/dpopsuev/origami/circuit"
	"github.com/dpopsuev/origami/circuit/def"
	"github.com/dpopsuev/origami/roster"
)

// InstrumentRegistry maps instrument names to their loaded manifests.
type InstrumentRegistry map[string]*circuit.InstrumentManifest

// InstrumentDispatcher executes an instrument action at runtime.
// Each DispatchMode (exec, mcp, docker, go) gets its own implementation.
type InstrumentDispatcher interface {
	Dispatch(ctx context.Context, input json.RawMessage) (json.RawMessage, error)
}

// instrumentNode implements circuit.Node by dispatching through an InstrumentDispatcher.
type instrumentNode struct {
	name       string
	element    roster.Element
	manifest   *circuit.InstrumentManifest
	actionName string
	action     def.ActionDef
	dispatcher InstrumentDispatcher
	prompt     string         // from NodeDef.Prompt
	input      string         // from NodeDef.Input (e.g. "${recall.output}")
	config     map[string]any // circuit vars
}

func (n *instrumentNode) Name() string                    { return n.name }
func (n *instrumentNode) ElementAffinity() roster.Element { return n.element }

func (n *instrumentNode) Process(ctx context.Context, nc circuit.NodeContext) (circuit.Artifact, error) {
	logger := slog.Default().With(slog.Any(circuit.LogKeyComponent, circuit.LogComponentInstrument))

	// 1. Resolve input from prior artifact or ${} expression.
	var input any
	if n.input != "" && nc.WalkerState != nil {
		resolved, err := ResolveInput(n.input, nc.WalkerState.Outputs)
		if err != nil {
			return nil, fmt.Errorf("%w: node %s: resolve input: %w", ErrInstrument, n.name, err)
		}
		if resolved != nil {
			input = resolved.Raw()
		}
	} else if nc.PriorArtifact != nil {
		input = nc.PriorArtifact.Raw()
	}

	// 2. Render prompt template if set.
	prompt := n.prompt
	if prompt != "" && nc.WalkerState != nil {
		sources := make(map[string]any)
		if nc.WalkerState.Outputs != nil {
			for k, v := range nc.WalkerState.Outputs {
				sources[k] = v.Raw()
			}
		}
		tmplCtx := circuit.TemplateContext{
			Output:  input,
			State:   nc.WalkerState,
			Config:  n.config,
			Sources: sources,
			Node:    n.name,
		}
		rendered, err := RenderPrompt(prompt, tmplCtx)
		if err != nil {
			return nil, fmt.Errorf("%w: node %s: render prompt: %w", ErrInstrument, n.name, err)
		}
		prompt = rendered
	}

	// 3. Marshal input to JSON payload for the dispatcher.
	payload, err := buildInstrumentPayload(input, prompt)
	if err != nil {
		return nil, fmt.Errorf("%w: node %s: marshal input: %w", ErrInstrument, n.name, err)
	}

	logger.DebugContext(ctx, circuit.LogInstrumentDispatching,
		slog.Any(circuit.LogKeyNode, n.name),
		slog.Any(circuit.LogKeyInstrument, n.manifest.Name),
		slog.Any(circuit.LogKeyAction, n.actionName),
		slog.Any(circuit.LogKeyDispatchMode, string(n.manifest.Dispatch)))

	// 4. Dispatch.
	start := time.Now()
	output, err := n.dispatcher.Dispatch(ctx, payload)
	elapsed := time.Since(start)

	if err != nil {
		logger.ErrorContext(ctx, circuit.LogInstrumentFailed,
			slog.Any(circuit.LogKeyNode, n.name),
			slog.Any(circuit.LogKeyInstrument, n.manifest.Name),
			slog.Any(circuit.LogKeyAction, n.actionName),
			slog.Any(circuit.LogKeyError, err.Error()),
			slog.Any(circuit.LogKeyElapsed, elapsed.Milliseconds()))
		return nil, fmt.Errorf("%w: %s/%s (node %s): %w", ErrInstrumentDispatch, n.manifest.Name, n.actionName, n.name, err)
	}

	logger.DebugContext(ctx, circuit.LogInstrumentCompleted,
		slog.Any(circuit.LogKeyNode, n.name),
		slog.Any(circuit.LogKeyInstrument, n.manifest.Name),
		slog.Any(circuit.LogKeyAction, n.actionName),
		slog.Any(circuit.LogKeyElapsed, elapsed.Milliseconds()))

	// 5. Parse output and wrap as artifact.
	raw := parseInstrumentOutput(output)

	// 6. Validate output against declared schema (if present).
	if n.action.OutputSchema != "" {
		if err := validateInstrumentOutput(output, n.action.OutputSchema); err != nil {
			return nil, fmt.Errorf("%w: %s/%s (node %s): output schema violation: %w", ErrInstrument, n.manifest.Name, n.actionName, n.name, err)
		}
	}

	return &instrumentArtifact{
		instrumentName: n.manifest.Name,
		actionName:     n.actionName,
		confidence:     1.0,
		raw:            raw,
	}, nil
}

// buildInstrumentPayload marshals input and prompt into a JSON payload.
func buildInstrumentPayload(input any, prompt string) (json.RawMessage, error) {
	if input == nil && prompt == "" {
		return json.RawMessage(`{}`), nil
	}

	payload := make(map[string]any)
	if input != nil {
		payload["input"] = input
	}
	if prompt != "" {
		payload["prompt"] = prompt
	}

	data, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}
	return data, nil
}

// parseInstrumentOutput tries to parse JSON output into a map; falls back to raw string.
func parseInstrumentOutput(output json.RawMessage) any {
	if len(output) == 0 {
		return nil
	}
	var m map[string]any
	if err := json.Unmarshal(output, &m); err == nil {
		return m
	}
	return string(output)
}

// validateInstrumentOutput checks that instrument output JSON conforms to the
// declared output_schema. Validates type match (object/array/string) and
// required fields. This is the runtime counterpart to lint rule I4.
func validateInstrumentOutput(output json.RawMessage, schema string) error {
	if len(output) == 0 {
		return nil
	}

	var schemaDef struct {
		Type       string              `json:"type"`
		Required   []string            `json:"required"`
		Properties map[string]struct{} `json:"properties"`
	}
	if err := json.Unmarshal([]byte(schema), &schemaDef); err != nil {
		return nil // malformed schema — skip validation (lint catches this at author time)
	}

	var parsed any
	if err := json.Unmarshal(output, &parsed); err != nil {
		return fmt.Errorf("%w: output is not valid JSON", ErrOutputSchemaViolation)
	}

	switch schemaDef.Type {
	case "object":
		obj, ok := parsed.(map[string]any)
		if !ok {
			return fmt.Errorf("%w: expected object, got %T", ErrOutputSchemaViolation, parsed)
		}
		for _, req := range schemaDef.Required {
			if _, exists := obj[req]; !exists {
				return fmt.Errorf("%w: missing required field %q", ErrOutputSchemaViolation, req)
			}
		}
	case "array":
		if _, ok := parsed.([]any); !ok {
			return fmt.Errorf("%w: expected array, got %T", ErrOutputSchemaViolation, parsed)
		}
	case "string":
		if _, ok := parsed.(string); !ok {
			return fmt.Errorf("%w: expected string, got %T", ErrOutputSchemaViolation, parsed)
		}
	}

	return nil
}

// instrumentArtifact wraps instrument output as a circuit.Artifact.
type instrumentArtifact struct {
	instrumentName string
	actionName     string
	confidence     float64
	raw            any
}

func (a *instrumentArtifact) Type() string {
	return "instrument:" + a.instrumentName + ":" + a.actionName
}
func (a *instrumentArtifact) Confidence() float64 { return a.confidence }
func (a *instrumentArtifact) Raw() any            { return a.raw }

// resolveInstrumentNode creates an instrumentNode from a manifest and NodeDef.
func resolveInstrumentNode(_ *circuit.CircuitDef, nd *circuit.NodeDef, manifest *circuit.InstrumentManifest, elem roster.Element, workDir string) (circuit.Node, error) {
	name := string(nd.Name)
	actionName := nd.Action
	if actionName == "" {
		actionName = name
	}

	action, err := manifest.Action(actionName)
	if err != nil {
		return nil, fmt.Errorf("%w: %q: %w", ErrInstrument, name, err)
	}

	dispatcher, err := createDispatcher(manifest, action, workDir)
	if err != nil {
		return nil, fmt.Errorf("%w: %q: %w", ErrInstrument, name, err)
	}

	return &instrumentNode{
		name:       name,
		element:    elem,
		manifest:   manifest,
		actionName: actionName,
		action:     action,
		dispatcher: dispatcher,
		prompt:     nd.Prompt,
		input:      nd.Input,
		config:     nil, // populated by caller if needed
	}, nil
}

// createDispatcher creates the appropriate InstrumentDispatcher for the manifest's dispatch mode.
// Inproc instruments are handled separately by inprocResolvers — they don't go through this path.
func createDispatcher(manifest *circuit.InstrumentManifest, action def.ActionDef, workDir string) (InstrumentDispatcher, error) {
	switch manifest.Dispatch {
	case circuit.DispatchCLI:
		return &ExecDispatcher{Binary: manifest.Binary, Command: action.Command, WorkDir: workDir}, nil
	default:
		return nil, fmt.Errorf("%w: dispatch mode %q is not supported for manifest-based dispatch", ErrInstrument, manifest.Dispatch)
	}
}
