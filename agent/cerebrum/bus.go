package cerebrum

import (
	"context"
	"time"
)

type Event struct {
	ID         string
	Kind       EventKind
	Source     string
	Payload    []byte
	ToolCallID string
	Seal       bool
	CreatedAt  time.Time
}

func (e Event) IsOrgan() bool  { return e.Kind == EventOrgan }
func (e Event) IsResult() bool { return e.Kind == EventOrganResult }
func (e Event) IsError() bool  { return e.Kind == EventOrganError }

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
