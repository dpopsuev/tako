package agent

import "context"

// StubReactivity is a hardcoded single-cycle FSM: Intent→Plan→Execute→Assert→Done.
type StubReactivity struct {
	phase Phase
}

var _ Reactivity = (*StubReactivity)(nil)

func (r *StubReactivity) Phase() Phase { return r.phase }

func (r *StubReactivity) Advance() Phase {
	switch r.phase {
	case Intent:
		r.phase = Plan
	case Assess:
		r.phase = Plan
	case Plan:
		r.phase = Execute
	case Execute:
		r.phase = Assert
	case Assert:
		r.phase = Retrospect
	case Retrospect:
		r.phase = Done
	}
	return r.phase
}

func (r *StubReactivity) Reset()        { r.phase = Intent }
func (r *StubReactivity) IsIdle() bool   { return r.phase == Intent || r.phase == Done }
func (r *StubReactivity) IsBusy() bool   { return !r.IsIdle() && !r.IsTerminal() }
func (r *StubReactivity) IsTerminal() bool { return false }

// StubRunner executes a single reactivity cycle — walks through all phases.
type StubRunner struct {
	Executed bool
}

var _ Runner = (*StubRunner)(nil)

func (r *StubRunner) Run(_ context.Context, agent *Agent) error {
	agent.Reactivity.Reset()
	for agent.Reactivity.Phase() != Done {
		agent.Reactivity.Advance()
	}
	r.Executed = true
	return nil
}
