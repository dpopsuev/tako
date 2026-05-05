package shell

import (
	"context"
	"encoding/json"
)

// CapabilitySource identifies where a capability comes from.
type CapabilitySource int

const (
	BuiltIn     CapabilitySource = iota // agent's own: andon, recollector, compactor
	Environment                         // provided by the world: take, cook, read_file
)

func (s CapabilitySource) String() string {
	return [...]string{"built-in", "environment"}[s]
}

// Capability is a single thing the agent can do.
// Unified: no organ/instrument/shell distinction.
// Some are built-in (agent reflexes), some come from the environment.
// Reads/Writes declare which state dimensions this capability touches
// (STRIPS-style scope declaration for GOAP-style planning).
type Capability struct {
	Name        string
	Description string
	Schema      json.RawMessage
	Mode        ActionMode
	Risk        float64
	Approval    ActionApproval
	Source      CapabilitySource
	Reads       []string // state dimensions this capability observes
	Writes      []string // state dimensions this capability modifies
	Execute     func(ctx context.Context, input json.RawMessage) (Result, error)
}

// TouchesDimension returns true if this capability Writes to the given dimension.
func (c Capability) TouchesDimension(dim string) bool {
	for _, w := range c.Writes {
		if w == dim {
			return true
		}
	}
	return false
}

// CapabilitySet is a registry of capabilities. Preserves insertion order.
type CapabilitySet struct {
	caps  map[string]Capability
	order []string
}

func NewCapabilitySet() *CapabilitySet {
	return &CapabilitySet{caps: make(map[string]Capability)}
}

func (cs *CapabilitySet) Register(c Capability) {
	if _, exists := cs.caps[c.Name]; !exists {
		cs.order = append(cs.order, c.Name)
	}
	cs.caps[c.Name] = c
}

func (cs *CapabilitySet) Get(name string) (Capability, bool) {
	c, ok := cs.caps[name]
	return c, ok
}

func (cs *CapabilitySet) Names() []string {
	return append([]string(nil), cs.order...)
}

func (cs *CapabilitySet) All() []Capability {
	out := make([]Capability, 0, len(cs.order))
	for _, name := range cs.order {
		out = append(out, cs.caps[name])
	}
	return out
}

// ForDimension returns capabilities that Write to the given state dimension.
func (cs *CapabilitySet) ForDimension(dim string) []Capability {
	var out []Capability
	for _, name := range cs.order {
		c := cs.caps[name]
		if c.TouchesDimension(dim) {
			out = append(out, c)
		}
	}
	return out
}

// Voluntary returns capabilities the LLM can call as tools.
func (cs *CapabilitySet) Voluntary() []Capability {
	var out []Capability
	for _, name := range cs.order {
		c := cs.caps[name]
		if c.Execute != nil {
			out = append(out, c)
		}
	}
	return out
}
