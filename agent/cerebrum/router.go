package cerebrum

import (
	"log/slog"

	"github.com/dpopsuev/tako/agent/reactivity"
	tangle "github.com/dpopsuev/tangle"
)

type CompleterRouter interface {
	Route(m *reactivity.Molecule) tangle.Completer
}

type singleCompleter struct {
	c tangle.Completer
}

func SingleRouter(c tangle.Completer) CompleterRouter {
	return singleCompleter{c: c}
}

func (s singleCompleter) Route(_ *reactivity.Molecule) tangle.Completer {
	return s.c
}

type PhaseRouter struct {
	routes   map[reactivity.Triad]tangle.Completer
	fallback tangle.Completer
}

func NewPhaseRouter(fallback tangle.Completer) *PhaseRouter {
	return &PhaseRouter{
		routes:   make(map[reactivity.Triad]tangle.Completer),
		fallback: fallback,
	}
}

func (r *PhaseRouter) Set(triad reactivity.Triad, c tangle.Completer) {
	r.routes[triad] = c
}

func (r *PhaseRouter) Route(m *reactivity.Molecule) tangle.Completer {
	if c, ok := r.routes[m.Phase().Triad]; ok {
		return c
	}
	return r.fallback
}

type AdaptiveRouter struct {
	fast     tangle.Completer
	deep     tangle.Completer
	fallback tangle.Completer
	config   *reactivity.Config
	prev     tangle.Completer
}

func NewAdaptiveRouter(fast, deep tangle.Completer, cfg *reactivity.Config) *AdaptiveRouter {
	return &AdaptiveRouter{
		fast:     fast,
		deep:     deep,
		fallback: deep,
		config:   cfg,
	}
}

func (r *AdaptiveRouter) Route(m *reactivity.Molecule) tangle.Completer {
	recollected := m.SourceMass(reactivity.Recollected)
	total := m.TotalMass()
	ratio := float64(0)
	if total > 0 {
		ratio = float64(recollected) / float64(total)
	}

	var selected tangle.Completer
	var mode string

	if ratio > r.config.RecollectionMin && m.Distance() < r.config.DistanceClose {
		selected = r.fast
		mode = "fast"
	} else if m.Distance() < r.config.DistanceClose {
		selected = r.fast
		mode = "fast"
	} else {
		selected = r.deep
		mode = "deep"
	}

	if r.prev != nil && r.prev != selected {
		slog.Warn("router.model_switch",
			slog.String("mode", mode),
			slog.Float64("distance", m.Distance()),
			slog.Float64("recollection", ratio),
			slog.String("molecule", m.ID))
	}

	slog.Info("router.selected",
		slog.String("mode", mode),
		slog.Float64("distance", m.Distance()),
		slog.Float64("recollection", ratio),
		slog.Int("turn", m.Turns()))

	r.prev = selected
	return selected
}
