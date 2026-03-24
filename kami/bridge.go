package kami

import (
	"sync"
	"time"

	"github.com/dpopsuev/bugle/signal"
	"github.com/dpopsuev/origami/circuit"
)

// EventBridge unifies WalkObserver events and SignalBus signals into a
// single KamiEvent stream. It implements circuit.WalkObserver and
// polls a SignalBus, normalizing both into Event values broadcast to
// all subscribers.
//
// Thread-safe: multiple goroutines may call Subscribe/Unsubscribe
// while events are being emitted.
type EventBridge struct {
	mu          sync.RWMutex
	subscribers map[int]chan Event
	nextID      int
	closed      bool

	signalBus  signal.Bus
	signalIdx  int
	polling    bool
	pollDone   chan struct{}
	pollCancel chan struct{}
}

// NewEventBridge creates a bridge. If bus is non-nil, call StartPolling
// to begin draining signals. Call Close() when done.
func NewEventBridge(bus signal.Bus) *EventBridge {
	return &EventBridge{
		subscribers: make(map[int]chan Event),
		signalBus:   bus,
	}
}

// Subscribe returns a channel that receives all future events.
// The channel is buffered to avoid blocking the emitter.
// Call Unsubscribe with the returned id when done.
func (b *EventBridge) Subscribe() (id int, ch <-chan Event) {
	b.mu.Lock()
	defer b.mu.Unlock()
	c := make(chan Event, 64)
	id = b.nextID
	b.nextID++
	b.subscribers[id] = c
	return id, c
}

// Unsubscribe removes a subscriber and closes its channel.
func (b *EventBridge) Unsubscribe(id int) {
	b.mu.Lock()
	defer b.mu.Unlock()
	if c, ok := b.subscribers[id]; ok {
		close(c)
		delete(b.subscribers, id)
	}
}

// Emit broadcasts an event to all subscribers. Non-blocking: slow
// subscribers that fall behind have events dropped.
func (b *EventBridge) Emit(e Event) {
	if e.Timestamp.IsZero() {
		e.Timestamp = time.Now().UTC()
	}
	b.mu.RLock()
	defer b.mu.RUnlock()
	for _, ch := range b.subscribers {
		select {
		case ch <- e:
		default:
		}
	}
}

// OnEvent implements circuit.WalkObserver. It converts a WalkEvent
// into a KamiEvent and broadcasts it.
func (b *EventBridge) OnEvent(we circuit.WalkEvent) {
	e := Event{
		Type:      EventType(we.Type),
		Timestamp: time.Now().UTC(),
		Node:      we.Node,
		Agent:     we.Walker,
		Edge:      we.Edge,
	}
	if we.Elapsed > 0 {
		e.ElapsedMs = we.Elapsed.Milliseconds()
	}
	if we.Error != nil {
		e.Error = we.Error.Error()
	}
	if we.Metadata != nil {
		e.Data = we.Metadata
	}
	b.Emit(e)
}

// StartPolling begins polling the SignalBus for new signals in a
// background goroutine. Interval controls the poll frequency.
// Stops when Close() is called.
func (b *EventBridge) StartPolling(interval time.Duration) {
	if b.signalBus == nil {
		return
	}
	b.mu.Lock()
	b.polling = true
	b.pollDone = make(chan struct{})
	b.pollCancel = make(chan struct{})
	b.mu.Unlock()

	go func() {
		defer close(b.pollDone)
		ticker := time.NewTicker(interval)
		defer ticker.Stop()
		for {
			select {
			case <-b.pollCancel:
				return
			case <-ticker.C:
				b.drainSignals()
			}
		}
	}()
}

func (b *EventBridge) drainSignals() {
	signals := b.signalBus.Since(b.signalIdx)
	for _, sig := range signals {
		ts, _ := time.Parse(time.RFC3339, sig.Timestamp)
		if ts.IsZero() {
			ts = time.Now().UTC()
		}
		data := make(map[string]any, len(sig.Meta)+1)
		data["signal_event"] = sig.Event
		if sig.Step != "" {
			data["step"] = sig.Step
		}
		for k, v := range sig.Meta {
			data[k] = v
		}
		b.Emit(Event{
			Type:      EventSignal,
			Timestamp: ts,
			Agent:     sig.Agent,
			CaseID:    sig.CaseID,
			Data:      data,
		})
	}
	b.signalIdx += len(signals)
}

// Close stops the signal poller and closes all subscriber channels.
func (b *EventBridge) Close() {
	b.mu.Lock()
	if b.closed {
		b.mu.Unlock()
		return
	}
	b.closed = true
	polling := b.polling
	b.mu.Unlock()

	if polling {
		close(b.pollCancel)
		<-b.pollDone
	}

	b.mu.Lock()
	defer b.mu.Unlock()
	for id, ch := range b.subscribers {
		close(ch)
		delete(b.subscribers, id)
	}
}
