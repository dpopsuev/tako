package sumi

import (
	"context"
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/dpopsuev/origami/circuit"
	"github.com/dpopsuev/origami/view"
)

// TxRxPanel implements Panel for displaying agent prompts (TX) and responses (RX).
type TxRxPanel struct {
	log          *view.TxRxLog
	noColor      bool
	workerFilter string
	scrollTx     int
}

// NewTxRxPanel creates a TX/RX panel stub.
func NewTxRxPanel(log *view.TxRxLog, noColor bool) *TxRxPanel {
	return &TxRxPanel{
		log:     log,
		noColor: noColor,
	}
}

func (p *TxRxPanel) ID() string                { return "txrx" }
func (p *TxRxPanel) Title() string             { return "TX/RX" }
func (p *TxRxPanel) Focusable() bool           { return true }
func (p *TxRxPanel) PreferredSize() (int, int) { return 40, 10 }

// SetWorkerFilter restricts TX/RX display to a specific worker.
func (p *TxRxPanel) SetWorkerFilter(wid string) {
	p.workerFilter = wid
}

func (p *TxRxPanel) Update(msg tea.Msg) tea.Cmd {
	km, ok := msg.(tea.KeyMsg)
	if !ok {
		return nil
	}
	switch km.String() {
	case "up":
		if p.scrollTx > 0 {
			p.scrollTx--
		}
	case "down":
		p.scrollTx++
	}
	return nil
}

func (p *TxRxPanel) View(area Rect) string {
	inner := area.Inner()
	if inner.W <= 0 || inner.H <= 0 {
		return ""
	}

	if p.log == nil || p.log.Len() == 0 {
		return "TX/RX: waiting for data..."
	}

	tx, rx := p.log.LastTxRx(p.workerFilter)

	halfH := inner.H / 2
	if halfH < 1 {
		halfH = 1
	}

	var sb strings.Builder

	sb.WriteString("─── TX (prompt) ───\n")
	if tx != nil {
		sb.WriteString(truncateContent(tx.Content, inner.W, halfH-1))
	} else {
		sb.WriteString("(no prompt)")
	}
	sb.WriteByte('\n')

	sb.WriteString("─── RX (response) ───\n")
	if rx != nil {
		sb.WriteString(truncateContent(rx.Content, inner.W, halfH-1))
	} else {
		sb.WriteString("(no response)")
	}

	return sb.String()
}

func truncateContent(content string, width, maxLines int) string {
	lines := strings.Split(content, "\n")
	if len(lines) > maxLines {
		lines = lines[:maxLines]
		lines = append(lines, fmt.Sprintf("... (%d more lines)", len(strings.Split(content, "\n"))-maxLines))
	}
	for i, line := range lines {
		if len([]rune(line)) > width {
			lines[i] = string([]rune(line)[:width-1]) + "…"
		}
	}
	return strings.Join(lines, "\n")
}

// BootstrapStoreFromSnapshot builds a client-side CircuitStore from
// a CircuitSnapshot (as returned by Kami's /api/snapshot endpoint).
func BootstrapStoreFromSnapshot(snap view.CircuitSnapshot) *view.CircuitStore {
	def := &circuit.CircuitDef{Circuit: snap.CircuitName}
	for name := range snap.Nodes {
		def.Nodes = append(def.Nodes, circuit.NodeDef{Name: name})
	}

	store := view.NewCircuitStore(def)

	for name, ns := range snap.Nodes {
		if ns.State == view.NodeActive || ns.State == view.NodeCompleted || ns.State == view.NodeError {
			var evtType circuit.WalkEventType
			switch ns.State {
			case view.NodeActive:
				evtType = circuit.EventNodeEnter
			case view.NodeCompleted:
				evtType = circuit.EventNodeExit
			case view.NodeError:
				evtType = circuit.EventWalkError
			}
			store.OnEvent(circuit.WalkEvent{Type: evtType, Node: name})
		}
	}

	for walkerID, wp := range snap.Walkers {
		store.OnEvent(circuit.WalkEvent{
			Type:   circuit.EventNodeEnter,
			Node:   wp.Node,
			Walker: walkerID,
		})
	}

	return store
}

// SSEClientLoop is the public entry point for the SSE client loop.
// It connects to a Kami SSE endpoint and feeds events into the store,
// reconnecting with exponential backoff on disconnect.
func SSEClientLoop(ctx context.Context, addr string, store *view.CircuitStore) {
	sseClientLoop(ctx, addr, store, sumiLogger())
}
