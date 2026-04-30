package motor

import (
	"context"
	"sync"

	"github.com/dpopsuev/tako/agent/cerebrum"
)

// Mux fans out Commands to registered adapters by Kind and collects Signals.
// Implements both MotorBus and SensoryBus — single plug for Cerebrum.
type Mux struct {
	mu       sync.Mutex
	motors   map[string]cerebrum.MotorBus
	sensory  map[string]cerebrum.SensoryBus
}

var _ cerebrum.MotorBus = (*Mux)(nil)
var _ cerebrum.SensoryBus = (*Mux)(nil)

func NewMux() *Mux {
	return &Mux{
		motors:  make(map[string]cerebrum.MotorBus),
		sensory: make(map[string]cerebrum.SensoryBus),
	}
}

func (m *Mux) Register(kind string, motor cerebrum.MotorBus, sensory cerebrum.SensoryBus) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.motors[kind] = motor
	if sensory != nil {
		m.sensory[kind] = sensory
	}
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

func (m *Mux) Receive(ctx context.Context) (cerebrum.Signal, bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, s := range m.sensory {
		if sig, ok := s.Receive(ctx); ok {
			return sig, true
		}
	}
	return cerebrum.Signal{}, false
}
