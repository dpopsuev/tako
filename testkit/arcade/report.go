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
	Session      int                    `json:"session"`
	Scenario     string                 `json:"scenario"`
	Solved       bool                   `json:"solved"`
	Report       cerebrum.SessionReport `json:"report"`
	GameState    map[string]any         `json:"game_state"`
	PipeCount    int                    `json:"pipe_count"`
	OptimalTurns int                    `json:"optimal_turns"`
	OptimalSteps []string               `json:"optimal_steps"`
	ActualSteps  []string               `json:"actual_steps"`
}

func CollectResult(session int, scenario Scenario, agent *assemble.Agent, reflexStore cerebrum.ReflexStore) ArcadeResult {
	report := agent.LastReport()

	var actualSteps []string
	for _, e := range report.ChainEvents {
		if e.Organ != "" && e.Organ != "cerebrum.text" && e.Organ != "cerebrum.reflex" {
			actualSteps = append(actualSteps, e.Organ)
		}
	}

	return ArcadeResult{
		Session:      session,
		Scenario:     scenario.Name,
		Solved:       scenario.IsSolved(scenario.Adventure.State()),
		Report:       report,
		GameState:    scenario.Adventure.State(),
		PipeCount:    reflexStore.Len(),
		OptimalTurns: scenario.OptimalTurns,
		OptimalSteps: scenario.OptimalSteps,
		ActualSteps:  actualSteps,
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

	t.AppendHeader(table.Row{"#", "Turns", "Optimal", "Solved", "Dist", "Pipes", "TokIn", "TokOut", "Tools", "Conv"})

	for _, s := range r.Sessions {
		rep := s.Report
		turnsLabel := fmt.Sprintf("%d", rep.TotalTurns)
		if s.OptimalTurns > 0 {
			turnsLabel = fmt.Sprintf("%d/%d", rep.TotalTurns, s.OptimalTurns)
		}
		t.AppendRow(table.Row{
			s.Session,
			turnsLabel,
			strings.Join(s.OptimalSteps, "→"),
			s.Solved,
			fmt.Sprintf("%.2f", rep.FinalDistance),
			s.PipeCount,
			rep.TotalTokensIn,
			rep.TotalTokensOut,
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

	if len(r.Sessions) > 0 && len(r.Sessions[0].OptimalSteps) > 0 {
		s := r.Sessions[0]
		fmt.Fprintf(&b, "\nOptimal: %s\n", strings.Join(s.OptimalSteps, " → "))
		fmt.Fprintf(&b, "Actual:  %s\n", strings.Join(s.ActualSteps, " → "))
		extra := len(s.ActualSteps) - len(s.OptimalSteps)
		if extra > 0 {
			fmt.Fprintf(&b, "Gap:     %d extra steps\n", extra)
		} else if extra == 0 {
			fmt.Fprintf(&b, "Gap:     optimal path achieved\n")
		}
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
