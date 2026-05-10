package arcade

import (
	"fmt"
	"strings"

	"github.com/dpopsuev/tako/agent/cerebrum"
	"github.com/dpopsuev/tako/assemble"
)

type ArcadeResult struct {
	Session   int                    `json:"session"`
	Scenario  string                 `json:"scenario"`
	Solved    bool                   `json:"solved"`
	Report    cerebrum.SessionReport `json:"report"`
	GameState map[string]any         `json:"game_state"`
	PipeCount int                    `json:"pipe_count"`
}

func CollectResult(session int, scenario Scenario, agent *assemble.Agent, reflexStore cerebrum.ReflexStore) ArcadeResult {
	return ArcadeResult{
		Session:   session,
		Scenario:  scenario.Name,
		Solved:    scenario.IsSolved(scenario.Adventure.State()),
		Report:    agent.LastReport(),
		GameState: scenario.Adventure.State(),
		PipeCount: reflexStore.Len(),
	}
}

type ExperimentReport struct {
	Scenario string         `json:"scenario"`
	Sessions []ArcadeResult `json:"sessions"`
}

func (r ExperimentReport) JSON() string {
	return r.Sessions[0].Report.JSON()
}

func (r ExperimentReport) Pretty() string {
	var b strings.Builder
	b.WriteString(fmt.Sprintf("\n=== EXPERIMENT: %s (%d sessions) ===\n", r.Scenario, len(r.Sessions)))
	b.WriteString(fmt.Sprintf("%-4s %-5s %-6s %-8s %-8s %-5s %-6s %-6s %-5s %-6s %-6s\n",
		"#", "Turns", "Solved", "Distance", "Pressure", "Pipes", "TokIn", "TokOut", "OAE", "Tools", "Conv"))

	for _, s := range r.Sessions {
		rep := s.Report
		conv := "chaotic"
		if rep.ClearPct > 50 {
			conv = "clear"
		} else if rep.ComplicatedPct > 50 {
			conv = "compli"
		} else if rep.ComplexPct > 50 {
			conv = "complex"
		}

		b.WriteString(fmt.Sprintf("%-4d %-5d %-6v %-8.2f %-8.2f %-5d %-6d %-6d %-5.2f %-6d %-6s\n",
			s.Session, rep.TotalTurns, s.Solved, rep.FinalDistance,
			rep.Pressure, s.PipeCount, rep.TotalTokensIn, rep.TotalTokensOut,
			rep.OAE, rep.TotalToolCalls, conv))
	}

	solvedCount := 0
	for _, s := range r.Sessions {
		if s.Solved {
			solvedCount++
		}
	}
	b.WriteString(fmt.Sprintf("\nSolved: %d/%d sessions\n", solvedCount, len(r.Sessions)))

	if len(r.Sessions) >= 2 {
		first := r.Sessions[0].Report
		last := r.Sessions[len(r.Sessions)-1].Report
		b.WriteString(fmt.Sprintf("Flywheel: turns %d→%d, tokens %d→%d\n",
			first.TotalTurns, last.TotalTurns,
			first.TotalTokensIn+first.TotalTokensOut,
			last.TotalTokensIn+last.TotalTokensOut))
	}

	return b.String()
}
