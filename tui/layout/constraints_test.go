package layout

import "testing"

func TestSolve_AllFixed(t *testing.T) {
	constraints := []PanelConstraints{
		{Min: 5},  // fixed 5
		{Min: 10}, // fixed 10
		{Min: 3},  // fixed 3
	}
	heights := Solve(constraints, 100)
	if heights[0] != 5 || heights[1] != 10 || heights[2] != 3 {
		t.Errorf("heights = %v, want [5, 10, 3]", heights)
	}
}

func TestSolve_FillDistribution(t *testing.T) {
	constraints := []PanelConstraints{
		{Min: 5},  // fixed 5
		{Fill: 1}, // flex weight 1
		{Fill: 2}, // flex weight 2
	}
	heights := Solve(constraints, 35)
	// Fixed: 5. Remaining: 30. Weight 1 gets 10, weight 2 gets 20.
	if heights[0] != 5 {
		t.Errorf("fixed = %d, want 5", heights[0])
	}
	if heights[1] != 10 {
		t.Errorf("fill(1) = %d, want 10", heights[1])
	}
	if heights[2] != 20 {
		t.Errorf("fill(2) = %d, want 20", heights[2])
	}
}

func TestSolve_Percentage(t *testing.T) {
	constraints := []PanelConstraints{
		{Pct: 0.3}, // 30% of 100 = 30
		{Pct: 0.2}, // 20% of 100 = 20
		{Fill: 1},  // remaining
	}
	heights := Solve(constraints, 100)
	if heights[0] != 30 {
		t.Errorf("30%% = %d, want 30", heights[0])
	}
	if heights[1] != 20 {
		t.Errorf("20%% = %d, want 20", heights[1])
	}
	if heights[2] != 50 {
		t.Errorf("fill = %d, want 50", heights[2])
	}
}

func TestSolve_MinMax(t *testing.T) {
	constraints := []PanelConstraints{
		{Fill: 1, Min: 10, Max: 20}, // clamped
		{Fill: 1},                   // unclamped
	}
	heights := Solve(constraints, 100)
	if heights[0] < 10 || heights[0] > 20 {
		t.Errorf("clamped = %d, want 10-20", heights[0])
	}
}

func TestSolve_Empty(t *testing.T) {
	heights := Solve(nil, 100)
	if heights != nil {
		t.Errorf("empty = %v, want nil", heights)
	}
}

func TestSolve_ZeroAvailable(t *testing.T) {
	constraints := []PanelConstraints{
		{Min: 5},
		{Fill: 1},
	}
	heights := Solve(constraints, 0)
	if heights[0] != 5 {
		t.Errorf("fixed should still get min, got %d", heights[0])
	}
}

func TestClassifyHeight(t *testing.T) {
	tests := []struct {
		height int
		want   HeightBreakpoint
	}{
		{10, HeightTiny},
		{20, HeightSmall},
		{30, HeightMedium},
		{50, HeightLarge},
	}
	for _, tt := range tests {
		got := ClassifyHeight(tt.height)
		if got != tt.want {
			t.Errorf("ClassifyHeight(%d) = %d, want %d", tt.height, got, tt.want)
		}
	}
}
