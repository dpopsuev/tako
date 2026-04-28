package stubs

import "github.com/dpopsuev/tako/circuit"

// PortStub is a test double for circuit port I/O.
type PortStub struct {
	Name      string
	Direction string
	Data      map[string]any
}

// PortStubSet builds stubs for all ports in a circuit definition.
// Provides canned data for input ports and collects output port data.
func PortStubSet(def *circuit.CircuitDef) map[string]*PortStub {
	stubs := make(map[string]*PortStub)
	for _, p := range def.Ports {
		stubs[p.Name] = &PortStub{
			Name:      p.Name,
			Direction: p.Direction,
			Data:      make(map[string]any),
		}
	}
	return stubs
}

// SetInput sets canned data for an input port stub.
func (s *PortStub) SetInput(key string, value any) {
	s.Data[key] = value
}

// Output returns collected data from an output port stub.
func (s *PortStub) Output() map[string]any {
	return s.Data
}
