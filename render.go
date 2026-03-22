package framework

// Category: DSL & Build — aliases to circuit/ package.

import "github.com/dpopsuev/origami/circuit"

// Render generates a Mermaid flowchart string from a circuit definition.
func Render(def *CircuitDef) string { return circuit.Render(def) }
