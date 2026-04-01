package engine

// Category: DSL & Build — walker construction from YAML definitions.

import (
	"fmt"
	"strings"

	"github.com/dpopsuev/origami/agentport"
	"github.com/dpopsuev/origami/circuit"
)

// validElements is the set of recognized element names for validation.
var validElements = map[agentport.Element]bool{
	agentport.ElementFire:      true,
	agentport.ElementLightning: true,
	agentport.ElementEarth:     true,
	agentport.ElementDiamond:   true,
	agentport.ElementWater:     true,
	agentport.ElementAir:       true,
}

// ValidateElement checks that name is a recognized element and returns it.
func ValidateElement(name string) (agentport.Element, error) {
	e := agentport.Element(strings.ToLower(name))
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

	id := agentport.AgentIdentity{}

	if d.Persona != "" {
		resolver := agentport.GetDefaultPersonaResolver()
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
		elem, ok := agentport.ResolveApproach(strings.ToLower(d.Approach))
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
		r := agentport.Role(strings.ToLower(d.Role))
		if !agentport.ValidRoles[r] {
			return nil, fmt.Errorf("%w: %q (valid: worker, manager, enforcer, broker)", ErrUnknownRole, d.Role)
		}
		id.Role = r
	}

	return circuit.NewProcessWalkerWithIdentity(&id, d.Name), nil
}
