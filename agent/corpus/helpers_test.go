package corpus

import (
	"context"
	"sync"

	"github.com/dpopsuev/tako/agent/cerebrum"
)

type captureBus struct {
	mu     sync.Mutex
	events []cerebrum.Event
}

func (b *captureBus) Send(_ context.Context, event cerebrum.Event) error {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.events = append(b.events, event)
	return nil
}

func (b *captureBus) Receive(_ context.Context) (cerebrum.Event, bool) {
	return cerebrum.Event{}, false
}

func (b *captureBus) Events() []cerebrum.Event {
	b.mu.Lock()
	defer b.mu.Unlock()
	return append([]cerebrum.Event(nil), b.events...)
}
