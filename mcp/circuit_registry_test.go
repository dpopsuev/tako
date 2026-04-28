package mcp

import (
	"context"
	"testing"

	"github.com/dpopsuev/tako/dispatch"
	"github.com/dpopsuev/tangle/signal"
)

func makeTestType(name string) *CircuitType {
	return &CircuitType{
		Name:        name,
		Description: name + " test type",
		StepSchemas: []StepSchema{{Name: name + "-step"}},
		CreateSession: func(_ context.Context, _ StartParams, _ *dispatch.MuxDispatcher, _ signal.Bus) (RunFunc, SessionMeta, error) {
			return func(_ context.Context) (any, error) { return name, nil },
				SessionMeta{Scenario: name},
				nil
		},
	}
}

func TestCircuitTypeRegistry_Register(t *testing.T) {
	reg := NewCircuitTypeRegistry()
	reg.Register(makeTestType("alpha"))
	reg.Register(makeTestType("beta"))

	names := reg.Names()
	if len(names) != 2 {
		t.Fatalf("expected 2 types, got %d", len(names))
	}
	if names[0] != "alpha" || names[1] != "beta" {
		t.Errorf("names should be sorted: got %v", names)
	}
}

func TestCircuitTypeRegistry_RegisterDuplicatePanics(t *testing.T) {
	reg := NewCircuitTypeRegistry()
	reg.Register(makeTestType("alpha"))
	defer func() {
		if r := recover(); r == nil {
			t.Error("expected panic on duplicate")
		}
	}()
	reg.Register(makeTestType("alpha"))
}

func TestCircuitTypeRegistry_Lookup(t *testing.T) {
	reg := NewCircuitTypeRegistry()
	reg.Register(makeTestType("alpha"))

	ct := reg.Lookup("alpha")
	if ct == nil {
		t.Fatal("lookup returned nil")
	}
	if ct.Name != "alpha" {
		t.Errorf("got %s, want alpha", ct.Name)
	}

	if reg.Lookup("missing") != nil {
		t.Error("lookup should return nil for missing type")
	}
}

func TestCircuitTypeRegistry_RouteSession_SingleType(t *testing.T) {
	reg := NewCircuitTypeRegistry()
	reg.Register(makeTestType("alpha"))

	runFn, meta, err := reg.RouteSession(context.Background(), StartParams{}, nil, nil)
	if err != nil {
		t.Fatalf("route single type: %v", err)
	}
	if meta.Scenario != "alpha" {
		t.Errorf("meta.Scenario: got %s, want alpha", meta.Scenario)
	}
	result, _ := runFn(context.Background())
	if result != "alpha" {
		t.Errorf("result: got %v, want alpha", result)
	}
}

func TestCircuitTypeRegistry_RouteSession_ExplicitType(t *testing.T) {
	reg := NewCircuitTypeRegistry()
	reg.Register(makeTestType("alpha"))
	reg.Register(makeTestType("beta"))

	params := StartParams{Extra: map[string]any{"circuit_type": "beta"}}
	_, meta, err := reg.RouteSession(context.Background(), params, nil, nil)
	if err != nil {
		t.Fatalf("route explicit type: %v", err)
	}
	if meta.Scenario != "beta" {
		t.Errorf("meta.Scenario: got %s, want beta", meta.Scenario)
	}
}

func TestCircuitTypeRegistry_RouteSession_MissingTypeMultiple(t *testing.T) {
	reg := NewCircuitTypeRegistry()
	reg.Register(makeTestType("alpha"))
	reg.Register(makeTestType("beta"))

	_, _, err := reg.RouteSession(context.Background(), StartParams{}, nil, nil)
	if err == nil {
		t.Error("expected error when circuit_type missing with multiple types")
	}
}

func TestCircuitTypeRegistry_RouteSession_UnknownType(t *testing.T) {
	reg := NewCircuitTypeRegistry()
	reg.Register(makeTestType("alpha"))

	params := StartParams{Extra: map[string]any{"circuit_type": "bad"}}
	_, _, err := reg.RouteSession(context.Background(), params, nil, nil)
	if err == nil {
		t.Error("expected error for unknown circuit_type")
	}
}

func TestCircuitTypeRegistry_MergedStepSchemas(t *testing.T) {
	reg := NewCircuitTypeRegistry()
	reg.Register(makeTestType("alpha"))
	reg.Register(makeTestType("beta"))

	merged := reg.MergedStepSchemas()
	if len(merged) != 2 {
		t.Errorf("expected 2 merged schemas, got %d", len(merged))
	}
}
