package reactivity

import "testing"

func TestDefaultConfig_Values(t *testing.T) {
	cfg := DefaultConfig
	if cfg.DistanceClose != 0.3 {
		t.Errorf("DistanceClose: got %v, want 0.3", cfg.DistanceClose)
	}
	if cfg.DistanceMid != 0.5 {
		t.Errorf("DistanceMid: got %v, want 0.5", cfg.DistanceMid)
	}
	if cfg.RecollectionMin != 0.3 {
		t.Errorf("RecollectionMin: got %v, want 0.3", cfg.RecollectionMin)
	}
	if cfg.UnmetDimMax != 2 {
		t.Errorf("UnmetDimMax: got %v, want 2", cfg.UnmetDimMax)
	}
	if cfg.BackwardTurnLimit != 3 {
		t.Errorf("BackwardTurnLimit: got %v, want 3", cfg.BackwardTurnLimit)
	}
	if cfg.CompactMaxChars != 500 {
		t.Errorf("CompactMaxChars: got %v, want 500", cfg.CompactMaxChars)
	}
	if cfg.ContractSummaryMax != 120 {
		t.Errorf("ContractSummaryMax: got %v, want 120", cfg.ContractSummaryMax)
	}
}

func TestNewTreeNavigator_CustomConfig(t *testing.T) {
	cfg := DefaultConfig
	cfg.DistanceClose = 0.1

	nav := NewTreeNavigator(&cfg)
	m := NewMoleculeWithCatalyst("test", Catalyst{
		Need:    "test",
		Desired: map[string]any{"a": true, "b": true, "c": true, "d": true},
	})

	next := nav(m, IntentAtom)
	if next == SelectionAtom {
		t.Error("with DistanceClose=0.1 and distance=1.0, should NOT shortcut to selection")
	}

	cfg2 := DefaultConfig
	cfg2.DistanceClose = 0.99
	nav2 := NewTreeNavigator(&cfg2)

	m2 := NewMoleculeWithCatalyst("test2", Catalyst{
		Need:    "test",
		Desired: map[string]any{"a": true},
	})

	next2 := nav2(m2, IntentAtom)
	if next2 != SelectionAtom {
		t.Errorf("with DistanceClose=0.99 and distance=1.0, should shortcut; got %s", next2)
	}
}

func TestNewTreeNavigator_BackwardTurnLimit(t *testing.T) {
	cfg := DefaultConfig
	cfg.BackwardTurnLimit = 1

	nav := NewTreeNavigator(&cfg)
	m := NewMoleculeWithCatalyst("test", Catalyst{
		Need:    "test",
		Desired: map[string]any{"a": true, "b": true},
	})

	m.ReportSensor("a", true)
	m.Tick()
	m.ReportSensor("a", "wrong")
	m.Tick()

	next := nav(m, AcclimationAtom)
	if next != RetrospectionAtom {
		t.Errorf("with BackwardTurnLimit=1 and turns=2 and D>0, should cut losses; got %s", next)
	}
}
