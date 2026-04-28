package contracts

import (
	"testing"

	"github.com/dpopsuev/tako/mcp"
)

// RunSessionObserverContract runs the SessionObserver compliance suite.
// The factory must return a ready-to-use observer.
func RunSessionObserverContract(t *testing.T, factory func() mcp.SessionObserver) {
	t.Helper()

	t.Run("AllMethods_NoPanic", func(t *testing.T) {
		obs := factory()
		func() {
			defer func() {
				if r := recover(); r != nil {
					t.Errorf("SessionObserver method panicked: %v", r)
				}
			}()
			obs.OnStepDispatched("C01", "STEP_A")
			obs.OnStepCompleted("C01", "STEP_A", 1)
			obs.OnCircuitDone()
			obs.OnSessionEnd()
		}()
	})

	t.Run("SequentialLifecycle_NoPanic", func(t *testing.T) {
		obs := factory()
		func() {
			defer func() {
				if r := recover(); r != nil {
					t.Errorf("sequential lifecycle panicked: %v", r)
				}
			}()
			// Simulate a full session lifecycle: multiple dispatches,
			// completions, then circuit done and session end.
			obs.OnStepDispatched("C01", "STEP_A")
			obs.OnStepCompleted("C01", "STEP_A", 1)
			obs.OnStepDispatched("C01", "STEP_B")
			obs.OnStepCompleted("C01", "STEP_B", 2)
			obs.OnStepDispatched("C02", "STEP_A")
			obs.OnStepCompleted("C02", "STEP_A", 3)
			obs.OnCircuitDone()
			obs.OnSessionEnd()
		}()
	})

	t.Run("MultipleSessions_NoPanic", func(t *testing.T) {
		obs := factory()
		func() {
			defer func() {
				if r := recover(); r != nil {
					t.Errorf("multiple sessions panicked: %v", r)
				}
			}()
			// First session
			obs.OnStepDispatched("C01", "STEP_A")
			obs.OnStepCompleted("C01", "STEP_A", 1)
			obs.OnCircuitDone()
			obs.OnSessionEnd()

			// Second session (observer reuse)
			obs.OnStepDispatched("C01", "STEP_A")
			obs.OnStepCompleted("C01", "STEP_A", 2)
			obs.OnCircuitDone()
			obs.OnSessionEnd()
		}()
	})
}
