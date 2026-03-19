package framework

import (
	"testing"
)

type memoryEngine struct {
	opened bool
	closed bool
	config map[string]string
}

func (e *memoryEngine) Name() string                 { return "memory" }
func (e *memoryEngine) Open(cfg map[string]string) error { e.opened = true; e.config = cfg; return nil }
func (e *memoryEngine) Close() error                 { e.closed = true; return nil }
func (e *memoryEngine) Migrate(_ *StoreSchema) error { return nil }

func TestStoreRegistry_ResolveByName(t *testing.T) {
	wiring := &StoreWiring{
		Stores: map[string]StoreBinding{
			"rca":       {Engine: "memory"},
			"gnd": {Engine: "memory", Config: map[string]string{"mode": "ephemeral"}},
		},
	}
	reg := NewStoreRegistry(wiring)
	reg.RegisterEngine("memory", func() StoreEngine { return &memoryEngine{} })

	rca, err := reg.Resolve("rca")
	if err != nil {
		t.Fatalf("resolve rca: %v", err)
	}
	if !rca.(*memoryEngine).opened {
		t.Error("rca engine should be opened")
	}

	gnd, err := reg.Resolve("gnd")
	if err != nil {
		t.Fatalf("resolve gnd: %v", err)
	}
	if gnd.(*memoryEngine).config["mode"] != "ephemeral" {
		t.Error("gnd config should have mode=ephemeral")
	}

	// Second resolve returns same instance.
	rca2, err := reg.Resolve("rca")
	if err != nil {
		t.Fatal(err)
	}
	if rca2 != rca {
		t.Error("second resolve should return same instance")
	}
}

func TestStoreRegistry_DefaultEngine(t *testing.T) {
	wiring := &StoreWiring{
		Default: "memory",
	}
	reg := NewStoreRegistry(wiring)
	reg.RegisterEngine("memory", func() StoreEngine { return &memoryEngine{} })

	engine, err := reg.Resolve("anything")
	if err != nil {
		t.Fatalf("resolve with default: %v", err)
	}
	if engine.Name() != "memory" {
		t.Errorf("got %s, want memory", engine.Name())
	}
}

func TestStoreRegistry_NoEngine(t *testing.T) {
	reg := NewStoreRegistry(nil)
	_, err := reg.Resolve("missing")
	if err == nil {
		t.Error("expected error for missing engine")
	}
}

func TestStoreRegistry_UnknownEngine(t *testing.T) {
	wiring := &StoreWiring{
		Stores: map[string]StoreBinding{
			"test": {Engine: "postgres"},
		},
	}
	reg := NewStoreRegistry(wiring)
	_, err := reg.Resolve("test")
	if err == nil {
		t.Error("expected error for unknown engine")
	}
}

func TestStoreRegistry_CloseAll(t *testing.T) {
	wiring := &StoreWiring{
		Stores: map[string]StoreBinding{
			"a": {Engine: "memory"},
			"b": {Engine: "memory"},
		},
	}
	reg := NewStoreRegistry(wiring)
	reg.RegisterEngine("memory", func() StoreEngine { return &memoryEngine{} })

	a, _ := reg.Resolve("a")
	b, _ := reg.Resolve("b")

	if err := reg.CloseAll(); err != nil {
		t.Fatalf("close all: %v", err)
	}
	if !a.(*memoryEngine).closed {
		t.Error("a should be closed")
	}
	if !b.(*memoryEngine).closed {
		t.Error("b should be closed")
	}
}

func TestStoreRegistry_RegisterDuplicatePanics(t *testing.T) {
	reg := NewStoreRegistry(nil)
	reg.RegisterEngine("memory", func() StoreEngine { return &memoryEngine{} })

	defer func() {
		if r := recover(); r == nil {
			t.Error("expected panic on duplicate registration")
		}
	}()
	reg.RegisterEngine("memory", func() StoreEngine { return &memoryEngine{} })
}
