package tui

import (
	"testing"
)

func TestCabinLayout_Proportions(t *testing.T) {
	m := NewModel(nil, "test-model")
	m.engine.Resize(80, 24)

	heights := m.engine.ComputeHeights()

	statusH := heights[m.status.ID()]
	cabinH := heights[m.cabin.ID()]
	footerH := heights[m.footer.ID()]

	if statusH < 1 {
		t.Errorf("status height = %d, want >= 1", statusH)
	}
	if cabinH < 10 {
		t.Errorf("cabin height = %d, want >= 10 (MinHeight)", cabinH)
	}
	if footerH < 1 {
		t.Errorf("footer height = %d, want >= 1", footerH)
	}

	total := statusH + cabinH + footerH
	if cabinH < total*3/4 {
		t.Errorf("cabin should get majority of space: cabin=%d total=%d", cabinH, total)
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

	if cabinH < 40 {
		t.Errorf("cabin should scale with terminal: got %d for height=50", cabinH)
	}
}
