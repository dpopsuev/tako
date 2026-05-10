package organ

import (
	"context"
	"encoding/json"
)

// FuncSource identifies where a capability comes from.
type FuncSource int

const (
	BuiltIn     FuncSource = iota // agent's own: andon, recollector, compactor
	Environment                         // provided by the world: take, cook, read_file
)

func (s FuncSource) String() string {
	return [...]string{"built-in", "environment"}[s]
}

// Func is a single thing the agent can do.
// Unified: no organ/instrument/shell distinction.
// Some are built-in (agent reflexes), some come from the environment.
// Reads/Writes declare which state dimensions this capability touches
// (STRIPS-style scope declaration for GOAP-style planning).
type Func struct {
	Name        string
	Description string
	Schema      json.RawMessage
	Mode        ActionMode
	Risk        float64
	Approval    ActionApproval
	Source      FuncSource
	Reads       []string // state dimensions this capability observes
	Writes      []string // state dimensions this capability modifies
	Response    bool     // results are user-facing responses (dialog, not side-effects)
	Execute     func(ctx context.Context, input json.RawMessage) (Result, error)
}

// TouchesDimension returns true if this capability Writes to the given dimension.
func (c Func) TouchesDimension(dim string) bool {
	for _, w := range c.Writes {
		if w == dim {
			return true
		}
	}
	return false
}

// FuncSet is a registry of capabilities. Preserves insertion order.
type FuncSet struct {
	caps  map[string]Func
	order []string
}

func NewFuncSet() *FuncSet {
	return &FuncSet{caps: make(map[string]Func)}
}

func (cs *FuncSet) Register(c Func) {
	if _, exists := cs.caps[c.Name]; !exists {
		cs.order = append(cs.order, c.Name)
	}
	cs.caps[c.Name] = c
}

func (cs *FuncSet) Get(name string) (Func, bool) {
	c, ok := cs.caps[name]
	return c, ok
}

func (cs *FuncSet) Names() []string {
	return append([]string(nil), cs.order...)
}

func (cs *FuncSet) All() []Func {
	out := make([]Func, 0, len(cs.order))
	for _, name := range cs.order {
		out = append(out, cs.caps[name])
	}
	return out
}

// ForDimension returns capabilities that Write to the given state dimension.
func (cs *FuncSet) ForDimension(dim string) []Func {
	var out []Func
	for _, name := range cs.order {
		c := cs.caps[name]
		if c.TouchesDimension(dim) {
			out = append(out, c)
		}
	}
	return out
}

// Voluntary returns capabilities the LLM can call as tools.
func (cs *FuncSet) Voluntary() []Func {
	var out []Func
	for _, name := range cs.order {
		c := cs.caps[name]
		if c.Execute != nil {
			out = append(out, c)
		}
	}
	return out
}
