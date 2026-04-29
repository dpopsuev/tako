package agent

// Phase represents a stage in the Reactivity FSM.
type Phase int

const (
	Intent     Phase = iota // what should I do?
	Assess                  // what do I know?
	Plan                    // how will I do it?
	Execute                 // do it
	Assert                  // did it work?
	Retrospect              // what did I learn?
	Done                    // cycle complete
)

func (p Phase) String() string {
	names := [...]string{"intent", "assess", "plan", "execute", "assert", "retrospect", "done"}
	if int(p) < len(names) {
		return names[p]
	}
	return "unknown"
}

// Reactivity is the agent's cognitive loop FSM.
type Reactivity interface {
	Phase() Phase
	Advance() Phase
	Reset()
	IsIdle() bool
	IsBusy() bool
	IsTerminal() bool
}
