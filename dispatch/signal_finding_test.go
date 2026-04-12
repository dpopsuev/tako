package dispatch

import (
	"testing"

	"github.com/dpopsuev/origami/circuit"
	"github.com/dpopsuev/troupe/signal"
)

func TestEmitFinding_RoundTrip(t *testing.T) {
	bus := signal.NewMemBus()
	f := circuit.Finding{
		Severity: circuit.FindingError,
		Domain:   "security.auth",
		Source:   "auth-enforcer",
		NodeName: "login",
		Message:  "credentials exposed",
		Evidence: map[string]any{"field": "password"},
	}

	EmitFinding(bus, &f)

	if bus.Len() != 1 {
		t.Fatalf("bus.Len() = %d, want 1", bus.Len())
	}

	sig := bus.Since(0)[0]
	if sig.Event != "enforcer:error" {
		t.Errorf("Event = %q, want %q", sig.Event, "enforcer:error")
	}
	if sig.Meta["domain"] != "security.auth" {
		t.Errorf("Meta[domain] = %q, want %q", sig.Meta["domain"], "security.auth")
	}

	decoded, ok := DecodeFinding(&sig)
	if !ok {
		t.Fatal("DecodeFinding returned false")
	}
	if decoded.Severity != circuit.FindingError {
		t.Errorf("Severity = %q, want %q", decoded.Severity, circuit.FindingError)
	}
	if decoded.Domain != "security.auth" {
		t.Errorf("Domain = %q, want %q", decoded.Domain, "security.auth")
	}
	if decoded.Evidence["field"] != "password" {
		t.Errorf("Evidence[field] = %v, want %q", decoded.Evidence["field"], "password")
	}
}

func TestDecodeFinding_NonFindingSignal(t *testing.T) {
	sig := signal.Signal{Event: "step:complete", Meta: map[string]string{"node": "a"}}
	_, ok := DecodeFinding(&sig)
	if ok {
		t.Error("DecodeFinding should return false for non-finding signal")
	}
}

func TestFindingsSince_MixedSignals(t *testing.T) {
	bus := signal.NewMemBus()

	bus.Emit(&signal.Signal{Event: "step:start", Agent: "agent", Step: "nodeA"})
	EmitFinding(bus, &circuit.Finding{Severity: circuit.FindingWarning, Domain: "test", Source: "tester", Message: "flaky"})
	bus.Emit(&signal.Signal{Event: "step:complete", Agent: "agent", Step: "nodeA"})
	EmitFinding(bus, &circuit.Finding{Severity: circuit.FindingError, Domain: "security", Source: "auditor", Message: "vuln"})

	findings := FindingsSince(bus, 0)
	if len(findings) != 2 {
		t.Fatalf("FindingsSince returned %d findings, want 2", len(findings))
	}
	if findings[0].Severity != circuit.FindingWarning {
		t.Errorf("findings[0].Severity = %q, want %q", findings[0].Severity, circuit.FindingWarning)
	}
	if findings[1].Severity != circuit.FindingError {
		t.Errorf("findings[1].Severity = %q, want %q", findings[1].Severity, circuit.FindingError)
	}
}

func TestFindingsSince_Offset(t *testing.T) {
	bus := signal.NewMemBus()
	EmitFinding(bus, &circuit.Finding{Severity: circuit.FindingInfo, Message: "first"})
	EmitFinding(bus, &circuit.Finding{Severity: circuit.FindingWarning, Message: "second"})

	findings := FindingsSince(bus, 1)
	if len(findings) != 1 {
		t.Fatalf("FindingsSince(1) returned %d, want 1", len(findings))
	}
	if findings[0].Message != "second" {
		t.Errorf("Message = %q, want %q", findings[0].Message, "second")
	}
}

func TestEmitFinding_NoEvidence(t *testing.T) {
	bus := signal.NewMemBus()
	EmitFinding(bus, &circuit.Finding{Severity: circuit.FindingInfo, Message: "no evidence"})

	sig := bus.Since(0)[0]
	if _, ok := sig.Meta["evidence"]; ok {
		t.Error("evidence meta should not be set when Evidence is nil")
	}

	decoded, ok := DecodeFinding(&sig)
	if !ok {
		t.Fatal("DecodeFinding returned false")
	}
	if decoded.Evidence != nil {
		t.Errorf("Evidence = %v, want nil", decoded.Evidence)
	}
}
