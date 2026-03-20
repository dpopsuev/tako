package framework
// Category: Processing & Support

import (
	"context"
	"fmt"
	"log/slog"
	"reflect"
	"strings"
	"time"
)

// Transformer processes input data and produces structured output.
// Primary processing primitive in the Origami DSL. Built-in transformers
// (llm, http, jq, file) cover common cases; domain tools register custom
// transformers for specialized needs.
type Transformer interface {
	Name() string
	Transform(ctx context.Context, tc *TransformerContext) (any, error)
}

// DeterministicTransformer is an optional marker interface for transformers
// that declare their determinism. Deterministic transformers produce identical
// output for identical input (e.g., core.jq, core.file). Stochastic
// transformers vary per invocation (e.g., core.llm). Transformers that do not
// implement this interface are assumed stochastic (safe default).
type DeterministicTransformer interface {
	Deterministic() bool
}

// IsDeterministic returns true if t implements DeterministicTransformer and
// reports itself as deterministic. Unknown transformers default to stochastic.
func IsDeterministic(t Transformer) bool {
	if dt, ok := t.(DeterministicTransformer); ok {
		return dt.Deterministic()
	}
	return false
}

// TypedTransformer is optionally implemented by transformers that declare
// their expected input type. When set, the framework validates input types
// before calling Transform(), producing clear errors instead of nil panics.
type TypedTransformer interface {
	Transformer
	InputType() reflect.Type // expected input type; nil = accept any
}

// TransformerContext carries all inputs needed by a transformer.
type TransformerContext struct {
	Input       any            // prior node's output (or circuit input)
	Config      map[string]any // circuit vars
	Prompt      string         // prompt template path or content
	NodeName    string         // current node name
	Meta        map[string]any // additional metadata from NodeDef or walk state
	WalkerState *WalkerState   // walker state including context, outputs, and loop counts
}

// TransformerRegistry maps transformer names to implementations.
type TransformerRegistry map[string]Transformer

// Get returns the transformer registered under name, or an error if not found.
// Supports FQCN resolution: a dot-qualified name (e.g. "core.llm") does a
// direct lookup; an unqualified name tries direct first, then scans for a
// matching ".name" suffix among registered FQCNs.
func (r TransformerRegistry) Get(name string) (Transformer, error) {
	if r == nil {
		return nil, fmt.Errorf("transformer registry is nil")
	}
	if t, ok := r[name]; ok {
		return t, nil
	}
	if !strings.Contains(name, ".") {
		suffix := "." + name
		for k, t := range r {
			if strings.HasSuffix(k, suffix) {
				return t, nil
			}
		}
	}
	return nil, fmt.Errorf("transformer %q not registered", name)
}

// Register adds a transformer. Panics on duplicate.
func (r TransformerRegistry) Register(t Transformer) {
	if _, exists := r[t.Name()]; exists {
		panic(fmt.Sprintf("duplicate transformer registration: %q", t.Name()))
	}
	r[t.Name()] = t
}

// transformerNode is a Node that delegates to a Transformer.
// Created by BuildGraph when NodeDef.Transformer is set.
type transformerNode struct {
	name     string
	element  Element
	trans    Transformer
	prompt   string         // from NodeDef.Prompt
	input    string         // from NodeDef.Input (e.g. "${recall.output}")
	provider string         // from NodeDef.Provider (e.g. "cursor", "codex")
	config   map[string]any // circuit vars (from CircuitDef.Vars)
	meta     map[string]any // from NodeDef.Meta
}

func (n *transformerNode) Name() string            { return n.name }
func (n *transformerNode) ElementAffinity() Element { return n.element }

