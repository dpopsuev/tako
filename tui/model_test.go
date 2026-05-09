package tui

import (
	"testing"
)

func TestCabinLayout_Proportions(t *testing.T) {
	m := NewModel(nil, "test-model")
	m.engine.Resize(80, 24)

	heights := m.engine.ComputeHeights()
	cabinH := heights[m.cabin.ID()]

	if cabinH < 20 {
		t.Errorf("cabin height = %d, want >= 20 at 80x24", cabinH)
	}
}

func TestCabinLayout_SmallTerminal(t *testing.T) {
	m := NewModel(nil, "test-model")
	m.engine.Resize(40, 12)

	heights := m.engine.ComputeHeights()
	cabinH := heights[m.cabin.ID()]

	if cabinH < 10 {
		t.Errorf("cabin MinHeight should be respected: got %d, want >= 10", cabinH)
	}
}

func TestCabinLayout_WideTerminal(t *testing.T) {
	m := NewModel(nil, "test-model")
	m.engine.Resize(200, 50)

	heights := m.engine.ComputeHeights()
	cabinH := heights[m.cabin.ID()]

	if cabinH < 45 {
		t.Errorf("cabin should scale with terminal: got %d for height=50", cabinH)
	}
}
