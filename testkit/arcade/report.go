package arcade

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/jedib0t/go-pretty/v6/table"

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
	data, _ := json.MarshalIndent(r, "", "  ")
	return string(data)
}

func (r ExperimentReport) Pretty() string {
	var b strings.Builder

	t := table.NewWriter()
	t.SetOutputMirror(&b)
	t.SetStyle(table.StyleLight)
	t.SetTitle(fmt.Sprintf("EXPERIMENT: %s (%d sessions)", r.Scenario, len(r.Sessions)))

	t.AppendHeader(table.Row{"#", "Turns", "Solved", "Dist", "Pressure", "Pipes", "TokIn", "TokOut", "OAE", "Tools", "Conv"})

	for _, s := range r.Sessions {
		rep := s.Report
		t.AppendRow(table.Row{
			s.Session,
			rep.TotalTurns,
			s.Solved,
			fmt.Sprintf("%.2f", rep.FinalDistance),
			fmt.Sprintf("%.2f", rep.Pressure),
			s.PipeCount,
			rep.TotalTokensIn,
			rep.TotalTokensOut,
			fmt.Sprintf("%.2f", rep.OAE),
			rep.TotalToolCalls,
			conventionality(rep),
		})
	}

	solvedCount := 0
	for _, s := range r.Sessions {
		if s.Solved {
			solvedCount++
		}
	}

	footer := fmt.Sprintf("Solved: %d/%d", solvedCount, len(r.Sessions))
	if len(r.Sessions) >= 2 {
		first := r.Sessions[0].Report
		last := r.Sessions[len(r.Sessions)-1].Report
		footer += fmt.Sprintf(" | Flywheel: turns %d→%d, tokens %d→%d",
			first.TotalTurns, last.TotalTurns,
			first.TotalTokensIn+first.TotalTokensOut,
			last.TotalTokensIn+last.TotalTokensOut)
	}
	t.SetCaption(footer)

	t.Render()
	return b.String()
}

func conventionality(rep cerebrum.SessionReport) string {
	switch {
	case rep.ClearPct > 50:
		return "clear"
	case rep.ComplicatedPct > 50:
		return "complicated"
	case rep.ComplexPct > 50:
		return "complex"
	default:
		return "chaotic"
	}
}
