package render

import "sync"

// StubCanvas is a headless Canvas — accepts posts, no rendering.
type StubCanvas struct {
	mu     sync.Mutex
	panels []Panel
	events chan Event
}

var _ Canvas = (*StubCanvas)(nil)

func NewStubCanvas() *StubCanvas {
	return &StubCanvas{events: make(chan Event, 64)}
}

func (b *StubCanvas) Post(panel Panel) {
	b.mu.Lock()
	defer b.mu.Unlock()
	for i, p := range b.panels {
		if p.ID == panel.ID {
			b.panels[i] = panel
			b.events <- Event{Type: Updated, PanelID: panel.ID}
			return
		}
	}
	b.panels = append(b.panels, panel)
	b.events <- Event{Type: Posted, PanelID: panel.ID}
}

func (b *StubCanvas) Retract(id string) {
	b.mu.Lock()
	defer b.mu.Unlock()
	for i, p := range b.panels {
		if p.ID == id {
			b.panels = append(b.panels[:i], b.panels[i+1:]...)
			b.events <- Event{Type: Retracted, PanelID: id}
			return
		}
	}
}

func (b *StubCanvas) Panels() []Panel {
	b.mu.Lock()
	defer b.mu.Unlock()
	out := make([]Panel, len(b.panels))
	copy(out, b.panels)
	return out
}

func (b *StubCanvas) Subscribe() <-chan Event {
	return b.events
}

func (b *StubCanvas) PanelCount() int {
	b.mu.Lock()
	defer b.mu.Unlock()
	return len(b.panels)
}
