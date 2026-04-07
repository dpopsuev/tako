package def

import "testing"

// observableEventValues lists every log constant value that represents an
// observable event (decision points, state transitions). This list is the
// bridge between circuit/log.go constants and the ObservableEvents registry.
//
// If you add a new observable event constant to circuit/log.go, add its
// value here. The trap test below will fail if the registry doesn't have
// a matching entry — poka-yoke.
//
// NOT included: informational log messages (warnings, config notes, errors
// that don't represent state transitions).
var observableEventValues = []string{
	// Walk
	"node enter", "node exit", "edge taken", "no matching edge",
	"loop incremented", "walk complete", "walk error",
	"delegate start", "delegate complete",
	// DSL
	"sub-circuit loaded", "overlay merge", "overlay merge complete",
	"merge components",
	// Calibrate
	"calibration run start", "case complete", "starting run", "case walk failed",
	// Dispatch
	"dispatch begin", "dispatch round-trip", "dispatch timeout",
	"step complete", "mux dispatch registered", "mux artifact routed",
	"mux dispatcher abort",
	// Session
	"circuit session started", "circuit session failed", "circuit complete",
	"circuit run complete", "step dispatched to worker",
	"step artifact accepted", "step delivered",
	// Worker
	"workers spawned", "worker registered",
	// Transformer
	"transformer executing", "transformer completed", "transformer failed",
	// Health
	"component health check", "component healthy", "component unhealthy",
	"all components healthy",
	// Fold
	"export validation complete",
	// Signal
	"signal emitted",
	// HITL
	"inspect checkpoint", "resume walk",
}

// TestObservableEvents_AllRegistered ensures every observable event value
// has an entry in ObservableEvents. Adding an event to observableEventValues
// without registering it in observable_registry.go causes this test to fail.
func TestObservableEvents_AllRegistered(t *testing.T) {
	for _, event := range observableEventValues {
		if !ObservableEvents.Has(event) {
			t.Errorf("observable event %q has no ObservableEvents registry entry — register it in observable_registry.go", event)
		}
	}
}

// TestObservableEvents_NoStaleEntries ensures every registry entry
// corresponds to an event in the observableEventValues list. Removing
// an event without cleaning up the registry causes this test to fail.
func TestObservableEvents_NoStaleEntries(t *testing.T) {
	eventSet := make(map[string]bool, len(observableEventValues))
	for _, v := range observableEventValues {
		eventSet[v] = true
	}
	for event := range ObservableEvents {
		if !eventSet[event] {
			t.Errorf("ObservableEvents has entry %q but it's not in observableEventValues — remove stale entry or add to event list", event)
		}
	}
}

// TestObservableEvents_HasCategory ensures every event has a non-empty category.
func TestObservableEvents_HasCategory(t *testing.T) {
	for event, def := range ObservableEvents {
		if def.Category == "" {
			t.Errorf("observable event %q has empty category", event)
		}
	}
}
