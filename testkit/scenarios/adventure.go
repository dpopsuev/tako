package scenarios

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"

	"github.com/dpopsuev/tako/instrument"
)

type InstrumentFunc func(state map[string]any, input string) string

type AdventureInstrument struct {
	name        string
	description string
	fn          InstrumentFunc
}

func (i *AdventureInstrument) Name() string                  { return i.name }
func (i *AdventureInstrument) Description() string           { return i.description }
func (i *AdventureInstrument) InputSchema() json.RawMessage  { return json.RawMessage(`{"type":"string"}`) }

func (i *AdventureInstrument) Execute(ctx context.Context, input json.RawMessage) (instrument.Result, error) {
	var s string
	json.Unmarshal(input, &s)
	// fn is called by TextAdventure.Exec, not directly
	return instrument.TextResult(s), nil
}

type TextAdventure struct {
	mu          sync.Mutex
	state       map[string]any
	instruments map[string]*AdventureInstrument
	fns         map[string]InstrumentFunc
}

var _ instrument.Shell = (*TextAdventure)(nil)

func NewTextAdventure(initialState map[string]any) *TextAdventure {
	return &TextAdventure{
		state:       initialState,
		instruments: make(map[string]*AdventureInstrument),
		fns:         make(map[string]InstrumentFunc),
	}
}

func (a *TextAdventure) AddInstrument(name, description string, fn InstrumentFunc) {
	a.instruments[name] = &AdventureInstrument{name: name, description: description, fn: fn}
	a.fns[name] = fn
}

func (a *TextAdventure) Names() []string {
	out := make([]string, 0, len(a.instruments))
	for name := range a.instruments {
		out = append(out, name)
	}
	return out
}

func (a *TextAdventure) Describe(name string) (string, error) {
	inst, ok := a.instruments[name]
	if !ok {
		return "", fmt.Errorf("unknown instrument: %s", name)
	}
	return inst.description, nil
}

func (a *TextAdventure) Schema(name string) (json.RawMessage, error) {
	if _, ok := a.instruments[name]; !ok {
		return nil, fmt.Errorf("unknown instrument: %s", name)
	}
	return json.RawMessage(`{"type":"string"}`), nil
}

func (a *TextAdventure) Exec(_ context.Context, name string, input json.RawMessage) (instrument.Result, error) {
	a.mu.Lock()
	defer a.mu.Unlock()

	fn, ok := a.fns[name]
	if !ok {
		return instrument.ErrorResult(fmt.Sprintf("unknown instrument: %s", name)), nil
	}

	var s string
	json.Unmarshal(input, &s)
	result := fn(a.state, s)
	return instrument.TextResult(result), nil
}

func (a *TextAdventure) State() map[string]any {
	a.mu.Lock()
	defer a.mu.Unlock()
	cp := make(map[string]any, len(a.state))
	for k, v := range a.state {
		cp[k] = v
	}
	return cp
}
