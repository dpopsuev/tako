package mcp

import (
	"context"
	"testing"

	"github.com/dpopsuev/bugle/signal"
	"github.com/dpopsuev/origami/dispatch"
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
	reg.Register(makeTestType("rca"))
	reg.Register(makeTestType("gnd"))

	names := reg.Names()
	if len(names) != 2 {
		t.Fatalf("expected 2 types, got %d", len(names))
	}
	if names[0] != "gnd" || names[1] != "rca" {
		t.Errorf("names should be sorted: got %v", names)
	}
}

func TestCircuitTypeRegistry_RegisterDuplicatePanics(t *testing.T) {
	reg := NewCircuitTypeRegistry()
	reg.Register(makeTestType("rca"))
	defer func() {
		if r := recover(); r == nil {
			t.Error("expected panic on duplicate")
		}
	}()
	reg.Register(makeTestType("rca"))
}

func TestCircuitTypeRegistry_Lookup(t *testing.T) {
	reg := NewCircuitTypeRegistry()
	reg.Register(makeTestType("rca"))

	ct := reg.Lookup("rca")
	if ct == nil {
		t.Fatal("lookup returned nil")
	}
	if ct.Name != "rca" {
		t.Errorf("got %s, want rca", ct.Name)
	}

	if reg.Lookup("missing") != nil {
		t.Error("lookup should return nil for missing type")
	}
}

func TestCircuitTypeRegistry_RouteSession_SingleType(t *testing.T) {
	reg := NewCircuitTypeRegistry()
	reg.Register(makeTestType("rca"))

	runFn, meta, err := reg.RouteSession(context.Background(), StartParams{}, nil, nil)
	if err != nil {
		t.Fatalf("route single type: %v", err)
	}
	if meta.Scenario != "rca" {
		t.Errorf("meta.Scenario: got %s, want rca", meta.Scenario)
	}
	result, _ := runFn(context.Background())
	if result != "rca" {
		t.Errorf("result: got %v, want rca", result)
	}
}

func TestCircuitTypeRegistry_RouteSession_ExplicitType(t *testing.T) {
	reg := NewCircuitTypeRegistry()
	reg.Register(makeTestType("rca"))
	reg.Register(makeTestType("gnd"))

	params := StartParams{Extra: map[string]any{"circuit_type": "gnd"}}
	_, meta, err := reg.RouteSession(context.Background(), params, nil, nil)
	if err != nil {
		t.Fatalf("route explicit type: %v", err)
	}
	if meta.Scenario != "gnd" {
		t.Errorf("meta.Scenario: got %s, want gnd", meta.Scenario)
	}
}

func TestCircuitTypeRegistry_RouteSession_MissingTypeMultiple(t *testing.T) {
	reg := NewCircuitTypeRegistry()
	reg.Register(makeTestType("rca"))
	reg.Register(makeTestType("gnd"))

	_, _, err := reg.RouteSession(context.Background(), StartParams{}, nil, nil)
	if err == nil {
		t.Error("expected error when circuit_type missing with multiple types")
	}
}

func TestCircuitTypeRegistry_RouteSession_UnknownType(t *testing.T) {
	reg := NewCircuitTypeRegistry()
	reg.Register(makeTestType("rca"))

	params := StartParams{Extra: map[string]any{"circuit_type": "bad"}}
	_, _, err := reg.RouteSession(context.Background(), params, nil, nil)
	if err == nil {
		t.Error("expected error for unknown circuit_type")
	}
}

func TestCircuitTypeRegistry_MergedStepSchemas(t *testing.T) {
	reg := NewCircuitTypeRegistry()
	reg.Register(makeTestType("rca"))
	reg.Register(makeTestType("gnd"))

	merged := reg.MergedStepSchemas()
	if len(merged) != 2 {
		t.Errorf("expected 2 merged schemas, got %d", len(merged))
	}
}
