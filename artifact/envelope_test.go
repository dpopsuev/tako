package artifact

import "testing"

func TestEnvelopeSealAndVerify(t *testing.T) {
	e := NewEnvelope("station-1", []byte("work output"))
	e.Labels["reviewed"] = "true"
	e.Labels["priority"] = "high"
	e.Seal()

	if e.Hash == "" {
		t.Fatal("Seal produced empty hash")
	}
	if !e.Verify() {
		t.Error("Verify failed on sealed envelope")
	}
}

func TestEnvelopeTamperDetection(t *testing.T) {
	e := NewEnvelope("station-1", []byte("work output"))
	e.Labels["reviewed"] = "true"
	e.Seal()

	e.Labels["reviewed"] = "false"
	if e.Verify() {
		t.Error("Verify should fail after label tampering")
	}
}

func TestEnvelopePayloadTamper(t *testing.T) {
	e := NewEnvelope("station-1", []byte("original"))
	e.Seal()

	e.Payload = []byte("modified")
	if e.Verify() {
		t.Error("Verify should fail after payload tampering")
	}
}

func TestEnvelopeUnsealedFails(t *testing.T) {
	e := NewEnvelope("station-1", []byte("data"))
	if e.Verify() {
		t.Error("Verify should fail on unsealed envelope")
	}
}

func TestVerifyHashPolicy(t *testing.T) {
	policy := VerifyHashPolicy{}

	e := NewEnvelope("station-1", []byte("data"))
	e.Seal()
	if err := policy.OnPush("shelf-1", e); err != nil {
		t.Errorf("OnPush should pass for sealed envelope: %v", err)
	}

	e.Labels["injected"] = "true"
	if err := policy.OnPush("shelf-1", e); err == nil {
		t.Error("OnPush should reject tampered envelope")
	}
}
