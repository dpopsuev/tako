package cerebrum

import (
	"context"
	"time"
)

type Event struct {
	ID        string
	Kind      string
	Source    string
	Payload   []byte
	CreatedAt time.Time
}

type Bus interface {
	Send(ctx context.Context, event Event) error
	Receive(ctx context.Context) (Event, bool)
}

type ChannelBus struct {
	ch chan Event
}

func NewChannelBus(size int) *ChannelBus {
	return &ChannelBus{ch: make(chan Event, size)}
}

func (b *ChannelBus) Send(ctx context.Context, event Event) error {
	select {
	case b.ch <- event:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

func (b *ChannelBus) Receive(ctx context.Context) (Event, bool) {
	select {
	case event := <-b.ch:
		return event, true
	case <-ctx.Done():
		return Event{}, false
	}
}
