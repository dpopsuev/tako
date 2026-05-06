package tui

import (
	"context"
	"fmt"
	"log/slog"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/dpopsuev/tako/agent/cerebrum"
	"github.com/dpopsuev/tako/agent/reactivity"
	"github.com/dpopsuev/tako/assemble"
	"github.com/dpopsuev/tako/tui/widgets"
)

func runAgent(agent *assemble.Agent, task string) tea.Cmd {
	return func() tea.Msg {
		ctx := context.Background()
		result, err := agent.Run(ctx, task)
		if err != nil {
			return widgets.ErrorMsg{Err: err}
		}
		m := agent.Result()
		_ = result
		return widgets.AgentDoneMsg{
			Sealed:   m.Sealed(),
			Distance: m.Distance(),
			Turns:    m.Turns(),
			OAE:      computeOAE(m),
		}
	}
}

func observeAgent(agent *assemble.Agent, p *tea.Program) {
	m := agent.Result()
	if m == nil {
		return
	}
	m.Subscribe(func(event reactivity.MoleculeEvent) {
		switch event.Kind {
		case "phase_changed":
			p.Send(widgets.PhaseChangeMsg{
				Phase: event.Phase.String(),
			})
		case "atom_inserted":
			if event.Atom != nil {
				slog.Debug("tui.atom_inserted",
					slog.String("type", event.Atom.Type.String()),
					slog.String("taxonomy", event.Atom.Taxonomy))
			}
		}
	})
}

func motorToTUI(bus cerebrum.Bus, p *tea.Program, ctx context.Context) {
	for {
		event, ok := bus.Receive(ctx)
		if !ok {
			return
		}
		switch event.Kind {
		case "instrument":
			p.Send(widgets.ToolCallStartMsg{
				ID:    event.ToolCallID,
				Name:  event.Source,
				Input: truncate(string(event.Payload), 100),
			})
		case "instrument.result":
			p.Send(widgets.ToolCallResultMsg{
				ID:     event.ToolCallID,
				Name:   event.Source,
				Result: truncate(string(event.Payload), 200),
			})
		}
	}
}

func computeOAE(m *reactivity.Molecule) float64 {
	if m.Turns() == 0 {
		return 0
	}
	optimal := max(1, m.Turns()/2)
	oae := float64(optimal) / float64(m.Turns())
	if oae > 1 {
		return 1
	}
	return oae
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + fmt.Sprintf("... (%d more bytes)", len(s)-n)
}
