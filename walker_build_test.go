package framework

import (
	"os"
	"testing"

	"gopkg.in/yaml.v3"
)

func TestWalkerDefParsesFromYAML(t *testing.T) {
	raw := `
circuit: test
description: walker def test
nodes:
  - name: start_node
    family: stub
  - name: end_node
    family: stub
edges:
  - id: e1
    name: start to end
    from: start_node
    to: done
walkers:
  - name: scout
    approach: rapid
    persona: herald
    preamble: "You are a scout."
    step_affinity:
      recall: 0.9
      triage: 0.7
  - name: analyst
    approach: analytical
    persona: seeker
start: start_node
done: done
`
	var def CircuitDef
	if err := yaml.Unmarshal([]byte(raw), &def); err != nil {
		t.Fatalf("parse YAML: %v", err)
	}

	if len(def.Walkers) != 2 {
		t.Fatalf("expected 2 walkers, got %d", len(def.Walkers))
	}

	scout := def.Walkers[0]
	if scout.Name != "scout" {
		t.Errorf("walker[0].Name = %q, want %q", scout.Name, "scout")
	}
	if scout.Approach != "rapid" {
		t.Errorf("walker[0].Approach = %q, want %q", scout.Approach, "rapid")
	}
	if scout.Persona != "herald" {
		t.Errorf("walker[0].Persona = %q, want %q", scout.Persona, "herald")
	}
	if scout.Preamble != "You are a scout." {
		t.Errorf("walker[0].Preamble = %q, want %q", scout.Preamble, "You are a scout.")
	}
	if scout.StepAffinity["recall"] != 0.9 {
		t.Errorf("walker[0].StepAffinity[recall] = %v, want 0.9", scout.StepAffinity["recall"])
	}
}

func TestWalkerDefEmptyIsBackwardCompatible(t *testing.T) {
	raw := `
circuit: test
nodes:
  - name: n1
    family: stub
edges:
  - id: e1
    name: e
    from: n1
    to: done
start: n1
done: done
`
	var def CircuitDef
	if err := yaml.Unmarshal([]byte(raw), &def); err != nil {
		t.Fatalf("parse YAML: %v", err)
	}
	if len(def.Walkers) != 0 {
		t.Errorf("expected 0 walkers for circuit without walkers section, got %d", len(def.Walkers))
	}
}

func TestBuildWalkersFromDef(t *testing.T) {
	defs := []WalkerDef{
		{Name: "scout", Approach: "rapid", Persona: "herald", Preamble: "Custom preamble."},
		{Name: "analyst", Approach: "analytical", Persona: "seeker"},
	}

	walkers, err := BuildWalkersFromDef(defs)
	if err != nil {
		t.Fatalf("BuildWalkersFromDef: %v", err)
	}
	if len(walkers) != 2 {
		t.Fatalf("expected 2 walkers, got %d", len(walkers))
	}

	scout := walkers[0]
	if scout.Identity().Element != ElementFire {
		t.Errorf("scout.Element = %q, want %q", scout.Identity().Element, ElementFire)
	}
	if scout.Identity().PersonaName != "Herald" {
		t.Errorf("scout.PersonaName = %q, want %q", scout.Identity().PersonaName, "Herald")
	}
	if scout.Identity().PromptPreamble != "Custom preamble." {
		t.Errorf("scout.PromptPreamble = %q, want %q", scout.Identity().PromptPreamble, "Custom preamble.")
	}
	if scout.State().ID != "scout" {
		t.Errorf("scout.State().ID = %q, want %q", scout.State().ID, "scout")
	}

	analyst := walkers[1]
	if analyst.Identity().Element != ElementWater {
		t.Errorf("analyst.Element = %q, want %q", analyst.Identity().Element, ElementWater)
	}
	if analyst.Identity().PersonaName != "Seeker" {
		t.Errorf("analyst.PersonaName = %q, want %q", analyst.Identity().PersonaName, "Seeker")
	}
}

func TestBuildWalkersFromDefUnknownPersona(t *testing.T) {
	defs := []WalkerDef{
		{Name: "bad", Persona: "nonexistent"},
	}
	_, err := BuildWalkersFromDef(defs)
	if err == nil {
		t.Fatal("expected error for unknown persona")
	}
}

func TestBuildWalkersFromDefUnknownApproach(t *testing.T) {
	defs := []WalkerDef{
		{Name: "bad", Approach: "plasma"},
	}
	_, err := BuildWalkersFromDef(defs)
	if err == nil {
		t.Fatal("expected error for unknown approach")
	}
}

func TestBuildWalkersFromDefEmptyName(t *testing.T) {
	defs := []WalkerDef{
		{Name: ""},
	}
	_, err := BuildWalkersFromDef(defs)
	if err == nil {
		t.Fatal("expected error for empty walker name")
	}
}

