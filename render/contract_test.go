package render

import "testing"

func TestCanvasPost(t *testing.T) {
	bb := NewStubCanvas()
	bb.Post(Panel{ID: "fab-map", Source: "fab", Data: []byte("stations")})
	bb.Post(Panel{ID: "andon", Source: "andon", Data: []byte("green")})

	if bb.PanelCount() != 2 {
		t.Errorf("expected 2 panels, got %d", bb.PanelCount())
	}
}

func TestCanvasUpdate(t *testing.T) {
	bb := NewStubCanvas()
	bb.Post(Panel{ID: "andon", Source: "andon", Data: []byte("green")})
	bb.Post(Panel{ID: "andon", Source: "andon", Data: []byte("yellow")})

	if bb.PanelCount() != 1 {
		t.Errorf("expected 1 panel after update, got %d", bb.PanelCount())
	}
	panels := bb.Panels()
	if string(panels[0].Data) != "yellow" {
		t.Errorf("expected 'yellow', got %q", panels[0].Data)
	}
}

func TestCanvasRetract(t *testing.T) {
	bb := NewStubCanvas()
	bb.Post(Panel{ID: "fab-map", Source: "fab", Data: []byte("stations")})
	bb.Post(Panel{ID: "andon", Source: "andon", Data: []byte("green")})
	bb.Retract("fab-map")

	if bb.PanelCount() != 1 {
		t.Errorf("expected 1 panel after retract, got %d", bb.PanelCount())
	}
}

func TestCanvasSubscribe(t *testing.T) {
	bb := NewStubCanvas()
	ch := bb.Subscribe()
	bb.Post(Panel{ID: "andon", Source: "andon", Data: []byte("green")})

	ev := <-ch
	if ev.Type != Posted {
		t.Errorf("expected Posted event, got %d", ev.Type)
	}
	if ev.PanelID != "andon" {
		t.Errorf("expected panel ID 'andon', got %q", ev.PanelID)
	}
}
