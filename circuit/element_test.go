package circuit

import (
	"strings"
	"testing"
)

func TestDefaultTraits_AllSixElements(t *testing.T) {
	tests := []struct {
		element              Element
		speed                SpeedClass
		maxLoops             int
		convergenceThreshold float64
		shortcutAffinity     float64
		evidenceDepth        int
	}{
		{ElementFire, SpeedFast, 0, 0.50, 0.9, 2},
		{ElementLightning, SpeedFastest, 0, 0.40, 1.0, 1},
		{ElementEarth, SpeedSteady, 1, 0.70, 0.1, 5},
		{ElementDiamond, SpeedPrecise, 0, 0.95, 0.5, 10},
		{ElementWater, SpeedDeep, 3, 0.85, 0.1, 8},
		{ElementAir, SpeedHolistic, 1, 0.60, 0.6, 3},
	}

	for _, tt := range tests {
		t.Run(string(tt.element), func(t *testing.T) {
			traits := DefaultTraits(tt.element)
			if traits.Element != tt.element {
				t.Errorf("Element = %s, want %s", traits.Element, tt.element)
			}
			if traits.Speed != tt.speed {
				t.Errorf("Speed = %s, want %s", traits.Speed, tt.speed)
			}
			if traits.MaxLoops != tt.maxLoops {
				t.Errorf("MaxLoops = %d, want %d", traits.MaxLoops, tt.maxLoops)
			}
			if traits.ConvergenceThreshold != tt.convergenceThreshold {
				t.Errorf("ConvergenceThreshold = %.2f, want %.2f", traits.ConvergenceThreshold, tt.convergenceThreshold)
			}
			if traits.ShortcutAffinity != tt.shortcutAffinity {
				t.Errorf("ShortcutAffinity = %.2f, want %.2f", traits.ShortcutAffinity, tt.shortcutAffinity)
			}
			if traits.EvidenceDepth != tt.evidenceDepth {
				t.Errorf("EvidenceDepth = %d, want %d", traits.EvidenceDepth, tt.evidenceDepth)
			}
			if traits.FailureMode == "" {
				t.Error("FailureMode should not be empty")
			}
		})
	}
}

func TestDefaultTraits_UnknownElement(t *testing.T) {
	traits := DefaultTraits("nonexistent")
	if traits.Element != "" {
		t.Errorf("expected zero-value traits for unknown element, got %+v", traits)
	}
}

func TestAllElements(t *testing.T) {
	elems := AllElements()
	if len(elems) != 6 {
		t.Fatalf("expected 6 core elements, got %d", len(elems))
	}

	seen := make(map[Element]bool)
	for _, e := range elems {
		if seen[e] {
			t.Errorf("duplicate element: %s", e)
		}
		seen[e] = true
	}

	for _, want := range []Element{ElementFire, ElementLightning, ElementEarth, ElementDiamond, ElementWater, ElementAir} {
		if !seen[want] {
			t.Errorf("missing expected element: %s", want)
		}
	}

}

func TestAllElements_ReturnsCopy(t *testing.T) {
	a := AllElements()
	b := AllElements()
	a[0] = "mutated"
	if b[0] == "mutated" {
		t.Error("AllElements should return a copy, not a shared slice")
	}
}

func TestResolveApproach(t *testing.T) {
	tests := []struct {
		approach string
		element  Element
		ok       bool
	}{
		{"rapid", ElementFire, true},
		{"aggressive", ElementLightning, true},
		{"methodical", ElementEarth, true},
		{"rigorous", ElementDiamond, true},
		{"analytical", ElementWater, true},
		{"holistic", ElementAir, true},
		{"unknown", "", false},
	}
	for _, tt := range tests {
		t.Run(tt.approach, func(t *testing.T) {
			elem, ok := ResolveApproach(tt.approach)
			if ok != tt.ok {
				t.Errorf("ResolveApproach(%q) ok = %v, want %v", tt.approach, ok, tt.ok)
			}
			if elem != tt.element {
				t.Errorf("ResolveApproach(%q) = %v, want %v", tt.approach, elem, tt.element)
			}
		})
	}
}

func TestApproachForElement(t *testing.T) {
	for _, e := range AllElements() {
		a := ApproachForElement(e)
		if a == "" {
			t.Errorf("ApproachForElement(%q) returned empty", e)
		}
		roundTrip, _ := ResolveApproach(string(a))
		if roundTrip != e {
			t.Errorf("round-trip failed: %q → %q → %q", e, a, roundTrip)
		}
	}
}

func TestApproachEmoji(t *testing.T) {
	for _, a := range AllApproaches() {
		emoji := ApproachEmoji(a)
		if emoji == "" {
			t.Errorf("ApproachEmoji(%q) returned empty", a)
		}
	}
}

func TestApproachTraitsSummary(t *testing.T) {
	summary := ApproachTraitsSummary(ApproachAnalytical)
	if summary == "" {
		t.Error("ApproachTraitsSummary(analytical) returned empty")
	}
	if !strings.Contains(summary, "deep") {
		t.Errorf("expected 'deep' speed in summary, got: %s", summary)
	}
}

func TestAllApproaches(t *testing.T) {
	approaches := AllApproaches()
	if len(approaches) != 6 {
		t.Fatalf("expected 6 approaches, got %d", len(approaches))
	}
	seen := make(map[Approach]bool)
	for _, a := range approaches {
		if seen[a] {
			t.Errorf("duplicate approach: %s", a)
		}
		seen[a] = true
	}
}
