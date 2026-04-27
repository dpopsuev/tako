package render

import "testing"

func TestStubCanvasMount(t *testing.T) {
	canvas := &StubCanvas{}
	node := &Node{ID: "root", Data: []byte("hello")}
	canvas.Mount(node)

	data := canvas.Render()
	if string(data) != "hello" {
		t.Errorf("expected 'hello', got %q", data)
	}
}

func TestStubCanvasDamage(t *testing.T) {
	canvas := &StubCanvas{}
	canvas.Damage(DamageRegion{NodeID: "a"})
	canvas.Damage(DamageRegion{NodeID: "b"})
	if canvas.DamageCount() != 2 {
		t.Errorf("expected 2 damages, got %d", canvas.DamageCount())
	}
}

func TestStubCanvasRenderNil(t *testing.T) {
	canvas := &StubCanvas{}
	data := canvas.Render()
	if data != nil {
		t.Errorf("expected nil render from unmounted canvas, got %v", data)
	}
}
