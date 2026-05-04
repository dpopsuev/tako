package shell

import (
	"testing"
)

func TestFunctionShell_PerFunctionMetadata(t *testing.T) {
	sh := NewFunctionShell()
	sh.Add(EchoFunction{}, ReadAction, 0)

	writeFunc := EchoFunction{}
	sh.AddWithApproval(
		renamedFunc{writeFunc, "deploy"},
		WriteAction, 0.9, HITL,
	)

	if m := sh.Mode("echo"); m != ReadAction {
		t.Errorf("echo mode = %v, want ReadAction", m)
	}
	if m := sh.Mode("deploy"); m != WriteAction {
		t.Errorf("deploy mode = %v, want WriteAction", m)
	}
	if r := sh.Risk("echo"); r != 0 {
		t.Errorf("echo risk = %f, want 0", r)
	}
	if r := sh.Risk("deploy"); r != 0.9 {
		t.Errorf("deploy risk = %f, want 0.9", r)
	}
	if a := sh.Approval("deploy"); a != HITL {
		t.Errorf("deploy approval = %v, want HITL", a)
	}
}

type renamedFunc struct {
	Function
	name string
}

func (r renamedFunc) Name() string { return r.name }
