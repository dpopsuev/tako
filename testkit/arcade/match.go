package arcade

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/dpopsuev/tako/agent/cerebrum"
	"github.com/dpopsuev/tako/agent/corpus"
	"github.com/dpopsuev/tako/agent/reactivity"
	"github.com/dpopsuev/tako/service/andon"
	tangle "github.com/dpopsuev/tangle"
)

// MatchPlayer is one agent in a multi-agent Match.
type MatchPlayer struct {
	ID       string
	View     *PlayerView
	Catalyst reactivity.Catalyst
	Result   *reactivity.Molecule
	cerebrum *cerebrum.Cerebrum
}

// MatchResult captures the outcome of a multi-agent game.
type MatchResult struct {
	Players []PlayerResult
	Rounds  int
	Solved  bool
}

// PlayerResult is one player's contribution to the match.
type PlayerResult struct {
	ID       string
	Mass     int
	TokensIn int
	TokensOut int
}

// Match coordinates turn-based multi-agent games.
// Each round, every player gets one Think() call.
// The game ends when IsSolved returns true or MaxRounds is reached.
type Match struct {
	Game      *Game
	IsSolved  func(map[string]any) bool
	MaxRounds int
	players   []*MatchPlayer
}

func NewMatch(game *Game, isSolved func(map[string]any) bool, maxRounds int) *Match {
	return &Match{
		Game:      game,
		IsSolved:  isSolved,
		MaxRounds: maxRounds,
	}
}

// AddPlayer registers an agent with a restricted instrument view.
func (m *Match) AddPlayer(id string, view *PlayerView, catalyst reactivity.Catalyst) {
	m.players = append(m.players, &MatchPlayer{
		ID:       id,
		View:     view,
		Catalyst: catalyst,
	})
}

// Run executes the match with a real LLM Completer.
func (m *Match) Run(ctx context.Context, completer tangle.Completer) MatchResult {
	for _, p := range m.players {
		m.wirePlayer(p, completer)
	}

	var rounds int
	for round := 0; round < m.MaxRounds; round++ {
		rounds = round + 1
		for _, p := range m.players {
			if m.IsSolved(m.Game.State()) {
				break
			}
			slog.InfoContext(ctx, "match.turn",
				slog.String("player", p.ID),
				slog.Int("round", round))

			state := m.Game.Observe()
			cat := p.Catalyst
			cat.Need = cat.Need + "\n\nCurrent state: " + state

			if err := p.cerebrum.Think(ctx, cat); err != nil {
				slog.WarnContext(ctx, "match.think_error",
					slog.String("player", p.ID),
					slog.Any("error", err))
			}
			p.Result = p.cerebrum.Result()
		}
		if m.IsSolved(m.Game.State()) {
			break
		}
	}

	result := MatchResult{Rounds: rounds, Solved: m.IsSolved(m.Game.State())}
	for _, p := range m.players {
		pr := PlayerResult{ID: p.ID}
		if p.Result != nil {
			pr.Mass = p.Result.TotalMass()
		}
		result.Players = append(result.Players, pr)
	}
	return result
}

func (m *Match) wirePlayer(p *MatchPlayer, completer tangle.Completer) {
	reactor := reactivity.NewReactor(
		reactivity.WithDirective(reactivity.ExecutionAtom,
			reactivity.Directive("Available instruments:\n"+instrumentListFromView(p.View)),
		),
	)

	sensory := cerebrum.NewChannelBus(64)
	signal := NewFixtureSignal()
	pool := &StubRecorder{}

	corp := corpus.New()
	for _, cap := range p.View.Capabilities() {
		corp.Register(cap)
	}

	var cb *cerebrum.Cerebrum
	motorBus := corp.MotorBus(sensory, signal, func() reactivity.Triad {
		if cb == nil {
			return reactivity.ThinkTriad
		}
		mol := cb.Result()
		if mol == nil {
			return reactivity.ThinkTriad
		}
		return mol.CurrentTriad()
	})

	tools := instrumentToolsFromView(p.View)

	cb = cerebrum.New(reactor, completer,
		cerebrum.WithSensory(sensory),
		cerebrum.WithMotor(motorBus),
		cerebrum.WithSignal(signal),
		cerebrum.WithRecorder(pool),
		cerebrum.WithHalter(&andon.StubSignal{}),
		cerebrum.WithCompactor(cerebrum.SummaryCompactor{}),
		cerebrum.WithBudget(cerebrum.Budget{
			MaxTurns:    15,
			TurnTimeout: 30 * time.Second,
		}),
		cerebrum.WithTools(tools),
	)

	m.Game.WithSensory(sensory)
	p.cerebrum = cb
}

func instrumentListFromView(v *PlayerView) string {
	var parts []string
	for _, name := range v.Names() {
		desc, _ := v.Describe(name)
		parts = append(parts, fmt.Sprintf("- %s: %s", name, desc))
	}
	result := ""
	for _, p := range parts {
		result += p + "\n"
	}
	return result
}

func instrumentToolsFromView(v *PlayerView) []tangle.Tool {
	var tools []tangle.Tool
	for _, name := range v.Names() {
		desc, _ := v.Describe(name)
		schema, _ := v.Schema(name)
		tools = append(tools, tangle.Tool{
			Name:        name,
			Description: desc,
			InputSchema: schema,
		})
	}
	return tools
}

// GameMode labels the topology of a multi-agent game.
type GameMode int

const (
	Solo   GameMode = iota
	Team            // cooperative: shared goal, shared or split instruments
	Versus          // adversarial: opposing goals, separate instrument sets
)

func (m GameMode) String() string {
	return [...]string{"solo", "team", "versus"}[m]
}
