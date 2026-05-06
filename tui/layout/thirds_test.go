package layout

import "testing"

// ===================================================================
// RED: Degenerate cases
// ===================================================================

func TestThirdsLayout_ZeroHeight(t *testing.T) {
	layout := ComputeThirdsLayout(0, 1)
	if layout.OutputHeight < MinOutputHeight {
		t.Fatalf("output = %d, want >= %d", layout.OutputHeight, MinOutputHeight)
	}
	if layout.InputHeight < MinInputHeight {
		t.Fatalf("input = %d, want >= %d", layout.InputHeight, MinInputHeight)
	}
	if layout.DashboardHeight < MinDashboardHeight {
		t.Fatalf("dashboard = %d, want >= %d", layout.DashboardHeight, MinDashboardHeight)
	}
}

func TestThirdsLayout_TinyTerminal(t *testing.T) {
	layout := ComputeThirdsLayout(10, 1)
	// All three panels should have minimum heights.
	if layout.OutputHeight < MinOutputHeight {
		t.Fatalf("output = %d", layout.OutputHeight)
	}
	if layout.InputHeight < MinInputHeight {
		t.Fatalf("input = %d", layout.InputHeight)
	}
	if layout.DashboardHeight < MinDashboardHeight {
		t.Fatalf("dashboard = %d", layout.DashboardHeight)
	}
}

// ===================================================================
// GREEN: Correct proportions
// ===================================================================

func TestThirdsLayout_40Rows(t *testing.T) {
	layout := ComputeThirdsLayout(40, 1)

	// Input bottom should be around 60% = row 24.
	// Output = rows 0-22 (~23 rows).
	// Input = 1 row.
	// Dashboard = ~10 rows (25% of 40).
	if layout.OutputHeight < 15 {
		t.Fatalf("output = %d, want >= 15 (40-row terminal)", layout.OutputHeight)
	}
	if layout.InputHeight != 1 {
		t.Fatalf("input = %d, want 1 (single line)", layout.InputHeight)
	}
	if layout.DashboardHeight < 5 {
		t.Fatalf("dashboard = %d, want >= 5", layout.DashboardHeight)
	}

	// Total should not exceed terminal height.
	total := layout.OutputHeight + layout.InputHeight + layout.DashboardHeight
	if total > 40 {
		t.Fatalf("total = %d > 40", total)
	}
}

func TestThirdsLayout_80Rows(t *testing.T) {
	layout := ComputeThirdsLayout(80, 1)

	// More space -> proportionally larger panels.
	if layout.OutputHeight < 30 {
		t.Fatalf("output = %d, want >= 30 (80-row terminal)", layout.OutputHeight)
	}
	if layout.DashboardHeight < 10 {
		t.Fatalf("dashboard = %d, want >= 10", layout.DashboardHeight)
	}

	total := layout.OutputHeight + layout.InputHeight + layout.DashboardHeight
	if total > 80 {
		t.Fatalf("total = %d > 80", total)
	}
}

func TestThirdsLayout_24Rows(t *testing.T) {
	layout := ComputeThirdsLayout(24, 1)

	// Minimum viable: all three panels fit.
	if layout.OutputHeight < MinOutputHeight {
		t.Fatalf("output = %d", layout.OutputHeight)
	}
	total := layout.OutputHeight + layout.InputHeight + layout.DashboardHeight
	if total > 24 {
		t.Fatalf("total = %d > 24", total)
	}
}

// ===================================================================
// BLUE: Dynamic expansion
// ===================================================================

func TestThirdsLayout_InputMultiLine(t *testing.T) {
	single := ComputeThirdsLayout(40, 1)
	multi := ComputeThirdsLayout(40, 3)

	// Multi-line input should be taller.
	if multi.InputHeight <= single.InputHeight {
		t.Fatalf("multi input = %d, single = %d — multi should be taller", multi.InputHeight, single.InputHeight)
	}

	// Output should shrink to accommodate.
	if multi.OutputHeight >= single.OutputHeight {
		t.Fatalf("multi output = %d, single = %d — output should shrink", multi.OutputHeight, single.OutputHeight)
	}
}

func TestThirdsLayout_InputMaxExpansion(t *testing.T) {
	// Even with 20-line input, output never below minimum.
	layout := ComputeThirdsLayout(40, 20)
	if layout.OutputHeight < MinOutputHeight {
		t.Fatalf("output = %d, should never go below %d", layout.OutputHeight, MinOutputHeight)
	}
}

func TestThirdsLayout_DashboardFixed(t *testing.T) {
	// Dashboard height should not change with input expansion.
	single := ComputeThirdsLayout(40, 1)
	multi := ComputeThirdsLayout(40, 5)

	if single.DashboardHeight != multi.DashboardHeight {
		t.Fatalf("dashboard changed: single=%d, multi=%d — should be fixed",
			single.DashboardHeight, multi.DashboardHeight)
	}
}

func TestThirdsLayout_InputAnchorPosition(t *testing.T) {
	layout := ComputeThirdsLayout(40, 1)

	// Input bottom = output + input height. Should be near 60% (24).
	inputBottom := layout.OutputHeight + layout.InputHeight
	expectedAnchor := int(40 * InputAnchorRatio) // 24

	// Allow +/-2 rows tolerance.
	if inputBottom < expectedAnchor-2 || inputBottom > expectedAnchor+2 {
		t.Fatalf("input bottom = %d, want ~%d (60%% of 40)", inputBottom, expectedAnchor)
	}
}
