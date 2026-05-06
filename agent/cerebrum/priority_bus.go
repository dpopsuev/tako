package cerebrum

import (
	"context"
	"sync"
)

type MotorPriority int

const (
	PriorityReflex  MotorPriority = 1
	PriorityWatcher MotorPriority = 2
	PriorityThinker MotorPriority = 3
)

type PriorityMotorBus struct {
	inner    Bus
	mu       sync.Mutex
	current  MotorPriority
}

func NewPriorityMotorBus(inner Bus) *PriorityMotorBus {
	return &PriorityMotorBus{inner: inner}
}

func (b *PriorityMotorBus) Send(ctx context.Context, event Event) error {
	return b.SendWithPriority(ctx, event, PriorityThinker)
}

func (b *PriorityMotorBus) SendWithPriority(ctx context.Context, event Event, priority MotorPriority) error {
	b.mu.Lock()
	b.current = priority
	b.mu.Unlock()
	defer func() {
		b.mu.Lock()
		b.current = 0
		b.mu.Unlock()
	}()
	return b.inner.Send(ctx, event)
}

func (b *PriorityMotorBus) Receive(ctx context.Context) (Event, bool) {
	return b.inner.Receive(ctx)
}

func (b *PriorityMotorBus) CurrentPriority() MotorPriority {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.current
}
