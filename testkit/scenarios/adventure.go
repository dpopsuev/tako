package scenarios

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"github.com/dpopsuev/tako/agent/cerebrum"
	"github.com/dpopsuev/tako/instrument"
)

type InstrumentFunc func(state map[string]any, input string) string

type TimerConfig struct {
	After   time.Duration
	Event   string
	Overdue time.Duration
	Penalty string
	Mutate  func(state map[string]any)
}

type TextAdventure struct {
	mu          sync.Mutex
	state       map[string]any
	instruments map[string]*adventureInstrument
	fns         map[string]InstrumentFunc
	sensory     cerebrum.Bus
	cancel      context.CancelFunc
}

type adventureInstrument struct {
	name        string
	description string
}

var _ instrument.Shell = (*TextAdventure)(nil)

func NewTextAdventure(initialState map[string]any) *TextAdventure {
	return &TextAdventure{
		state:       initialState,
		instruments: make(map[string]*adventureInstrument),
		fns:         make(map[string]InstrumentFunc),
	}
}

func (a *TextAdventure) WithSensory(bus cerebrum.Bus) *TextAdventure {
	a.sensory = bus
	return a
}

func (a *TextAdventure) AddInstrument(name, description string, fn InstrumentFunc) {
	a.instruments[name] = &adventureInstrument{name: name, description: description}
	a.fns[name] = fn
}

func (a *TextAdventure) StartTimer(ctx context.Context, cfg TimerConfig) {
	go func() {
		select {
		case <-time.After(cfg.After):
			a.mu.Lock()
			if cfg.Mutate != nil {
				cfg.Mutate(a.state)
			}
			a.mu.Unlock()

			a.sensory.Send(ctx, cerebrum.Event{
				ID:        fmt.Sprintf("timer-%d", time.Now().UnixNano()),
				Kind:      "sensory.timer",
				Source:    "environment",
				Payload:   []byte(cfg.Event),
				CreatedAt: time.Now(),
			})

			if cfg.Overdue > 0 && cfg.Penalty != "" {
				go func() {
					select {
					case <-time.After(cfg.Overdue):
						a.mu.Lock()
						a.mu.Unlock()
						a.sensory.Send(ctx, cerebrum.Event{
							ID:        fmt.Sprintf("overdue-%d", time.Now().UnixNano()),
							Kind:      "sensory.warning",
							Source:    "environment",
							Payload:   []byte(cfg.Penalty),
							CreatedAt: time.Now(),
						})
					case <-ctx.Done():
					}
				}()
			}
		case <-ctx.Done():
		}
	}()
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

func (a *TextAdventure) Exec(ctx context.Context, name string, input json.RawMessage) (instrument.Result, error) {
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
