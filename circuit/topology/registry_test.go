package topology

import (
	"sort"
	"testing"
)

func TestRegistry_Lookup(t *testing.T) {
	r := DefaultRegistry()

	for _, name := range []string{"cascade", "fan-out", "fan-in", "feedback-loop", "bridge"} {
		def, ok := r.Lookup(name)
		if !ok {
			t.Fatalf("Lookup(%q) not found", name)
		}
		if def.Name != name {
			t.Errorf("Lookup(%q).Name = %q", name, def.Name)
		}
	}

	_, ok := r.Lookup("nonexistent")
	if ok {
		t.Error("Lookup(nonexistent) should return false")
	}
}

func TestRegistry_List(t *testing.T) {
	r := DefaultRegistry()
	names := r.List()
	sort.Strings(names)

	want := []string{"bridge", "cascade", "delegate", "fan-in", "fan-out", "feedback-loop"}
	if len(names) != len(want) {
		t.Fatalf("List() = %v, want %v", names, want)
	}
	for i := range want {
		if names[i] != want[i] {
			t.Errorf("List()[%d] = %q, want %q", i, names[i], want[i])
		}
	}
}

func TestRegistry_DuplicateRegistration(t *testing.T) {
	r := DefaultRegistry()
	err := r.Register(&TopologyDef{Name: "cascade"})
	if err == nil {
		t.Fatal("expected error on duplicate registration")
	}
}

func TestCascade_Rules(t *testing.T) {
	r := DefaultRegistry()
	def, _ := r.Lookup("cascade")

	entry := def.RuleFor(PositionEntry)
	if entry == nil {
		t.Fatal("cascade missing entry rule")
	}
	if entry.MinInputs != 0 || entry.MaxInputs != 0 {
		t.Errorf("cascade entry inputs: min=%d max=%d, want 0/0", entry.MinInputs, entry.MaxInputs)
	}
	if entry.MinOutputs != 1 || entry.MaxOutputs != 1 {
		t.Errorf("cascade entry outputs: min=%d max=%d, want 1/1", entry.MinOutputs, entry.MaxOutputs)
	}

	mid := def.RuleFor(PositionIntermediate)
	if mid == nil {
		t.Fatal("cascade missing intermediate rule")
	}
	if mid.MinInputs != 1 || mid.MaxInputs != 1 || mid.MinOutputs != 1 || mid.MaxOutputs != 1 {
		t.Errorf("cascade intermediate: in=%d/%d out=%d/%d, want 1/1/1/1",
			mid.MinInputs, mid.MaxInputs, mid.MinOutputs, mid.MaxOutputs)
	}

	exit := def.RuleFor(PositionExit)
	if exit == nil {
		t.Fatal("cascade missing exit rule")
	}
	if exit.MinInputs != 1 || exit.MaxInputs != 1 || exit.MinOutputs != 0 || exit.MaxOutputs != 0 {
		t.Errorf("cascade exit: in=%d/%d out=%d/%d, want 1/1/0/0",
			exit.MinInputs, exit.MaxInputs, exit.MinOutputs, exit.MaxOutputs)
	}
}
