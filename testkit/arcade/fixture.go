package arcade

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"github.com/dpopsuev/tako/agent/cerebrum"
)

type FixtureMotor struct {
	mu          sync.Mutex
	instruments map[string]string
	adventure   *TextAdventure
	sensory     cerebrum.Bus
	calls       []cerebrum.Event
}

func NewFixtureMotor(instrumentNames []string, sensory cerebrum.Bus) *FixtureMotor {
	return &FixtureMotor{
		instruments: make(map[string]string),
		sensory:     sensory,
	}
}

func (f *FixtureMotor) Send(ctx context.Context, event cerebrum.Event) error {
	f.mu.Lock()
	f.calls = append(f.calls, event)
	f.mu.Unlock()

	if event.Kind == "instrument" && f.adventure != nil {
		result, _ := f.adventure.Exec(ctx, event.Source, json.RawMessage(event.Payload))
		f.sensory.Send(ctx, cerebrum.Event{
			ID:        fmt.Sprintf("fixture-%s-%d", event.Source, time.Now().UnixNano()),
			Kind:      "instrument.result",
			Source:    event.Source,
			Payload:   []byte(result.Text()),
			CreatedAt: time.Now(),
		})
	}
	return nil
}

func (f *FixtureMotor) Receive(_ context.Context) (cerebrum.Event, bool) {
	return cerebrum.Event{}, false
}

func (f *FixtureMotor) Calls() []cerebrum.Event {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]cerebrum.Event(nil), f.calls...)
}

type FixtureSignal struct {
	mu      sync.Mutex
	signals []cerebrum.Event
}

func NewFixtureSignal() *FixtureSignal {
	return &FixtureSignal{}
}

func (f *FixtureSignal) Send(_ context.Context, event cerebrum.Event) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.signals = append(f.signals, event)
	return nil
}

func (f *FixtureSignal) Receive(_ context.Context) (cerebrum.Event, bool) {
	return cerebrum.Event{}, false
}

func (f *FixtureSignal) Signals() []cerebrum.Event {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]cerebrum.Event(nil), f.signals...)
}
