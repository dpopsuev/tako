package andon

// Color represents the Andon health signal.
type Color int

const (
	Green  Color = iota // healthy
	Yellow              // warning, first pull
	Red                 // critical, second pull — may escalate to HITL
)

func (c Color) String() string {
	names := [...]string{"green", "yellow", "red"}
	if int(c) < len(names) {
		return names[c]
	}
	return "unknown"
}

// Signal is the two-pull Andon health escalation interface.
type Signal interface {
	Pull(agentID string)
	Status() Color
	Reset()
}
