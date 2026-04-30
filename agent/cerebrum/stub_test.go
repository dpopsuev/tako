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

type stubMotorBus struct {
	commands []Command
}

func (s *stubMotorBus) Send(_ context.Context, cmd Command) error {
	s.commands = append(s.commands, cmd)
	return nil
}

type emittingTriadReactor struct{}

func (emittingTriadReactor) React(m *reactivity.Molecule, atom reactivity.Atom) (reactivity.AssertResult, reactivity.Fortune) {
	m.Emit(reactivity.Emission{Kind: "instrument", Target: "emitted-tool", Payload: atom.Content})
	if m.Mass(reactivity.AssessmentAtom) > 0 {
		m.SealTriad(reactivity.ReasonTriad)
		m.SetPhase(reactivity.PlanAtom)
		return reactivity.Pass, reactivity.Fortune{}
	}
	if m.Phase() == reactivity.IntentAtom && m.Mass(reactivity.IntentAtom) > 0 {
		m.SetPhase(reactivity.AssessmentAtom)
		return reactivity.Pass, reactivity.Fortune{}
	}
	return reactivity.Insufficient, reactivity.Fortune{Result: reactivity.Insufficient}
}
