package widgets

type AppendOutputMsg struct{ Line string }
type StreamTokenMsg string
type FlushStreamMsg struct{}
type ClearOutputMsg struct{}
type SetOverlayMsg struct{ Text string }

type ToolCallStartMsg struct {
	ID   string
	Name string
	Input string
}

type ToolCallResultMsg struct {
	ID     string
	Name   string
	Result string
}

type PhaseChangeMsg struct {
	Phase string
	Turn  int
}

type AgentDoneMsg struct {
	Sealed   bool
	Distance float64
	Turns    int
	OAE      float64
}

type ErrorMsg struct{ Err error }
