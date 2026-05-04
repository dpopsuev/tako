package cerebrum

import (
	"github.com/dpopsuev/tako/agent/reactivity"
	tangle "github.com/dpopsuev/tangle"
)

// CompleterRouter selects a Completer based on the current phase.
type CompleterRouter interface {
	Route(phase reactivity.AtomType) tangle.Completer
}

type singleCompleter struct {
	c tangle.Completer
}

// SingleRouter returns a router that always returns the same Completer.
func SingleRouter(c tangle.Completer) CompleterRouter {
	return singleCompleter{c: c}
}

func (s singleCompleter) Route(_ reactivity.AtomType) tangle.Completer {
	return s.c
}

// PhaseRouter maps Triads to different Completers with a fallback default.
type PhaseRouter struct {
	routes   map[reactivity.Triad]tangle.Completer
	fallback tangle.Completer
}

// NewPhaseRouter creates a router with a default Completer and per-triad overrides.
func NewPhaseRouter(fallback tangle.Completer) *PhaseRouter {
	return &PhaseRouter{
		routes:   make(map[reactivity.Triad]tangle.Completer),
		fallback: fallback,
	}
}

func (r *PhaseRouter) Set(triad reactivity.Triad, c tangle.Completer) {
	r.routes[triad] = c
}

func (r *PhaseRouter) Route(phase reactivity.AtomType) tangle.Completer {
	if c, ok := r.routes[phase.Triad]; ok {
		return c
	}
	return r.fallback
}