func (n *transformerNode) Process(ctx context.Context, nc NodeContext) (Artifact, error) {
	logger := slog.Default().With("component", "transformer")
	var input any

	if n.input != "" {
		resolved, err := ResolveInput(n.input, nc.WalkerState.Outputs)
		if err != nil {
			logger.Warn("input resolution failed",
				"node", n.name, "input_expr", n.input, "error", err.Error())
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
		tmplCtx := TemplateContext{
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

	meta := nc.Meta
	if meta == nil {
		meta = make(map[string]any)
	}
	for k, v := range n.meta {
		meta[k] = v
	}
	if n.provider != "" {
		meta["provider"] = n.provider
	}

	tc := &TransformerContext{
		Input:       input,
		Config:      n.config,
		Prompt:      prompt,
		NodeName:    n.name,
		Meta:        meta,
		WalkerState: nc.WalkerState,
	}

	if typed, ok := n.trans.(TypedTransformer); ok {
		if expected := typed.InputType(); expected != nil {
			if tc.Input == nil {
				return nil, fmt.Errorf("node %s: expected input type %s but got nil", tc.NodeName, expected)
			}
			actual := reflect.TypeOf(tc.Input)
			if !actual.AssignableTo(expected) {
				return nil, fmt.Errorf("node %s: input type %s not assignable to expected %s", tc.NodeName, actual, expected)
			}
		}
	}

	logger.Debug("transformer executing",
		"node", n.name, "transformer", n.trans.Name(),
		"has_input", input != nil, "has_prompt", prompt != "")

	start := time.Now()
	result, err := n.trans.Transform(ctx, tc)
	elapsed := time.Since(start)

	if err != nil {
		logger.Error("transformer failed",
			"node", n.name, "transformer", n.trans.Name(),
			"error", err.Error(), "elapsed_ms", elapsed.Milliseconds())
		return nil, fmt.Errorf("transformer %q (node %s): %w", n.trans.Name(), n.name, err)
	}

	logger.Debug("transformer completed",
		"node", n.name, "transformer", n.trans.Name(),
		"elapsed_ms", elapsed.Milliseconds())

	return &transformerArtifact{
		typeName:   n.trans.Name(),
		confidence: 1.0,
		raw:        result,
	}, nil
}

// transformerArtifact wraps transformer output as an Artifact.
type transformerArtifact struct {
	typeName   string
	confidence float64
	raw        any
}

func (a *transformerArtifact) Type() string       { return a.typeName }
func (a *transformerArtifact) Confidence() float64 { return a.confidence }
func (a *transformerArtifact) Raw() any            { return a.raw }

// TransformerFunc adapts a plain function into a Transformer.
func TransformerFunc(name string, fn func(context.Context, *TransformerContext) (any, error)) Transformer {
	return &transformerFunc{name: name, fn: fn}
}

type transformerFunc struct {
	name string
	fn   func(context.Context, *TransformerContext) (any, error)
}

func (t *transformerFunc) Name() string { return t.name }
func (t *transformerFunc) Transform(ctx context.Context, tc *TransformerContext) (any, error) {
	return t.fn(ctx, tc)
}

// Built-in transformer names recognized by resolveNode.
const (
	BuiltinTransformerGoTemplate  = "go-template"
	BuiltinTransformerPassthrough = "passthrough"
)

// goTemplateTransformer is a built-in transformer that returns the
// already-rendered prompt as its output. The transformerNode.Process()
// method renders NodeDef.Prompt via RenderPrompt() before calling
// Transform(), so this transformer just captures the rendered result.
type goTemplateTransformer struct{}

func (t *goTemplateTransformer) Name() string            { return BuiltinTransformerGoTemplate }
func (t *goTemplateTransformer) Deterministic() bool     { return true }
func (t *goTemplateTransformer) Transform(_ context.Context, tc *TransformerContext) (any, error) {
	return tc.Prompt, nil
}

// passthroughTransformer is a built-in transformer that returns its
// input unchanged. Useful for nodes that only need hooks or schema
// validation without any transformation logic.
type passthroughTransformer struct{}

func (t *passthroughTransformer) Name() string            { return BuiltinTransformerPassthrough }
func (t *passthroughTransformer) Deterministic() bool     { return true }
func (t *passthroughTransformer) Transform(_ context.Context, tc *TransformerContext) (any, error) {
	return tc.Input, nil
}

// IsTransformerNode returns true if the node was created from a transformer.
func IsTransformerNode(n Node) bool {
	_, ok := n.(*transformerNode)
	return ok
}

// TransformerNodeName resolves a transformer name, handling the "builtin:" prefix.
func TransformerNodeName(name string) string {
	return strings.TrimPrefix(name, "builtin:")
}
