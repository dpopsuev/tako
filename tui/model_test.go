package tui

import (
	"testing"
)

func TestCabinLayout_Proportions(t *testing.T) {
	m := NewModel(nil, "test-model")
	m.engine.Resize(80, 24)

	heights := m.engine.ComputeHeights()

	statusH := heights[m.status.ID()]
	outputH := heights[m.output.ID()]
	inputH := heights[m.input.ID()]

	if statusH < 1 {
		t.Errorf("status height = %d, want >= 1", statusH)
	}
	if outputH < 5 {
		t.Errorf("output height = %d, want >= 5 (MinHeight)", outputH)
	}
	if inputH < 3 {
		t.Errorf("input height = %d, want >= 3", inputH)
	}

	total := statusH + outputH + inputH
	if outputH < total/2 {
		t.Errorf("output should get majority of space: output=%d total=%d", outputH, total)
	}
}

func TestCabinLayout_SmallTerminal(t *testing.T) {
	m := NewModel(nil, "test-model")
	m.engine.Resize(40, 12)

	heights := m.engine.ComputeHeights()
	outputH := heights[m.output.ID()]

	if outputH < 5 {
		t.Errorf("output MinHeight should be respected: got %d, want >= 5", outputH)
	}
}

func TestCabinLayout_WideTerminal(t *testing.T) {
	m := NewModel(nil, "test-model")
	m.engine.Resize(200, 50)

	heights := m.engine.ComputeHeights()
	outputH := heights[m.output.ID()]

	if outputH < 40 {
		t.Errorf("output should scale with terminal: got %d for height=50", outputH)
	}
}
