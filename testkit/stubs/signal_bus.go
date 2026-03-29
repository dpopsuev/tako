package stubs

import (
	"sync"
	"testing"
	"time"

	"github.com/dpopsuev/origami/agentport"
)

// StubSignalBus wraps agentport.MemBus with assertion and wait helpers.
// Thread-safe; designed for test code.
type StubSignalBus struct {
	mu     sync.Mutex
	bus    *agentport.MemBus
	events map[string]int // event name -> count
}

// NewStubSignalBus creates a StubSignalBus wrapping a new agentport.MemBus.
// It registers an onEmit callback to track events internally.
func NewStubSignalBus() *StubSignalBus {
	s := &StubSignalBus{
		bus:    agentport.NewMemBus(),
		events: make(map[string]int),
	}
	s.bus.OnEmit(func(sig agentport.Signal) {
		s.mu.Lock()
		s.events[sig.Event]++
		s.mu.Unlock()
	})
	return s
}

// Bus returns the underlying agentport.MemBus.
func (s *StubSignalBus) Bus() *agentport.MemBus {
	return s.bus
}

// Emit emits a signal on the underlying bus.
func (s *StubSignalBus) Emit(event, agent, caseID, step string, meta map[string]string) {
	s.bus.Emit(&agentport.Signal{
		Event:  event,
		Agent:  agent,
		CaseID: caseID,
		Step:   step,
		Meta:   meta,
	})
}

// AssertEventEmitted verifies that the given event was emitted at least once.
func (s *StubSignalBus) AssertEventEmitted(tb testing.TB, event string) {
	tb.Helper()
	s.mu.Lock()
	count := s.events[event]
	s.mu.Unlock()
	if count == 0 {
		tb.Errorf("expected event %q to be emitted, but it was not", event)
	}
}

// AssertEventCount verifies that the given event was emitted exactly n times.
func (s *StubSignalBus) AssertEventCount(tb testing.TB, event string, n int) {
	tb.Helper()
	s.mu.Lock()
	count := s.events[event]
	s.mu.Unlock()
	if count != n {
		tb.Errorf("expected event %q to be emitted %d times, got %d", event, n, count)
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
