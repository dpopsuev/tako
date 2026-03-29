package topology

import "testing"

type testGraph struct {
	start string
	done  string
	nodes []NodeInfo
}

func (g *testGraph) StartNode() string     { return g.start }
func (g *testGraph) DoneNode() string      { return g.done }
func (g *testGraph) NodeInfos() []NodeInfo { return g.nodes }
func (g *testGraph) NodeCount() int        { return len(g.nodes) }

func TestValidate_CascadeValid(t *testing.T) {
	r := DefaultRegistry()
	cascade, _ := r.Lookup("cascade")

	shape := &testGraph{
		start: "recall",
		done:  "report",
		nodes: []NodeInfo{
			{Name: "recall", Inputs: 0, Outputs: 1},
			{Name: "triage", Inputs: 1, Outputs: 1},
			{Name: "resolve", Inputs: 1, Outputs: 1},
			{Name: "investigate", Inputs: 1, Outputs: 1},
			{Name: "correlate", Inputs: 1, Outputs: 1},
			{Name: "review", Inputs: 1, Outputs: 1},
			{Name: "report", Inputs: 1, Outputs: 0},
		},
	}

	result := Validate(shape, cascade)
	if !result.OK() {
		t.Errorf("valid cascade should pass: %s", result.Error())
	}
}

func TestValidate_CascadeInvalidExtraInput(t *testing.T) {
	r := DefaultRegistry()
	cascade, _ := r.Lookup("cascade")

	shape := &testGraph{
		start: "A",
		done:  "C",
		nodes: []NodeInfo{
			{Name: "A", Inputs: 0, Outputs: 1},
			{Name: "B", Inputs: 2, Outputs: 1}, // violation: intermediate has 2 inputs
			{Name: "C", Inputs: 1, Outputs: 0},
		},
	}

	result := Validate(shape, cascade)
	if result.OK() {
		t.Fatal("cascade with 2-input intermediate should fail")
	}
	if len(result.Violations) != 1 {
		t.Fatalf("expected 1 violation, got %d: %s", len(result.Violations), result.Error())
	}
	v := result.Violations[0]
	if v.NodeName != "B" {
		t.Errorf("violation node = %q, want B", v.NodeName)
	}
	if v.Position != PositionIntermediate {
		t.Errorf("violation position = %q, want intermediate", v.Position)
	}
	if v.Field != "inputs" {
		t.Errorf("violation field = %q, want inputs", v.Field)
	}
}

func TestValidate_FanOutValid(t *testing.T) {
	r := DefaultRegistry()
	fanout, _ := r.Lookup("fan-out")

	shape := &testGraph{
		start: "source",
		done:  "",
		nodes: []NodeInfo{
			{Name: "source", Inputs: 0, Outputs: 3},
			{Name: "worker1", Inputs: 1, Outputs: 0},
			{Name: "worker2", Inputs: 1, Outputs: 0},
			{Name: "worker3", Inputs: 1, Outputs: 0},
		},
	}

	result := Validate(shape, fanout)
	if !result.OK() {
		t.Errorf("valid fan-out should pass: %s", result.Error())
	}
}

func TestValidate_FanOutTooFewOutputs(t *testing.T) {
	r := DefaultRegistry()
	fanout, _ := r.Lookup("fan-out")

	shape := &testGraph{
		start: "source",
		done:  "",
		nodes: []NodeInfo{
			{Name: "source", Inputs: 0, Outputs: 1}, // fan-out requires >= 2
			{Name: "worker1", Inputs: 1, Outputs: 0},
		},
	}

	result := Validate(shape, fanout)
	if result.OK() {
		t.Fatal("fan-out source with 1 output should fail")
	}
}

func TestValidate_TooFewNodes(t *testing.T) {
	r := DefaultRegistry()
	cascade, _ := r.Lookup("cascade")

	shape := &testGraph{
		start: "only",
		done:  "only",
		nodes: []NodeInfo{
			{Name: "only", Inputs: 0, Outputs: 0},
		},
	}

	result := Validate(shape, cascade)
	if result.OK() {
		t.Fatal("cascade with 1 node should fail (min 2)")
	}
}

func TestValidate_NoTopologySkips(t *testing.T) {
	shape := &testGraph{
		start: "A",
		done:  "B",
		nodes: []NodeInfo{
			{Name: "A", Inputs: 0, Outputs: 5},
			{Name: "B", Inputs: 5, Outputs: 0},
		},
	}

	result := Validate(shape, &TopologyDef{
		Name:     "any",
		MinNodes: 0,
		MaxNodes: -1,
	})
	if !result.OK() {
		t.Errorf("unconstrained topology should pass: %s", result.Error())
	}
}

func TestViolation_String(t *testing.T) {
	v := Violation{
		NodeName: "triage",
		Position: PositionIntermediate,
		Field:    "inputs",
		Expected: "exactly 1",
		Actual:   2,
	}
	got := v.String()
	want := `node "triage" at position "intermediate": inputs expected exactly 1, got 2`
	if got != want {
		t.Errorf("String() = %q, want %q", got, want)
	}
}

func TestValidationResult_Error(t *testing.T) {
	r := &ValidationResult{
		Violations: []Violation{
			{NodeName: "A", Position: PositionEntry, Field: "outputs", Expected: "exactly 1", Actual: 3},
		},
	}
	if r.OK() {
		t.Fatal("result with violations should not be OK")
	}
	errStr := r.Error()
	if errStr == "" {
		t.Fatal("Error() should not be empty")
	}
}
