package arcade

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/dpopsuev/tako/agent/cerebrum"
	"github.com/dpopsuev/tako/agent/reactivity"
	"github.com/dpopsuev/tako/testkit"
)

type ArcadeResult struct {
	Session      int                    `json:"session"`
	Scenario     string                 `json:"scenario"`
	Solved       bool                   `json:"solved"`
	Summary      cerebrum.SessionSummary `json:"summary"`
	GameState    map[string]any         `json:"game_state"`
	ChainEvents  []ChainEventRecord     `json:"chain_events"`
	ToolCalls    []testkit.ToolEvent    `json:"tool_calls"`
	Responses    []string               `json:"responses"`
	Errors       []string               `json:"errors,omitempty"`
	Pressure     float64                `json:"pressure"`
	PipeCount    int                    `json:"pipe_count"`
}

type ChainEventRecord struct {
	Kind       string `json:"kind"`
	Organ      string `json:"organ"`
	Output     string `json:"output"`
	IsResponse bool   `json:"is_response"`
}

func CollectResult(session int, scenario Scenario, agent interface {
	Result() *reactivity.Molecule
	LastSummary() cerebrum.SessionSummary
}, listener *testkit.CapturingListener, reflexStore cerebrum.ReflexStore) ArcadeResult {
	m := agent.Result()
	chain := m.Chain()

	var chainEvents []ChainEventRecord
	for _, e := range chain.All() {
		out := string(e.Output)
		if len(out) > 200 {
			out = out[:200] + "..."
		}
		chainEvents = append(chainEvents, ChainEventRecord{
			Kind:       e.Kind.String(),
			Organ:      e.Organ,
			Output:     out,
			IsResponse: e.IsResponse,
		})
	}

	var errs []string
	for _, e := range listener.Errors {
		errs = append(errs, e.Error())
	}

	return ArcadeResult{
		Session:     session,
		Scenario:    scenario.Name,
		Solved:      scenario.IsSolved(scenario.Adventure.State()),
		Summary:     agent.LastSummary(),
		GameState:   scenario.Adventure.State(),
		ChainEvents: chainEvents,
		ToolCalls:   listener.ToolCalls,
		Responses:   listener.Responses,
		Errors:      errs,
		Pressure:    m.Pressure(),
		PipeCount:   reflexStore.Len(),
	}
}

type ExperimentReport struct {
	Scenario string         `json:"scenario"`
	Sessions []ArcadeResult `json:"sessions"`
}

func (r ExperimentReport) JSON() string {
	data, _ := json.MarshalIndent(r, "", "  ")
	return string(data)
}

func (r ExperimentReport) Pretty() string {
	var b strings.Builder
	b.WriteString(fmt.Sprintf("\n=== EXPERIMENT: %s (%d sessions) ===\n", r.Scenario, len(r.Sessions)))
	b.WriteString(fmt.Sprintf("%-4s %-5s %-6s %-8s %-8s %-5s %-6s %-6s %-5s %-5s\n",
		"#", "Turns", "Solved", "Distance", "Pressure", "Pipes", "TokIn", "TokOut", "OAE", "Conv"))

	for _, s := range r.Sessions {
		conv := "chaotic"
		if s.Summary.ClearPct > 50 {
			conv = "clear"
		} else if s.Summary.ComplicatedPct > 50 {
			conv = "compli"
		} else if s.Summary.ComplexPct > 50 {
			conv = "complex"
		}

		b.WriteString(fmt.Sprintf("%-4d %-5d %-6v %-8.2f %-8.2f %-5d %-6d %-6d %-5.2f %-5s\n",
			s.Session, s.Summary.TotalTurns, s.Solved, s.Summary.FinalDistance,
			s.Pressure, s.PipeCount, s.Summary.TotalTokensIn, s.Summary.TotalTokensOut,
			s.Summary.OAE, conv))
	}

	solvedCount := 0
	for _, s := range r.Sessions {
		if s.Solved {
			solvedCount++
		}
	}
	b.WriteString(fmt.Sprintf("\nSolved: %d/%d sessions\n", solvedCount, len(r.Sessions)))

	if len(r.Sessions) >= 2 {
		first := r.Sessions[0]
		last := r.Sessions[len(r.Sessions)-1]
		b.WriteString(fmt.Sprintf("Flywheel: turns %d→%d, tokens %d→%d\n",
			first.Summary.TotalTurns, last.Summary.TotalTurns,
			first.Summary.TotalTokensIn+first.Summary.TotalTokensOut,
			last.Summary.TotalTokensIn+last.Summary.TotalTokensOut))
	}

	return b.String()
}
