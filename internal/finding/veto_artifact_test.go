package finding

import "testing"

func TestVetoArtifact(t *testing.T) {
	inner := &stubArtifact{typ: "test", confidence: 0.95, raw: "data"}
	v := &VetoArtifact{Inner: inner}

	if v.Type() != "test" {
		t.Errorf("Type() = %q, want %q", v.Type(), "test")
	}
	if v.Confidence() != 0 {
		t.Errorf("Confidence() = %f, want 0", v.Confidence())
	}
	if v.Raw() != "data" {
		t.Errorf("Raw() = %v, want %q", v.Raw(), "data")
	}
}
