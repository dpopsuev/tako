package layout

import "testing"

func TestLayout_Small(t *testing.T) {
	cfg := ComputeLayout(80, 24)
	if cfg.Breakpoint != Small {
		t.Fatalf("breakpoint = %s, want small", cfg.Breakpoint)
	}
	if cfg.InputHeight != 1 {
		t.Fatalf("input height = %d, want 1", cfg.InputHeight)
	}
	if cfg.ShowLogo {
		t.Fatal("small should not show logo")
	}
	if cfg.DashboardStyle != "compact" {
		t.Fatalf("dashboard = %q, want compact", cfg.DashboardStyle)
	}
}

func TestLayout_Medium(t *testing.T) {
	cfg := ComputeLayout(120, 40)
	if cfg.Breakpoint != Medium {
		t.Fatalf("breakpoint = %s, want medium", cfg.Breakpoint)
	}
	if cfg.InputHeight != 3 {
		t.Fatalf("input height = %d, want 3", cfg.InputHeight)
	}
	if !cfg.ShowLogo {
		t.Fatal("medium should show logo")
	}
}

func TestLayout_Large(t *testing.T) {
	cfg := ComputeLayout(200, 50)
	if cfg.Breakpoint != Large {
		t.Fatalf("breakpoint = %s, want large", cfg.Breakpoint)
	}
	if cfg.MaxContentWidth != 200 {
		t.Fatalf("max width = %d, want 200", cfg.MaxContentWidth)
	}
}

func TestLayout_Massive(t *testing.T) {
	cfg := ComputeLayout(300, 60)
	if cfg.Breakpoint != Massive {
		t.Fatalf("breakpoint = %s, want massive", cfg.Breakpoint)
	}
	if cfg.MaxContentWidth > 200 {
		t.Fatalf("max width = %d, should cap at 200", cfg.MaxContentWidth)
	}
}

func TestLayout_ShortTerminal(t *testing.T) {
	cfg := ComputeLayout(120, 15)
	if cfg.InputHeight != 1 {
		t.Fatalf("short terminal input height = %d, want 1", cfg.InputHeight)
	}
}

func TestLayout_InnerWidth(t *testing.T) {
	cfg := ComputeLayout(80, 24)
	if cfg.InnerWidth() != 78 {
		t.Fatalf("inner width = %d, want 78", cfg.InnerWidth())
	}
}

func TestLayout_FixedHeight(t *testing.T) {
	small := ComputeLayout(80, 24)
	medium := ComputeLayout(120, 40)

	// Small layout: all panels contribute 8 fixed lines total.
	if small.FixedHeight() != 8 {
		t.Fatalf("small fixed = %d, want 8", small.FixedHeight())
	}
	// Medium layout: all panels contribute 10 fixed lines total.
	if medium.FixedHeight() != 10 {
		t.Fatalf("medium fixed = %d, want 10", medium.FixedHeight())
	}
}

func TestLayout_BreakpointString(t *testing.T) {
	for _, tc := range []struct {
		b    Breakpoint
		want string
	}{
		{Small, "small"},
		{Medium, "medium"},
		{Large, "large"},
		{Massive, "massive"},
	} {
		if tc.b.String() != tc.want {
			t.Fatalf("%d.String() = %q, want %q", tc.b, tc.b.String(), tc.want)
		}
	}
}
