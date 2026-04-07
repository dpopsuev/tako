package engine

// Category: Processing & Support — transformer types.

import (
	"context"
	"fmt"
	"log/slog"
	"reflect"
	"strings"
	"time"

	"github.com/dpopsuev/origami/circuit"
	"github.com/dpopsuev/origami/roster"
)

// Transformer, TransformerContext, TransformerRegistry are defined in engine/handler.
// This file contains the engine-internal implementation: transformerNode, builtins.

// transformerNode is a Node that delegates to a Transformer.
// Created by BuildGraph when handler_type is "transformer".
type transformerNode struct {
	name       string
	element    roster.Element
	trans      Transformer
	prompt     string              // from circuit.NodeDef.Prompt
	input      string              // from circuit.NodeDef.Input (e.g. "${recall.output}")
	provider   string              // from circuit.NodeDef.Provider (e.g. "cursor", "codex")
	config     map[string]any      // circuit vars (from circuit.CircuitDef.Vars)
	nodeConfig *circuit.NodeConfig // from NodeDef.EffectiveConfig()
}

func (n *transformerNode) Name() string                    { return n.name }
func (n *transformerNode) ElementAffinity() roster.Element { return n.element }

func (n *transformerNode) Process(ctx context.Context, nc circuit.NodeContext) (circuit.Artifact, error) {
	logger := slog.Default().With(slog.Any(circuit.LogKeyComponent, circuit.LogComponentTransform))
	var input any

	if n.input != "" {
		resolved, err := ResolveInput(n.input, nc.WalkerState.Outputs)
		if err != nil {
			logger.WarnContext(ctx, circuit.LogInputResolutionFailed, slog.Any(circuit.LogKeyNode, n.name), slog.Any(circuit.LogKeyInputExpr, n.input), slog.Any(circuit.LogKeyError, err.Error()))
			return nil, fmt.Errorf("node %s: resolve input: %w", n.name, err)
		}
		if resolved != nil {
			input = resolved.Raw()
		}
	} else if nc.PriorArtifact != nil {
		input = nc.PriorArtifact.Raw()
	}

	prompt := n.prompt
	if prompt != "" {
		sources := make(map[string]any)
		if nc.WalkerState != nil && nc.WalkerState.Outputs != nil {
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
			return nil, fmt.Errorf("node %s: render prompt: %w", n.name, err)
		}
		prompt = rendered
	}

	tc := &TransformerContext{
		Input:       input,
		Config:      n.config,
		Prompt:      prompt,
		NodeName:    n.name,
		NodeConfig:  n.nodeConfig,
		Provider:    n.provider,
		WalkerState: nc.WalkerState,
	}

	if err := checkTransformerInputType(n.trans, tc); err != nil {
		return nil, err
	}

	logger.DebugContext(ctx, circuit.LogTransformerExecuting, slog.Any(circuit.LogKeyNode, n.name), slog.Any(circuit.LogKeyTransformer, n.trans.Name()), slog.Any(circuit.LogKeyHasInput, input != nil), slog.Any(circuit.LogKeyHasPrompt, prompt != ""))

	start := time.Now()
	result, err := n.trans.Transform(ctx, tc)
	elapsed := time.Since(start)

	if err != nil {
		logger.ErrorContext(ctx, circuit.LogTransformerFailed, slog.Any(circuit.LogKeyNode, n.name), slog.Any(circuit.LogKeyTransformer, n.trans.Name()), slog.Any(circuit.LogKeyError, err.Error()), slog.Any(circuit.LogKeyElapsed, elapsed.Milliseconds()))
		return nil, fmt.Errorf("transformer %q (node %s): %w", n.trans.Name(), n.name, err)
	}

	logger.DebugContext(ctx, circuit.LogTransformerCompleted, slog.Any(circuit.LogKeyNode, n.name), slog.Any(circuit.LogKeyTransformer, n.trans.Name()), slog.Any(circuit.LogKeyElapsed, elapsed.Milliseconds()))

	return &transformerArtifact{
		typeName:   n.trans.Name(),
		confidence: 1.0,
		raw:        result,
	}, nil
}

func checkTransformerInputType(trans Transformer, tc *TransformerContext) error {
	typed, ok := trans.(TypedTransformer)
	if !ok {
		return nil
	}
	expected := typed.InputType()
	if expected == nil {
		return nil
	}
	if tc.Input == nil {
		return fmt.Errorf("%w: %s: expected input type %s but got nil", ErrNode, tc.NodeName, expected)
	}
	actual := reflect.TypeOf(tc.Input)
	if !actual.AssignableTo(expected) {
		return fmt.Errorf("%w: %s: input type %s not assignable to expected %s", ErrNode, tc.NodeName, actual, expected)
	}
	return nil
}

// transformerArtifact wraps transformer output as an Artifact.
type transformerArtifact struct {
	typeName   string
	confidence float64
	raw        any
}

func (a *transformerArtifact) Type() string        { return a.typeName }
func (a *transformerArtifact) Confidence() float64 { return a.confidence }
func (a *transformerArtifact) Raw() any            { return a.raw }

// TransformerFunc is re-exported from handler/ via handler_reexport.go.

// Built-in transformer names recognized by resolveNode.
const (
	BuiltinTransformerGoTemplate  = "go-template"
	BuiltinTransformerPassthrough = "passthrough"
)

// goTemplateTransformer is a built-in transformer that returns the
// already-rendered prompt as its output.
type goTemplateTransformer struct{}

func (t *goTemplateTransformer) Name() string        { return BuiltinTransformerGoTemplate }
func (t *goTemplateTransformer) Deterministic() bool { return true }
func (t *goTemplateTransformer) Transform(_ context.Context, tc *TransformerContext) (any, error) {
	return tc.Prompt, nil
}

// passthroughTransformer is a built-in transformer that returns its
// input unchanged.
type passthroughTransformer struct{}

func (t *passthroughTransformer) Name() string        { return BuiltinTransformerPassthrough }
func (t *passthroughTransformer) Deterministic() bool { return true }
func (t *passthroughTransformer) Transform(_ context.Context, tc *TransformerContext) (any, error) {
	return tc.Input, nil
}

// IsTransformerNode returns true if the node was created from a transformer.
func IsTransformerNode(n circuit.Node) bool {
	_, ok := n.(*transformerNode)
	return ok
}

// TransformerNodeName resolves a transformer name, handling the "builtin:" prefix.
func TransformerNodeName(name string) string {
	return strings.TrimPrefix(name, "builtin:")
}
