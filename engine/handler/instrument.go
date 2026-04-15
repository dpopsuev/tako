// Package handler defines the processing interfaces that consumers implement
// to plug into the Origami engine. Instrument, Extractor, Renderer, Hook.
package handler

import (
	"context"
	"fmt"
	"reflect"
	"strings"

	"github.com/dpopsuev/origami/circuit"
	"github.com/dpopsuev/origami/engine/trace"
)

// Instrument processes input data and produces structured output.
// Named to align with Battery's tool.Tool contract — instruments are the
// universal dispatch model (TSK-842).
type Instrument interface {
	Name() string
	Transform(ctx context.Context, tc *InstrumentContext) (any, error)
}

// DeterministicInstrument is an optional marker interface.
type DeterministicInstrument interface {
	Deterministic() bool
}

// IsDeterministic returns true if t declares itself deterministic.
func IsDeterministic(t Instrument) bool {
	if dt, ok := t.(DeterministicInstrument); ok {
		return dt.Deterministic()
	}
	return false
}

// TypedInstrument validates input types before Transform().
type TypedInstrument interface {
	Instrument
	InputType() reflect.Type
}

// StationLoggable is an optional interface for instruments that produce
// structured StationLogger data. After Transform() returns, the engine
// checks for this interface and records the log in the FlightRecorder.
// Opt-in and backward compatible.
type StationLoggable interface {
	LastStationLog() trace.StationLogger
}

// InstrumentContext carries all inputs needed by an instrument.
type InstrumentContext struct {
	Input       any
	Config      map[string]any
	Prompt      string
	NodeName    string
	NodeConfig  *circuit.NodeConfig
	Provider    string
	WalkerState *circuit.WalkerState
}

// InstrumentRegistry maps instrument names to implementations.
type InstrumentRegistry map[string]Instrument

// Get returns the instrument registered under name, or an error if not found.
func (r InstrumentRegistry) Get(name string) (Instrument, error) {
	if r == nil {
		return nil, ErrInstrumentRegistryIsNil
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
	return nil, fmt.Errorf("%w: %q not registered", ErrInstrument, name)
}

// Register adds an instrument. Panics on duplicate.
func (r InstrumentRegistry) Register(t Instrument) {
	if _, exists := r[t.Name()]; exists {
		panic(fmt.Sprintf("duplicate instrument registration: %q", t.Name()))
	}
	r[t.Name()] = t
}

// InstrumentFunc adapts a plain function into an Instrument.
func InstrumentFunc(name string, fn func(context.Context, *InstrumentContext) (any, error)) Instrument {
	return &instrumentFuncImpl{name: name, fn: fn}
}

type instrumentFuncImpl struct {
	name string
	fn   func(context.Context, *InstrumentContext) (any, error)
}

func (t *instrumentFuncImpl) Name() string { return t.name }
func (t *instrumentFuncImpl) Transform(ctx context.Context, tc *InstrumentContext) (any, error) {
	return t.fn(ctx, tc)
}
