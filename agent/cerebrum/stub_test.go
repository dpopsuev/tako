package cerebrum

import (
	"context"

	"github.com/dpopsuev/tako/agent/reactivity"
)

type stubCompleter struct {
	response string
	err      error
}

func (s *stubCompleter) Complete(_ context.Context, _ string) (string, error) {
	return s.response, s.err
}

type stubBus struct {
	events []Event
}

var _ Bus = (*stubBus)(nil)

func (s *stubBus) Send(_ context.Context, event Event) error {
	s.events = append(s.events, event)
	return nil
}

func (s *stubBus) Receive(_ context.Context) (Event, bool) {
	return Event{}, false
}

type emittingTriadReactor struct{}

func (emittingTriadReactor) React(m *reactivity.Molecule, atom reactivity.Atom) (reactivity.YieldKind, reactivity.Yield) {
	m.Emit(reactivity.Emission{Kind: "instrument", Target: "emitted-tool", Payload: atom.Content})
	if m.Mass(reactivity.AssessmentAtom) > 0 {
		m.SealTriad(reactivity.ThinkTriad)
		m.SetPhase(reactivity.ExpansionAtom)
		return reactivity.Pass, reactivity.Yield{}
	}
	if m.Phase() == reactivity.IntentAtom && m.Mass(reactivity.IntentAtom) > 0 {
		m.SetPhase(reactivity.AssessmentAtom)
		return reactivity.Pass, reactivity.Yield{}
	}
	return reactivity.Insufficient, reactivity.Yield{Result: reactivity.Insufficient}
}
