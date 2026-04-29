package minitako

type Player interface {
	Act(state GameState) Action
}

type GameInspector interface {
	Score(state GameState, action Action) float64
	OptimalAction(state GameState) Action
}

type Renderer interface {
	Render(state GameState)
}

type RandomPlayer struct{}

func (RandomPlayer) Act(state GameState) Action {
	actions := AvailableActions(&state)
	if len(actions) == 0 {
		return Idle
	}
	return actions[0]
}

type StubInspector struct{}

func (StubInspector) Score(_ GameState, _ Action) float64  { return 1.0 }
func (StubInspector) OptimalAction(state GameState) Action {
	return AvailableActions(&state)[0]
}

type StubRenderer struct{}

func (StubRenderer) Render(_ GameState) {}
