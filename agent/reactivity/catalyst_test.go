package reactivity

import "testing"

func TestCatalyst_SealOnCriteriaMatch(t *testing.T) {
	catalyst := Catalyst{Need: "eat food", Criteria: map[string]any{"hungry": false}}
	m := NewMoleculeWithCatalyst("mol-1", catalyst)

	m.InsertAtom(Atom{ID: "a1", Type: IntentAtom, Taxonomy: "intent.need", Content: []byte("eat food")})

	if m.Sealed() {
		t.Fatal("should not be sealed before sensor report")
	}

	m.ReportSensor("hungry", false)

	if !m.Sealed() {
		t.Fatal("should be sealed after criteria met")
	}
	if m.Distance() != 0.0 {
		t.Errorf("distance should be 0.0, got %f", m.Distance())
	}
}

func TestCatalyst_DistanceFromSensors(t *testing.T) {
	catalyst := Catalyst{
		Need:     "eat",
		Criteria: map[string]any{"hungry": false, "plate": ""},
	}
	m := NewMoleculeWithCatalyst("mol-2", catalyst)

	if m.Distance() != 1.0 {
		t.Errorf("expected distance 1.0, got %f", m.Distance())
	}

	m.ReportSensor("plate", "")
	if m.Distance() != 0.5 {
		t.Errorf("expected distance 0.5, got %f", m.Distance())
	}

	m.ReportSensor("hungry", false)
	if m.Distance() != 0.0 {
		t.Errorf("expected distance 0.0, got %f", m.Distance())
	}
	if !m.Sealed() {
		t.Fatal("should be sealed when all criteria met")
	}
}

func TestCatalyst_NoCriteria_DistanceFallsBackToPhases(t *testing.T) {
	m := NewMolecule("mol-3")

	d := m.Distance()
	if d != 1.0 {
		t.Errorf("no criteria, no mass — distance should be 1.0, got %f", d)
	}

	m.InsertAtom(Atom{ID: "a1", Type: IntentAtom, Taxonomy: "intent.need", Content: []byte("test")})
	d = m.Distance()
	if d >= 1.0 {
		t.Errorf("with mass, distance should decrease, got %f", d)
	}
}