func TestBuildWalkersFromDefEmptySlice(t *testing.T) {
	walkers, err := BuildWalkersFromDef(nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(walkers) != 0 {
		t.Errorf("expected 0 walkers, got %d", len(walkers))
	}
}

func TestBuildWalkersFromDefPersonaOnlyNoApproach(t *testing.T) {
	defs := []WalkerDef{
		{Name: "minimal", Persona: "sentinel"},
	}
	walkers, err := BuildWalkersFromDef(defs)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	w := walkers[0]
	if w.Identity().Element != ElementEarth {
		t.Errorf("expected Sentinel's default element (earth), got %q", w.Identity().Element)
	}
}

func TestBuildWalkersFromDefStepAffinityOverride(t *testing.T) {
	defs := []WalkerDef{
		{
			Name:    "custom",
			Persona: "herald",
			StepAffinity: map[string]float64{
				"special_step": 1.0,
			},
		},
	}
	walkers, err := BuildWalkersFromDef(defs)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	w := walkers[0]
	if w.Identity().StepAffinity["special_step"] != 1.0 {
		t.Errorf("step affinity override not applied")
	}
	if _, has := w.Identity().StepAffinity["recall"]; has {
		t.Errorf("original persona step affinity should be replaced, not merged")
	}
}

func TestHierarchicalDelegationPatternParsesAndBuilds(t *testing.T) {
	data, err := os.ReadFile("testdata/patterns/hierarchical-delegation.yaml")
	if err != nil {
		t.Fatalf("read pattern YAML: %v", err)
	}

	def, err := LoadCircuit(data)
	if err != nil {
		t.Fatalf("LoadCircuit: %v", err)
	}

	if err := def.Validate(); err != nil {
		t.Fatalf("Validate: %v", err)
	}

	if def.Circuit != "hierarchical-delegation" {
		t.Errorf("circuit name = %q, want %q", def.Circuit, "hierarchical-delegation")
	}
	if len(def.Walkers) != 3 {
		t.Errorf("expected 3 walkers, got %d", len(def.Walkers))
	}
	if len(def.Zones) != 3 {
		t.Errorf("expected 3 zones, got %d", len(def.Zones))
	}

	parallelCount := 0
	for _, e := range def.Edges {
		if e.Parallel {
			parallelCount++
		}
	}
	if parallelCount != 2 {
		t.Errorf("expected 2 parallel edges (fan-out), got %d", parallelCount)
	}

	stubNodes := NodeRegistry{
		"plan":       func(nd NodeDef) Node { return &stubNode{name: nd.Name} },
		"research":   func(nd NodeDef) Node { return &stubNode{name: nd.Name} },
		"synthesize": func(nd NodeDef) Node { return &stubNode{name: nd.Name} },
	}
	_, err = BuildGraph(def, GraphRegistries{Nodes: stubNodes})
	if err != nil {
		t.Fatalf("BuildGraph: %v", err)
	}

	walkers, err := BuildWalkersFromDef(def.Walkers)
	if err != nil {
		t.Fatalf("BuildWalkersFromDef: %v", err)
	}
	if len(walkers) != 3 {
		t.Fatalf("expected 3 walkers, got %d", len(walkers))
	}
}

func TestBuildWalkersFromDefWithRole(t *testing.T) {
	defs := []WalkerDef{
		{Name: "lead", Persona: "herald", Role: "manager"},
		{Name: "grunt", Persona: "sentinel", Role: "worker"},
	}
	walkers, err := BuildWalkersFromDef(defs)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if walkers[0].Identity().Role != RoleManager {
		t.Errorf("walker 0 role = %q, want %q", walkers[0].Identity().Role, RoleManager)
	}
	if walkers[1].Identity().Role != RoleWorker {
		t.Errorf("walker 1 role = %q, want %q", walkers[1].Identity().Role, RoleWorker)
	}
}

func TestBuildWalkersFromDefUnknownRole(t *testing.T) {
	defs := []WalkerDef{
		{Name: "bad", Persona: "herald", Role: "ceo"},
	}
	_, err := BuildWalkersFromDef(defs)
	if err == nil {
		t.Fatal("expected error for unknown role")
	}
}

func TestBuildWalkersFromDefNoRole_BackwardCompat(t *testing.T) {
	defs := []WalkerDef{
		{Name: "old", Persona: "herald"},
	}
	walkers, err := BuildWalkersFromDef(defs)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if walkers[0].Identity().Role != "" {
		t.Errorf("role = %q, want empty (backward compat)", walkers[0].Identity().Role)
	}
	if walkers[0].Identity().HasRole() {
		t.Error("HasRole() = true, want false for no-role walker")
	}
}

func TestValidateElement(t *testing.T) {
	tests := []struct {
		input   string
		want    Element
		wantErr bool
	}{
		{"fire", ElementFire, false},
		{"Fire", ElementFire, false},
		{"EARTH", ElementEarth, false},
		{"plasma", "", true},
		{"", "", true},
	}
	for _, tt := range tests {
		got, err := ValidateElement(tt.input)
		if (err != nil) != tt.wantErr {
			t.Errorf("ValidateElement(%q) error = %v, wantErr %v", tt.input, err, tt.wantErr)
			continue
		}
		if got != tt.want {
			t.Errorf("ValidateElement(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}
