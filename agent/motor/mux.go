package motor

import (
	"context"
	"sync"

	"github.com/dpopsuev/tako/agent/cerebrum"
)

type Mux struct {
	mu     sync.Mutex
	routes map[string]cerebrum.Bus
}

func NewMux() *Mux {
	return &Mux{
		routes: make(map[string]cerebrum.Bus),
	}
}

func (m *Mux) Register(kind string, bus cerebrum.Bus) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.routes[kind] = bus
}

func (m *Mux) Send(ctx context.Context, event cerebrum.Event) error {
	m.mu.Lock()
	bus, ok := m.routes[event.Kind]
	m.mu.Unlock()
	if !ok {
		return nil
	}
	return bus.Send(ctx, event)
}

func (m *Mux) Receive(_ context.Context) (cerebrum.Event, bool) {
	return cerebrum.Event{}, false
}
