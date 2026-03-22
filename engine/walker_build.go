package engine

// Category: DSL & Build — walker construction from YAML definitions.

import (
	"fmt"
	"strings"

	"github.com/dpopsuev/origami/circuit"
	"github.com/dpopsuev/origami/core"
)

// validElements is the set of recognized element names for validation.
var validElements = map[core.Element]bool{
	core.ElementFire:      true,
	core.ElementLightning: true,
	core.ElementEarth:     true,
	core.ElementDiamond:   true,
	core.ElementWater:     true,
	core.ElementAir:       true,
}

// ValidateElement checks that name is a recognized element and returns it.
func ValidateElement(name string) (core.Element, error) {
	e := core.Element(strings.ToLower(name))
	if !validElements[e] {
		return "", fmt.Errorf("unknown element %q (valid: fire, lightning, earth, diamond, water, air)", name)
	}
	return e, nil
}

// BuildWalkersFromDef constructs Walker instances from YAML walker definitions.
// Each WalkerDef is resolved into a ProcessWalker by looking up the persona
// by name, overriding the element, and applying the preamble and step affinity.
func BuildWalkersFromDef(defs []circuit.WalkerDef) ([]core.Walker, error) {
	walkers := make([]core.Walker, 0, len(defs))
	for _, d := range defs {
		w, err := buildWalker(d)
		if err != nil {
			return nil, fmt.Errorf("walker %q: %w", d.Name, err)
		}
		walkers = append(walkers, w)
	}
	return walkers, nil
}

func buildWalker(d circuit.WalkerDef) (*core.ProcessWalker, error) {
	if d.Name == "" {
		return nil, fmt.Errorf("walker name is required")
	}

	id := core.AgentIdentity{}

	if d.Persona != "" {
		if core.DefaultPersonaResolver == nil {
			return nil, fmt.Errorf("persona %q requested but no persona resolver registered (import _ \"github.com/dpopsuev/origami/persona\")", d.Persona)
		}
		p, ok := core.DefaultPersonaResolver(d.Persona)
		if !ok {
			return nil, fmt.Errorf("unknown persona %q", d.Persona)
		}
		id = p.Identity
	}

	if d.Approach != "" {
		elem, ok := core.ResolveApproach(strings.ToLower(d.Approach))
		if !ok {
			return nil, fmt.Errorf("unknown approach %q", d.Approach)
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
		r := core.Role(strings.ToLower(d.Role))
		if !core.ValidRoles[r] {
			return nil, fmt.Errorf("unknown role %q (valid: worker, manager, enforcer, broker)", d.Role)
		}
		id.Role = r
	}

	return core.NewProcessWalkerWithIdentity(id, d.Name), nil
}
