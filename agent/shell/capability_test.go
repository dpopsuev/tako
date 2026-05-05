package shell

import (
	"context"
	"encoding/json"
	"testing"
)

func TestCapability_Execute(t *testing.T) {
	cap := Capability{
		Name:        "greet",
		Description: "Say hello",
		Schema:      json.RawMessage(`{"type":"string"}`),
		Mode:        ReadAction,
		Risk:        0,
		Source:      Environment,
		Execute: func(_ context.Context, input json.RawMessage) (Result, error) {
			return TextResult("hello " + string(input)), nil
		},
	}

	result, err := cap.Execute(context.Background(), json.RawMessage(`"world"`))
	if err != nil {
		t.Fatal(err)
	}
	if string(result.Text()) != `hello "world"` {
		t.Errorf("got %q", string(result.Text()))
	}
}

func TestCapability_BuiltInVsEnvironment(t *testing.T) {
	builtIn := Capability{
		Name:   "andon",
		Source: BuiltIn,
		Mode:   ReadAction,
	}
	env := Capability{
		Name:   "take",
		Source: Environment,
		Mode:   WriteAction,
		Risk:   0.5,
	}

	if builtIn.Source != BuiltIn {
		t.Error("expected BuiltIn source")
	}
	if env.Source != Environment {
		t.Error("expected Environment source")
	}
}

func TestCapability_RiskAndMode(t *testing.T) {
	cap := Capability{
		Name: "write_file",
		Mode: WriteAction,
		Risk: 0.7,
	}

	if cap.Mode != WriteAction {
		t.Error("expected WriteAction")
	}
	if cap.Risk != 0.7 {
		t.Errorf("expected risk 0.7, got %f", cap.Risk)
	}
}

func TestCapabilitySet_Register(t *testing.T) {
	noop := func(_ context.Context, _ json.RawMessage) (Result, error) { return Result{}, nil }
	cs := NewCapabilitySet()
	cs.Register(Capability{Name: "take", Mode: WriteAction, Risk: 0.5, Source: Environment, Execute: noop})
	cs.Register(Capability{Name: "look", Mode: ReadAction, Source: Environment, Execute: noop})
	cs.Register(Capability{Name: "andon", Mode: ReadAction, Source: BuiltIn, Execute: noop})

	if len(cs.All()) != 3 {
		t.Fatalf("expected 3 capabilities, got %d", len(cs.All()))
	}

	voluntary := cs.Voluntary()
	if len(voluntary) != 3 {
		t.Fatalf("expected 3 voluntary, got %d", len(voluntary))
	}

	cap, ok := cs.Get("take")
	if !ok {
		t.Fatal("expected to find 'take'")
	}
	if cap.Risk != 0.5 {
		t.Errorf("take risk = %f, want 0.5", cap.Risk)
	}

	_, ok = cs.Get("nonexistent")
	if ok {
		t.Error("expected not found")
	}
}

func TestCapabilitySet_Names(t *testing.T) {
	cs := NewCapabilitySet()
	cs.Register(Capability{Name: "b", Source: Environment})
	cs.Register(Capability{Name: "a", Source: Environment})
	cs.Register(Capability{Name: "c", Source: BuiltIn})

	names := cs.Names()
	if len(names) != 3 {
		t.Fatalf("expected 3 names, got %d", len(names))
	}
	if names[0] != "b" || names[1] != "a" || names[2] != "c" {
		t.Errorf("expected insertion order [b,a,c], got %v", names)
	}
}
