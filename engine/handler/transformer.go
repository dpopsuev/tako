// Package handler defines the processing interfaces that consumers implement
// to plug into the Origami engine. Transformer, Extractor, Renderer, Hook.
package handler

import (
	"context"
	"fmt"
	"reflect"
	"strings"

	"github.com/dpopsuev/origami/circuit"
	"github.com/dpopsuev/origami/engine/trace"
)

// Transformer processes input data and produces structured output.
type Transformer interface {
	Name() string
	Transform(ctx context.Context, tc *TransformerContext) (any, error)
}

// DeterministicTransformer is an optional marker interface.
type DeterministicTransformer interface {
	Deterministic() bool
}

// IsDeterministic returns true if t declares itself deterministic.
func IsDeterministic(t Transformer) bool {
	if dt, ok := t.(DeterministicTransformer); ok {
		return dt.Deterministic()
	}
	return false
}

// TypedTransformer validates input types before Transform().
type TypedTransformer interface {
	Transformer
	InputType() reflect.Type
}

// StationLoggable is an optional interface for transformers that produce
// structured StationLogger data. After Transform() returns, the engine
// checks for this interface and records the log in the FlightRecorder.
// Opt-in and backward compatible.
type StationLoggable interface {
	LastStationLog() trace.StationLogger
}

// TransformerContext carries all inputs needed by a transformer.
type TransformerContext struct {
	Input       any
	Config      map[string]any
	Prompt      string
	NodeName    string
	NodeConfig  *circuit.NodeConfig
	Provider    string
	WalkerState *circuit.WalkerState
}

// TransformerRegistry maps transformer names to implementations.
type TransformerRegistry map[string]Transformer

// Get returns the transformer registered under name, or an error if not found.
func (r TransformerRegistry) Get(name string) (Transformer, error) {
	if r == nil {
		return nil, ErrTransformerRegistryIsNil
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
	return nil, fmt.Errorf("%w: %q not registered", ErrTransformer, name)
}

// Register adds a transformer. Panics on duplicate.
func (r TransformerRegistry) Register(t Transformer) {
	if _, exists := r[t.Name()]; exists {
		panic(fmt.Sprintf("duplicate transformer registration: %q", t.Name()))
	}
	r[t.Name()] = t
}

// TransformerFunc adapts a plain function into a Transformer.
func TransformerFunc(name string, fn func(context.Context, *TransformerContext) (any, error)) Transformer {
	return &transformerFuncImpl{name: name, fn: fn}
}

type transformerFuncImpl struct {
	name string
	fn   func(context.Context, *TransformerContext) (any, error)
}

func (t *transformerFuncImpl) Name() string { return t.name }
func (t *transformerFuncImpl) Transform(ctx context.Context, tc *TransformerContext) (any, error) {
	return t.fn(ctx, tc)
}
