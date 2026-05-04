package arcade

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/dpopsuev/tako/agent/cerebrum"
	"github.com/dpopsuev/tako/agent/shell"
)

type InstrumentFunc func(state map[string]any, input string) string

type TimerConfig struct {
	After   time.Duration
	Event   string
	Overdue time.Duration
	Penalty string
	Mutate  func(state map[string]any)
}

type Game struct {
	mu          sync.Mutex
	state       map[string]any
	instruments map[string]*gameInstrument
	fns         map[string]InstrumentFunc
	sensory     cerebrum.Bus
	cancel      context.CancelFunc
}

type gameInstrument struct {
	name        string
	description string
	mode        shell.ActionMode
	approval    shell.ActionApproval
	risk        float64
}

var _ shell.Shell = (*Game)(nil)

func NewGame(initialState map[string]any) *Game {
	return &Game{
		state:       initialState,
		instruments: make(map[string]*gameInstrument),
		fns:         make(map[string]InstrumentFunc),
	}
}

func (a *Game) WithSensory(bus cerebrum.Bus) *Game {
	a.sensory = bus
	return a
}

// Observe returns the current world state as a string.
// Used to inject initial awareness before Think() starts.
func (a *Game) Observe() string {
	a.mu.Lock()
	defer a.mu.Unlock()
	var parts []string
	for k, v := range a.state {
		parts = append(parts, fmt.Sprintf("%s: %v", k, v))
	}
	return strings.Join(parts, ". ")
}

func (a *Game) AddInstrument(name, description string, mode shell.ActionMode, fn InstrumentFunc) {
	a.instruments[name] = &gameInstrument{name: name, description: description, mode: mode}
	a.fns[name] = fn
}

func (a *Game) StartTimer(ctx context.Context, cfg TimerConfig) {
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

func (a *Game) Names() []string {
	out := make([]string, 0, len(a.instruments))
	for name := range a.instruments {
		out = append(out, name)
	}
	return out
}

func (a *Game) Describe(name string) (string, error) {
	inst, ok := a.instruments[name]
	if !ok {
		return "", fmt.Errorf("unknown instrument: %s", name)
	}
	return inst.description, nil
}

func (a *Game) Mode(name string) shell.ActionMode {
	inst, ok := a.instruments[name]
	if !ok {
		return shell.ReadAction
	}
	return inst.mode
}

func (a *Game) Approval(name string) shell.ActionApproval {
	inst, ok := a.instruments[name]
	if !ok {
		return shell.Auto
	}
	return inst.approval
}

func (a *Game) Risk(name string) float64 {
	inst, ok := a.instruments[name]
	if !ok {
		return 0
	}
	if inst.risk > 0 {
		return inst.risk
	}
	if inst.mode == shell.WriteAction {
		return 0.5
	}
	return 0
}

func (a *Game) Schema(name string) (json.RawMessage, error) {
	if _, ok := a.instruments[name]; !ok {
		return nil, fmt.Errorf("unknown instrument: %s", name)
	}
	return json.RawMessage(`{"type":"string"}`), nil
}

func (a *Game) Exec(ctx context.Context, name string, input json.RawMessage) (shell.Result, error) {
	a.mu.Lock()
	defer a.mu.Unlock()

	fn, ok := a.fns[name]
	if !ok {
		return shell.ErrorResult(fmt.Sprintf("unknown instrument: %s", name)), nil
	}

	s := extractInput(input)
	result := fn(a.state, s)
	return shell.TextResult(result), nil
}

func extractInput(raw json.RawMessage) string {
	var s string
	if json.Unmarshal(raw, &s) == nil && s != "" {
		return s
	}
	var obj map[string]any
	if json.Unmarshal(raw, &obj) == nil {
		for _, v := range obj {
			if str, ok := v.(string); ok {
				return str
			}
		}
	}
	return string(raw)
}

func (a *Game) State() map[string]any {
	a.mu.Lock()
	defer a.mu.Unlock()
	cp := make(map[string]any, len(a.state))
	for k, v := range a.state {
		cp[k] = v
	}
	return cp
}
