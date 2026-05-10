package arcade

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/dpopsuev/tako/agent/cerebrum"
	"github.com/dpopsuev/tako/agent/organ"
)

type TimerConfig struct {
	After   time.Duration
	Event   string
	Overdue time.Duration
	Penalty string
	Mutate  func(state map[string]any)
}

type Game struct {
	mu      sync.Mutex
	state   map[string]any
	organs  []organ.Func
	sensory cerebrum.Bus
	cancel  context.CancelFunc
}

func NewGame(initialState map[string]any) *Game {
	return &Game{
		state: initialState,
	}
}

func (a *Game) WithSensory(bus cerebrum.Bus) *Game {
	a.sensory = bus
	return a
}

func (a *Game) Observe() string {
	a.mu.Lock()
	defer a.mu.Unlock()
	var parts []string
	for k, v := range a.state {
		parts = append(parts, fmt.Sprintf("%s: %v", k, v))
	}
	return strings.Join(parts, ". ")
}

func (a *Game) Register(f organ.Func) {
	a.organs = append(a.organs, f)
}

func (a *Game) Organ(name, description string, schema json.RawMessage, mode organ.ActionMode, fn func(state map[string]any, input json.RawMessage) (organ.Result, error)) {
	a.Register(organ.Func{
		Name:        name,
		Description: description,
		Schema:      schema,
		Mode:        mode,
		Source:      organ.Environment,
		Risk:        organRisk(mode),
		Execute: func(ctx context.Context, input json.RawMessage) (organ.Result, error) {
			a.mu.Lock()
			defer a.mu.Unlock()
			return fn(a.state, input)
		},
	})
}

func organRisk(mode organ.ActionMode) float64 {
	if mode == organ.WriteAction {
		return 0.5
	}
	return 0
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

func (a *Game) Organs() []organ.Func {
	return append([]organ.Func(nil), a.organs...)
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
