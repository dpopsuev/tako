package calibrate

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/dpopsuev/origami/circuit"
)

// Resolution defines a calibration scope level.
type Resolution string

const (
	ResolutionUnit       Resolution = "unit"       // single circuit in isolation
	ResolutionPairwise   Resolution = "pairwise"   // two circuits composed via ports
	ResolutionIntegrated Resolution = "integrated" // full end-to-end composition
)

// ParseResolution converts a string to Resolution. Returns an error for
// unrecognized values.
func ParseResolution(s string) (Resolution, error) {
	switch Resolution(s) {
	case ResolutionUnit, ResolutionPairwise, ResolutionIntegrated:
		return Resolution(s), nil
	default:
		return "", fmt.Errorf("unknown resolution %q (valid: unit, pairwise, integrated)", s)
	}
}

// MultiResolutionConfig defines a calibration plan across multiple resolutions.
type MultiResolutionConfig struct {
	Circuits []CircuitEntry   `yaml:"circuits"`
	Plans    []ResolutionPlan `yaml:"plans"`
}

// CircuitEntry names a circuit that participates in multi-resolution calibration.
type CircuitEntry struct {
	Name      string `yaml:"name"`
	Circuit   string `yaml:"circuit"`             // circuit name or import reference
	Scorecard string `yaml:"scorecard,omitempty"`
}

// ResolutionPlan describes one calibration resolution level.
type ResolutionPlan struct {
	Name       string     `yaml:"name"`
	Resolution Resolution `yaml:"resolution"`
	Circuits   []string   `yaml:"circuits"` // names from CircuitEntry
	Stubs      []StubDef  `yaml:"stubs,omitempty"`
}

// StubDef declares a port stub for isolated calibration.
// When calibrating a circuit in isolation, its port dependencies are
// replaced with canned data from the stub.
type StubDef struct {
	Port    string `yaml:"port"`    // "circuit.direction:port_name"
	Fixture string `yaml:"fixture"` // path to canned data file
}

// PortStubs maps port names (e.g. "rca.in:code-context") to loaded fixture
// data. Adapters check this map at port boundaries to short-circuit with
// canned data instead of invoking the real sub-circuit.
type PortStubs map[string]any

// LoadPortStubs loads fixture data from StubDefs. Each fixture file is
// expected to contain JSON. Returns a PortStubs map ready for injection
// into HarnessConfig or BatchCase context.
func LoadPortStubs(stubs []StubDef) (PortStubs, error) {
	if len(stubs) == 0 {
		return nil, nil
	}
	ps := make(PortStubs, len(stubs))
	for _, s := range stubs {
		data, err := os.ReadFile(s.Fixture)
		if err != nil {
			return nil, fmt.Errorf("load stub fixture %s for port %s: %w", s.Fixture, s.Port, err)
		}
		var v any
		if err := json.Unmarshal(data, &v); err != nil {
			return nil, fmt.Errorf("parse stub fixture %s: %w", s.Fixture, err)
		}
		ps[s.Port] = v
	}
	return ps, nil
}

// IsPortStubbed returns true if the given port is stubbed in the PortStubs map.
func (ps PortStubs) IsPortStubbed(port string) bool {
	if ps == nil {
		return false
	}
	_, ok := ps[port]
	return ok
}

// Get retrieves the fixture data for a port. Returns nil if not stubbed.
func (ps PortStubs) Get(port string) any {
	if ps == nil {
		return nil
	}
	return ps[port]
}

// BuildResolutionPlans generates calibration plans for a set of circuits.
// Unit plans are generated for each circuit. Pairwise plans for circuits
// that share ports. Integrated plans for the full composition.
func BuildResolutionPlans(circuits []CircuitEntry) []ResolutionPlan {
	plans := make([]ResolutionPlan, 0, len(circuits)+len(circuits)*(len(circuits)-1)/2+1)

	// Unit: each circuit independently
	for _, c := range circuits {
		plans = append(plans, ResolutionPlan{
			Name:       c.Name + "-unit",
			Resolution: ResolutionUnit,
			Circuits:   []string{c.Name},
		})
	}

	// Pairwise: each pair
	for i := 0; i < len(circuits); i++ {
		for j := i + 1; j < len(circuits); j++ {
			plans = append(plans, ResolutionPlan{
				Name:       circuits[i].Name + "-" + circuits[j].Name,
				Resolution: ResolutionPairwise,
				Circuits:   []string{circuits[i].Name, circuits[j].Name},
			})
		}
	}

	// Integrated: all circuits
	if len(circuits) > 1 {
		names := make([]string, len(circuits))
		for i, c := range circuits {
			names[i] = c.Name
		}
		plans = append(plans, ResolutionPlan{
			Name:       "integrated",
			Resolution: ResolutionIntegrated,
			Circuits:   names,
		})
	}

	return plans
}

// WrapForResolution decorates a circuit for a specific resolution level.
// It attaches resolution metadata and loaded port stubs (if any) to the
// circuit's Vars so adapters can inspect them at runtime.
func WrapForResolution(base *circuit.CircuitDef, plan *ResolutionPlan, config DecoratorConfig) *circuit.CircuitDef {
	wrapped := Wrap(base, config)

	if wrapped.Vars == nil {
		wrapped.Vars = make(map[string]any)
	}
	wrapped.Vars["_calibration_resolution"] = string(plan.Resolution)
	wrapped.Vars["_calibration_plan"] = plan.Name

	if len(plan.Stubs) > 0 {
		stubs, err := LoadPortStubs(plan.Stubs)
		if err == nil && len(stubs) > 0 {
			wrapped.Vars["_port_stubs"] = stubs
		}
	}

	return wrapped
}

// GetPortStubs retrieves the PortStubs from a circuit's Vars, if present.
// Returns nil if the circuit was not wrapped with port stubs.
func GetPortStubs(def *circuit.CircuitDef) PortStubs {
	if def == nil || def.Vars == nil {
		return nil
	}
	v, ok := def.Vars["_port_stubs"]
	if !ok {
		return nil
	}
	ps, ok := v.(PortStubs)
	if !ok {
		return nil
	}
	return ps
}

// GetResolution returns the calibration resolution from a circuit's Vars.
// Returns empty string if not set (circuit was not wrapped for resolution).
func GetResolution(def *circuit.CircuitDef) Resolution {
	if def == nil || def.Vars == nil {
		return ""
	}
	v, ok := def.Vars["_calibration_resolution"]
	if !ok {
		return ""
	}
	s, ok := v.(string)
	if !ok {
		return ""
	}
	return Resolution(s)
}
