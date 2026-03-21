package stubs

import (
	"sync"
	"testing"
	"time"

	"github.com/dpopsuev/origami/dispatch"
)

// StubSignalBus wraps dispatch.SignalBus with assertion and wait helpers.
// Thread-safe; designed for test code.
type StubSignalBus struct {
	mu     sync.Mutex
	bus    *dispatch.SignalBus
	events map[string]int // event name -> count
}

// NewStubSignalBus creates a StubSignalBus wrapping a new dispatch.SignalBus.
// It registers an onEmit callback to track events internally.
func NewStubSignalBus() *StubSignalBus {
	s := &StubSignalBus{
		bus:    dispatch.NewSignalBus(),
		events: make(map[string]int),
	}
	s.bus.SetOnEmit(func(sig dispatch.Signal) {
		s.mu.Lock()
		s.events[sig.Event]++
		s.mu.Unlock()
	})
	return s
}

// Bus returns the underlying dispatch.SignalBus.
func (s *StubSignalBus) Bus() *dispatch.SignalBus {
	return s.bus
}

// Emit emits a signal on the underlying bus.
func (s *StubSignalBus) Emit(event, agent, caseID, step string, meta map[string]string) {
	s.bus.Emit(event, agent, caseID, step, meta)
}

// AssertEventEmitted verifies that the given event was emitted at least once.
func (s *StubSignalBus) AssertEventEmitted(t testing.TB, event string) {
	t.Helper()
	s.mu.Lock()
	count := s.events[event]
	s.mu.Unlock()
	if count == 0 {
		t.Errorf("expected event %q to be emitted, but it was not", event)
	}
}

// AssertEventCount verifies that the given event was emitted exactly n times.
func (s *StubSignalBus) AssertEventCount(t testing.TB, event string, n int) {
	t.Helper()
	s.mu.Lock()
	count := s.events[event]
	s.mu.Unlock()
	if count != n {
		t.Errorf("expected event %q to be emitted %d times, got %d", event, n, count)
	}
}

// WaitForEvent polls until the event appears or the timeout expires.
// Returns true if the event was found, false on timeout.
func (s *StubSignalBus) WaitForEvent(event string, timeout time.Duration) bool {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		s.mu.Lock()
		count := s.events[event]
		s.mu.Unlock()
		if count > 0 {
			return true
		}
		time.Sleep(5 * time.Millisecond)
	}
	return false
}

// EventCount returns the number of times the given event was emitted.
func (s *StubSignalBus) EventCount(event string) int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.events[event]
}

// Reset clears tracked events but does not reset the underlying bus.
func (s *StubSignalBus) Reset() {
	s.mu.Lock()
	s.events = make(map[string]int)
	s.mu.Unlock()
}
