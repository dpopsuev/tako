package andon

import "testing"

func TestStubSignalAlwaysGreen(t *testing.T) {
	s := &StubSignal{}
	if s.Status() != Green {
		t.Errorf("expected Green, got %s", s.Status())
	}
	s.Pull("agent-1")
	s.Pull("agent-1")
	if s.Status() != Green {
		t.Errorf("stub signal should stay Green, got %s", s.Status())
	}
	if s.Pulls() != 2 {
		t.Errorf("expected 2 pulls, got %d", s.Pulls())
	}
}

func TestStubSignalReset(t *testing.T) {
	s := &StubSignal{}
	s.Pull("agent-1")
	s.Reset()
	if s.Pulls() != 0 {
		t.Errorf("expected 0 pulls after reset, got %d", s.Pulls())
	}
}
