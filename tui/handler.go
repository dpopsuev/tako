package tui

import (
	"context"
	"fmt"
	"log/slog"
	"strings"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/dpopsuev/tako/agent/cerebrum"
	"github.com/dpopsuev/tako/agent/reactivity"
	"github.com/dpopsuev/tako/assemble"
	"github.com/dpopsuev/tako/tui/widgets"
)

func runAgentCmd(agent *assemble.Agent, task string, p *tea.Program) tea.Cmd {
	return func() tea.Msg {
		ctx := context.Background()

		go observeSignal(agent.Signal, p, ctx)

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

func SubscribeMolecule(agent *assemble.Agent, p *tea.Program) {
	go func() {
		for {
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
						p.Send(widgets.AppendOutputMsg{
							Line: fmt.Sprintf("  [%s] %s", event.Atom.Type, event.Atom.Taxonomy),
						})
					}
				}
			})
			return
		}
	}()
}

func observeSignal(bus cerebrum.Bus, p *tea.Program, ctx context.Context) {
	if bus == nil {
		return
	}
	for {
		event, ok := bus.Receive(ctx)
		if !ok {
			return
		}
		switch {
		case event.Kind == "motor.execute":
			input := string(event.Payload)
			if len(input) > 100 {
				input = input[:100] + "..."
			}
			p.Send(widgets.ToolCallStartMsg{
				ID:    event.ToolCallID,
				Name:  event.Source,
				Input: input,
			})
		case strings.HasPrefix(event.Kind, "motor.denied"):
			p.Send(widgets.AppendOutputMsg{
				Line: fmt.Sprintf("  DENIED: %s (%s)", event.Source, event.Kind),
			})
		case event.Kind == "motor.pending.hitl":
			p.Send(widgets.AppendOutputMsg{
				Line: fmt.Sprintf("  APPROVAL NEEDED: %s (risk=high)", event.Source),
			})
		default:
			slog.Debug("tui.signal", slog.String("kind", event.Kind), slog.String("source", event.Source))
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
