package arcade

import (
	"encoding/json"
	"fmt"
	"strings"
	"text/tabwriter"

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

	fmt.Fprintf(&b, "\n=== EXPERIMENT: %s (%d sessions) ===\n", r.Scenario, len(r.Sessions))

	w := tabwriter.NewWriter(&b, 0, 0, 2, ' ', 0)
	fmt.Fprintln(w, "#\tTurns\tSolved\tDist\tPressure\tPipes\tTokIn\tTokOut\tOAE\tTools\tConv\t")

	for _, s := range r.Sessions {
		rep := s.Report
		fmt.Fprintf(w, "%d\t%d\t%v\t%.2f\t%.2f\t%d\t%d\t%d\t%.2f\t%d\t%s\t\n",
			s.Session, rep.TotalTurns, s.Solved, rep.FinalDistance,
			rep.Pressure, s.PipeCount, rep.TotalTokensIn, rep.TotalTokensOut,
			rep.OAE, rep.TotalToolCalls, conventionality(rep))
	}
	w.Flush()

	solvedCount := 0
	for _, s := range r.Sessions {
		if s.Solved {
			solvedCount++
		}
	}
	fmt.Fprintf(&b, "\nSolved: %d/%d sessions\n", solvedCount, len(r.Sessions))

	if len(r.Sessions) >= 2 {
		first := r.Sessions[0].Report
		last := r.Sessions[len(r.Sessions)-1].Report
		fmt.Fprintf(&b, "Flywheel: turns %d→%d, tokens %d→%d\n",
			first.TotalTurns, last.TotalTurns,
			first.TotalTokensIn+first.TotalTokensOut,
			last.TotalTokensIn+last.TotalTokensOut)
	}

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
