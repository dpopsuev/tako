package organ

import (
	"context"
	"encoding/json"
	"testing"
)

func TestCapability_Execute(t *testing.T) {
	cap := Func{
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
	builtIn := Func{
		Name:   "andon",
		Source: BuiltIn,
		Mode:   ReadAction,
	}
	env := Func{
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
	cap := Func{
		Name: "file.write",
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

func TestFuncSet_Register(t *testing.T) {
	noop := func(_ context.Context, _ json.RawMessage) (Result, error) { return Result{}, nil }
	cs := NewFuncSet()
	cs.Register(Func{Name: "take", Mode: WriteAction, Risk: 0.5, Source: Environment, Execute: noop})
	cs.Register(Func{Name: "look", Mode: ReadAction, Source: Environment, Execute: noop})
	cs.Register(Func{Name: "andon", Mode: ReadAction, Source: BuiltIn, Execute: noop})

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

func TestCapability_WritesMatchResidual(t *testing.T) {
	eat := Func{Name: "eat", Writes: []string{"hungry"}}
	cook := Func{Name: "cook", Writes: []string{"plate", "hand"}}
	look := Func{Name: "look", Reads: []string{"fridge"}}

	if !eat.TouchesDimension("hungry") {
		t.Error("eat should touch hungry")
	}
	if eat.TouchesDimension("plate") {
		t.Error("eat should not touch plate")
	}
	if !cook.TouchesDimension("plate") {
		t.Error("cook should touch plate")
	}
	if look.TouchesDimension("fridge") {
		t.Error("look only reads fridge, TouchesDimension checks Writes")
	}
}

func TestFuncSet_ForDimension(t *testing.T) {
	cs := NewFuncSet()
	cs.Register(Func{Name: "eat", Writes: []string{"hungry"}})
	cs.Register(Func{Name: "cook", Writes: []string{"plate", "hand"}})
	cs.Register(Func{Name: "take", Writes: []string{"hand", "fridge"}})
	cs.Register(Func{Name: "look", Reads: []string{"fridge"}})

	matches := cs.ForDimension("hungry")
	if len(matches) != 1 || matches[0].Name != "eat" {
		t.Errorf("expected [eat] for hungry, got %v", names(matches))
	}

	matches = cs.ForDimension("hand")
	if len(matches) != 2 {
		t.Errorf("expected 2 capabilities for hand, got %d", len(matches))
	}

	matches = cs.ForDimension("nonexistent")
	if len(matches) != 0 {
		t.Errorf("expected 0 for nonexistent, got %d", len(matches))
	}
}

func names(caps []Func) []string {
	out := make([]string, len(caps))
	for i, c := range caps {
		out[i] = c.Name
	}
	return out
}

func TestFuncSet_Names(t *testing.T) {
	cs := NewFuncSet()
	cs.Register(Func{Name: "b", Source: Environment})
	cs.Register(Func{Name: "a", Source: Environment})
	cs.Register(Func{Name: "c", Source: BuiltIn})

	names := cs.Names()
	if len(names) != 3 {
		t.Fatalf("expected 3 names, got %d", len(names))
	}
	if names[0] != "b" || names[1] != "a" || names[2] != "c" {
		t.Errorf("expected insertion order [b,a,c], got %v", names)
	}
}
