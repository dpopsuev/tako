package engine

// Category: DSL & Build — walker construction from YAML definitions.

import (
	"fmt"
	"strings"

	"github.com/dpopsuev/tako/circuit"
	"github.com/dpopsuev/tangle/visual"
)

// validElements is the set of recognized element names for validation.
var validElements = map[visual.Element]bool{
	visual.ElementFire:      true,
	visual.ElementLightning: true,
	visual.ElementEarth:     true,
	visual.ElementDiamond:   true,
	visual.ElementWater:     true,
	visual.ElementAir:       true,
}

// ValidateElement checks that name is a recognized element and returns it.
func ValidateElement(name string) (visual.Element, error) {
	e := visual.Element(strings.ToLower(name))
	if !validElements[e] {
		return "", fmt.Errorf("%w: %q (valid: fire, lightning, earth, diamond, water, air)", ErrUnknownElement, name)
	}
	return e, nil
}

// BuildWalkersFromDef constructs Walker instances from YAML walker definitions.
func BuildWalkersFromDef(defs []circuit.WalkerDef) ([]circuit.Walker, error) {
	walkers := make([]circuit.Walker, 0, len(defs))
	for i := range defs {
		w, err := buildWalker(&defs[i])
		if err != nil {
			return nil, fmt.Errorf("walker %q: %w", defs[i].Name, err)
		}
		walkers = append(walkers, w)
	}
	return walkers, nil
}

func buildWalker(d *circuit.WalkerDef) (*circuit.ProcessWalker, error) {
	if d.Name == "" {
		return nil, ErrWalkerNameIsRequired
	}

	id := circuit.AgentIdentity{
		Name: d.Name,
	}

	if d.Approach != "" {
		elem, ok := resolveApproach(strings.ToLower(d.Approach))
		if !ok {
			return nil, fmt.Errorf("%w: %q", ErrUnknownApproach, d.Approach)
		}
		id.Element = elem
	}

	if d.Role != "" {
		id.Role = strings.ToLower(d.Role)
	}

	if len(d.StepAffinity) > 0 {
		id.StepAffinity = d.StepAffinity
	}

	return circuit.NewProcessWalkerWithIdentity(&id, d.Name), nil
}
