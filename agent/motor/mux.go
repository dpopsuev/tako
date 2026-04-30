package motor

import (
	"context"
	"sync"

	"github.com/dpopsuev/tako/agent/cerebrum"
)

type Mux struct {
	mu     sync.Mutex
	motors map[string]cerebrum.MotorBus
}

var _ cerebrum.MotorBus = (*Mux)(nil)

func NewMux() *Mux {
	return &Mux{
		motors: make(map[string]cerebrum.MotorBus),
	}
}

func (m *Mux) Register(kind string, motor cerebrum.MotorBus) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.motors[kind] = motor
}

func (m *Mux) Send(ctx context.Context, cmd cerebrum.Command) error {
	m.mu.Lock()
	adapter, ok := m.motors[cmd.Kind]
	m.mu.Unlock()
	if !ok {
		return nil
	}
	return adapter.Send(ctx, cmd)
}
