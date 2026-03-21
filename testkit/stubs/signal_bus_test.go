package stubs_test

import (
	"sync"
	"testing"
	"time"

	"github.com/dpopsuev/origami/testkit/stubs"
)

func TestStubSignalBus_AssertEventEmitted(t *testing.T) {
	bus := stubs.NewStubSignalBus()
	bus.Emit("step_start", "agent-1", "case-1", "recall", nil)

	bus.AssertEventEmitted(t, "step_start")
}

func TestStubSignalBus_AssertEventEmitted_Missing(t *testing.T) {
	bus := stubs.NewStubSignalBus()

	// Verify event count is zero, which is what AssertEventEmitted would fail on.
	if bus.EventCount("step_start") != 0 {
		t.Error("expected event count 0 for un-emitted event")
	}
}

func TestStubSignalBus_AssertEventCount(t *testing.T) {
	bus := stubs.NewStubSignalBus()
	bus.Emit("step_start", "a", "c", "s", nil)
	bus.Emit("step_start", "a", "c", "s", nil)
	bus.Emit("step_end", "a", "c", "s", nil)

	bus.AssertEventCount(t, "step_start", 2)
	bus.AssertEventCount(t, "step_end", 1)
}

func TestStubSignalBus_AssertEventCount_Mismatch(t *testing.T) {
	bus := stubs.NewStubSignalBus()
	bus.Emit("step_start", "a", "c", "s", nil)

	// Verify the count directly rather than testing assertion failure.
	if bus.EventCount("step_start") != 1 {
		t.Errorf("EventCount = %d, want 1", bus.EventCount("step_start"))
	}
}

func TestStubSignalBus_WaitForEvent(t *testing.T) {
	bus := stubs.NewStubSignalBus()

	go func() {
		time.Sleep(20 * time.Millisecond)
		bus.Emit("done", "a", "c", "s", nil)
	}()

	if !bus.WaitForEvent("done", 2*time.Second) {
		t.Error("WaitForEvent timed out, expected event to appear")
	}
}

func TestStubSignalBus_WaitForEvent_Timeout(t *testing.T) {
	bus := stubs.NewStubSignalBus()

	if bus.WaitForEvent("never", 50*time.Millisecond) {
		t.Error("WaitForEvent should have returned false on timeout")
	}
}

func TestStubSignalBus_ConcurrentEmit(t *testing.T) {
	bus := stubs.NewStubSignalBus()
	const n = 100

	var wg sync.WaitGroup
	wg.Add(n)
	for i := 0; i < n; i++ {
		go func() {
			defer wg.Done()
			bus.Emit("concurrent", "a", "c", "s", nil)
		}()
	}
	wg.Wait()

	bus.AssertEventCount(t, "concurrent", n)
}

func TestStubSignalBus_Reset(t *testing.T) {
	bus := stubs.NewStubSignalBus()
	bus.Emit("x", "a", "c", "s", nil)
	bus.Reset()

	if bus.EventCount("x") != 0 {
		t.Error("event count should be 0 after Reset")
	}
}

func TestStubSignalBus_Bus(t *testing.T) {
	bus := stubs.NewStubSignalBus()
	if bus.Bus() == nil {
		t.Error("Bus() returned nil")
	}
}
