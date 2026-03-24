// Package assertions provides test assertion helpers for framework events,
// traces, and metrics. All functions follow the stdlib testing pattern:
// accept testing.TB, report failures via t.Errorf/t.Fatalf.
package assertions

import (
	"testing"
	"time"

	"github.com/dpopsuev/origami/agentport"
	"github.com/dpopsuev/origami/circuit"
)

// AssertEventOrder verifies that the given events contain the expected event
// types in order. Extra events between expected types are allowed (subsequence match).
func AssertEventOrder(t testing.TB, events []circuit.WalkEvent, expectedTypes []circuit.WalkEventType) {
	t.Helper()

	idx := 0
	for _, e := range events {
		if idx >= len(expectedTypes) {
			break
		}
		if e.Type == expectedTypes[idx] {
			idx++
		}
	}
	if idx < len(expectedTypes) {
		t.Errorf("event order mismatch: found %d of %d expected types; missing from index %d (%s)",
			idx, len(expectedTypes), idx, expectedTypes[idx])
	}
}

// AssertNoEvent verifies that no event of the given type exists in events.
func AssertNoEvent(t testing.TB, events []circuit.WalkEvent, eventType circuit.WalkEventType) {
	t.Helper()

	for i, e := range events {
		if e.Type == eventType {
			t.Errorf("unexpected event %s at index %d (node=%q)", eventType, i, e.Node)
			return
		}
	}
}

// WaitForSignal polls the signal bus until an event with the given name appears
// or the timeout expires. Fails the test on timeout.
func WaitForSignal(t testing.TB, bus agentport.Bus, event string, timeout time.Duration) {
	t.Helper()

	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		signals := bus.Since(0)
		for _, s := range signals {
			if s.Event == event {
				return
			}
		}
		time.Sleep(5 * time.Millisecond)
	}
	t.Errorf("timed out waiting for signal %q after %s", event, timeout)
}
