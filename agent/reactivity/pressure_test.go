package reactivity

import (
	"testing"
	"time"
)

func TestPressure_EmptyMolecule(t *testing.T) {
	m := NewMolecule("test")
	if p := m.Pressure(); p != 0 {
		t.Errorf("empty molecule pressure = %.2f, want 0", p)
	}
}

func TestPressure_IntentOnly_MaxPressure(t *testing.T) {
	m := NewMolecule("test")
	m.InsertAtom(Atom{ID: "i1", Type: IntentAtom, Content: []byte("help"), CreatedAt: time.Now()})

	p := m.Pressure()
	if p != 1.0 {
		t.Errorf("intent-only molecule pressure = %.2f, want 1.0", p)
	}
}

func TestPressure_NoDesired_ResponseDropsToZero(t *testing.T) {
	m := NewMolecule("test")
	m.InsertAtom(Atom{ID: "i1", Type: IntentAtom, Content: []byte("hello"), CreatedAt: time.Now()})

	if m.Pressure() != 1.0 {
		t.Fatal("before response, pressure should be 1.0")
	}

	m.Chain().Append(ChainEvent{Kind: Motor, Organ: "cerebrum.text", Output: []byte("hi"), IsResponse: true})

	if p := m.Pressure(); p != 0 {
		t.Errorf("after response, pressure = %.2f, want 0", p)
	}
}

func TestPressure_WithDesired_CriteriaMetDropsToZero(t *testing.T) {
	m := NewMoleculeWithCatalyst("test", Catalyst{
		Need:    "fix it",
		Desired: map[string]any{"fixed": true},
	})
	m.InsertAtom(Atom{ID: "i1", Type: IntentAtom, Content: []byte("fix"), CreatedAt: time.Now()})

	if p := m.Pressure(); p <= 0 {
		t.Fatal("before criteria met, pressure should be positive")
	}

	m.ReportSensor("fixed", true)

	if p := m.Pressure(); p != 0 {
		t.Errorf("after criteria met, pressure = %.2f, want 0", p)
	}
}

func TestPressure_WithDesired_UnmetKeepsPressureHigh(t *testing.T) {
	m := NewMoleculeWithCatalyst("test", Catalyst{
		Need:    "fix both",
		Desired: map[string]any{"build": true, "tests": true},
	})
	m.InsertAtom(Atom{ID: "i1", Type: IntentAtom, Content: []byte("fix"), CreatedAt: time.Now()})

	m.ReportSensor("build", true)

	p := m.Pressure()
	if p <= 0 {
		t.Error("with one unmet dimension, pressure should be positive")
	}
	if p >= 1.0 {
		t.Errorf("with one met dimension, pressure should be < 1.0, got %.2f", p)
	}
}

func TestSettled_DelegatesToPressure(t *testing.T) {
	m := NewMolecule("test")
	m.InsertAtom(Atom{ID: "i1", Type: IntentAtom, Content: []byte("hello"), CreatedAt: time.Now()})

	if m.Settled() {
		t.Error("should not be settled before response")
	}

	m.Chain().Append(ChainEvent{Kind: Motor, Organ: "cerebrum.text", Output: []byte("hi"), IsResponse: true})

	if !m.Settled() {
		t.Error("should be settled after response (no Desired)")
	}
}

func TestVectorGap(t *testing.T) {
	m := NewMolecule("test")

	if g := m.VectorGap(); g != 1.0 {
		t.Errorf("empty molecule vector gap = %.2f, want 1.0", g)
	}

	m.InsertAtom(Atom{ID: "i1", Type: IntentAtom, CreatedAt: time.Now()})
	if g := m.VectorGap(); g != 0.75 {
		t.Errorf("intent-only vector gap = %.2f, want 0.75", g)
	}

	m.InsertAtom(Atom{ID: "a1", Type: AssessmentAtom, CreatedAt: time.Now()})
	if g := m.VectorGap(); g != 0.5 {
		t.Errorf("intent+assessment vector gap = %.2f, want 0.5", g)
	}

	m.Chain().Append(ChainEvent{Kind: Motor, Organ: "cerebrum.text", Output: []byte("done"), IsResponse: true})
	if g := m.VectorGap(); g != 0.0 {
		t.Errorf("all vectors filled gap = %.2f, want 0.0", g)
	}
}
