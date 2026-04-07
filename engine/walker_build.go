package engine

// Category: DSL & Build — walker construction from YAML definitions.

import (
	"fmt"
	"strings"

	"github.com/dpopsuev/origami/circuit"
	"github.com/dpopsuev/origami/roster"
)

// validElements is the set of recognized element names for validation.
var validElements = map[roster.Element]bool{
	roster.ElementFire:      true,
	roster.ElementLightning: true,
	roster.ElementEarth:     true,
	roster.ElementDiamond:   true,
	roster.ElementWater:     true,
	roster.ElementAir:       true,
}

// ValidateElement checks that name is a recognized element and returns it.
func ValidateElement(name string) (roster.Element, error) {
	e := roster.Element(strings.ToLower(name))
	if !validElements[e] {
		return "", fmt.Errorf("%w: %q (valid: fire, lightning, earth, diamond, water, air)", ErrUnknownElement, name)
	}
	return e, nil
}

// BuildWalkersFromDef constructs Walker instances from YAML walker definitions.
// Each WalkerDef is resolved into a ProcessWalker by looking up the persona
// by name, overriding the element, and applying the preamble and step affinity.
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

	id := roster.AgentIdentity{}

	if d.Persona != "" {
		resolver := roster.GetDefaultPersonaResolver()
		if resolver == nil {
			return nil, fmt.Errorf("%w: %q requested but no persona resolver registered (import _ \"github.com/dpopsuev/origami/persona\")", ErrPersona, d.Persona)
		}
		p, ok := resolver(d.Persona)
		if !ok {
			return nil, fmt.Errorf("%w: %q", ErrUnknownPersona, d.Persona)
		}
		id = p
	}

	if d.Approach != "" {
		elem, ok := roster.ResolveApproach(strings.ToLower(d.Approach))
		if !ok {
			return nil, fmt.Errorf("%w: %q", ErrUnknownApproach, d.Approach)
		}
		id.Element = elem
	}

	if d.Preamble != "" {
		id.PromptPreamble = d.Preamble
	}

	if d.OffsetPreamble != "" {
		if id.PromptPreamble == "" {
			id.PromptPreamble = d.OffsetPreamble
		} else {
			id.PromptPreamble = id.PromptPreamble + "\n\n" + d.OffsetPreamble
		}
	}

	if len(d.StepAffinity) > 0 {
		id.StepAffinity = d.StepAffinity
	}

	if d.Role != "" {
		r := roster.Role(strings.ToLower(d.Role))
		if !roster.ValidRoles[r] {
			return nil, fmt.Errorf("%w: %q (valid: worker, manager, enforcer, broker)", ErrUnknownRole, d.Role)
		}
		id.Role = r
	}

	return circuit.NewProcessWalkerWithIdentity(&id, d.Name), nil
}
