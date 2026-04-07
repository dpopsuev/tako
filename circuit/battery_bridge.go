package circuit

// Category: Processing & Support

// Battery observer bridge — strangler fig step 1.
//
// battery/observer.Event and circuit.WalkEvent are NOT type-alias compatible:
//   - WalkEvent is circuit-walk-specific (Node, Walker, Edge, Artifact, Error as error)
//   - Battery Event is cross-component tracing (Component, Action, Server, Tool, Error as bool)
//   - Battery uses Ring+Tracer (struct-based), not an Observer interface
//   - WalkObserver is interface-based (OnEvent(*WalkEvent))
//
// Convergence path: WalkObserver stays as the circuit-level interface.
// A battery.Tracer adapter can translate WalkEvents into Battery Events
// for cross-component visibility, but that's a behavior change (BTT-TSK-14+).
//
// For now, import the package to verify the dependency compiles.

import _ "github.com/dpopsuev/battery/observer" // verify dependency wiring
