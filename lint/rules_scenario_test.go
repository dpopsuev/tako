package lint

import (
	"strings"
	"testing"

	"github.com/dpopsuev/origami/circuit"
	"github.com/dpopsuev/origami/engine"
)

// --- S35: expected-path-node-names ---

func TestS35_ExpectedPathNodeNames_InvalidNodes(t *testing.T) {
	// Circuit with nodes [recall, triage], scenario references ["F0", "F1"] → finding
	def := &circuit.CircuitDef{
		Circuit: "test",
		Nodes: []circuit.NodeDef{
			{Name: "recall"},
			{Name: "triage"},
		},
		Edges: []circuit.EdgeDef{
			{ID: "e1", From: "recall", To: "triage"},
			{ID: "e2", From: "triage", To: "_done"},
		},
		Start: "recall",
		Done:  "_done",
	}

	ctx := NewLintContextFromDef(def, "circuit.yaml")
	ctx.ProjectFiles = map[string][]ProjectFile{
		"scenario": {{
			File: "scenario.yaml",
			Kind: "scenario",
			Data: map[string]any{
				"kind": "scenario",
				"cases": []any{
					map[string]any{
						"expected_path": []any{"F0", "F1"},
					},
				},
			},
		}},
	}

	rule := &ExpectedPathNodeNames{}
	findings := rule.Check(ctx)
	if len(findings) != 2 {
		t.Fatalf("expected 2 findings (F0 and F1), got %d: %+v", len(findings), findings)
	}
	for _, f := range findings {
		if f.RuleID != "S35/expected-path-node-names" {
			t.Errorf("unexpected rule ID: %s", f.RuleID)
		}
		if f.Severity != SeverityError {
			t.Errorf("expected error severity, got %v", f.Severity)
		}
		if !strings.Contains(f.Message, "F0") && !strings.Contains(f.Message, "F1") {
			t.Errorf("expected message to reference F0 or F1, got %q", f.Message)
		}
	}
}

func TestS35_ExpectedPathNodeNames_ValidNodes(t *testing.T) {
	// Circuit with nodes [recall, triage], scenario references ["recall", "triage"] → no finding
	def := &circuit.CircuitDef{
		Circuit: "test",
		Nodes: []circuit.NodeDef{
			{Name: "recall"},
			{Name: "triage"},
		},
		Edges: []circuit.EdgeDef{
			{ID: "e1", From: "recall", To: "triage"},
			{ID: "e2", From: "triage", To: "_done"},
		},
		Start: "recall",
		Done:  "_done",
	}

	ctx := NewLintContextFromDef(def, "circuit.yaml")
	ctx.ProjectFiles = map[string][]ProjectFile{
		"scenario": {{
			File: "scenario.yaml",
			Kind: "scenario",
			Data: map[string]any{
				"kind": "scenario",
				"cases": []any{
					map[string]any{
						"expected_path": []any{"recall", "triage"},
					},
				},
			},
		}},
	}

	rule := &ExpectedPathNodeNames{}
	findings := rule.Check(ctx)
	if len(findings) != 0 {
		t.Errorf("expected 0 findings for valid node names, got %d: %+v", len(findings), findings)
	}
}

// --- S36: circuit-handler-resolution ---

func TestS36_CircuitHandlerResolution_Unresolvable(t *testing.T) {
	// Node with handler_type=circuit handler=gnd, no circuit file for gnd → finding
	def := &circuit.CircuitDef{
		Circuit: "rca",
		Nodes: []circuit.NodeDef{
			{Name: "gather", HandlerType: "circuit", Handler: "gnd"},
			{Name: "triage", Approach: "rapid"},
		},
		Edges: []circuit.EdgeDef{
			{ID: "e1", From: "gather", To: "triage"},
			{ID: "e2", From: "triage", To: "_done"},
		},
		Start: "gather",
		Done:  "_done",
	}

	ctx := NewLintContextFromDef(def, "rca.yaml")
	ctx.ProjectFiles = map[string][]ProjectFile{
		"circuit": {{
			File: "rca.yaml",
			Kind: "circuit",
			Data: map[string]any{"circuit": "rca"},
		}},
	}

	rule := &CircuitHandlerResolution{}
	findings := rule.Check(ctx)
	if len(findings) != 1 {
		t.Fatalf("expected 1 finding, got %d: %+v", len(findings), findings)
	}
	if findings[0].RuleID != "S36/circuit-handler-resolution" {
		t.Errorf("unexpected rule ID: %s", findings[0].RuleID)
	}
	if !strings.Contains(findings[0].Message, "gnd") {
		t.Errorf("expected message to mention 'gnd', got %q", findings[0].Message)
	}
	if findings[0].Severity != SeverityError {
		t.Errorf("expected error severity, got %v", findings[0].Severity)
	}
}

func TestS36_CircuitHandlerResolution_Resolvable(t *testing.T) {
	def := &circuit.CircuitDef{
		Circuit: "rca",
		Nodes: []circuit.NodeDef{
			{Name: "gather", HandlerType: "circuit", Handler: "gnd"},
		},
		Edges: []circuit.EdgeDef{
			{ID: "e1", From: "gather", To: "_done"},
		},
		Start: "gather",
		Done:  "_done",
	}

	ctx := NewLintContextFromDef(def, "rca.yaml")
	ctx.ProjectFiles = map[string][]ProjectFile{
		"circuit": {
			{File: "rca.yaml", Kind: "circuit", Data: map[string]any{"circuit": "rca"}},
			{File: "gnd.yaml", Kind: "circuit", Data: map[string]any{"circuit": "gnd"}},
		},
	}

	rule := &CircuitHandlerResolution{}
	findings := rule.Check(ctx)
	if len(findings) != 0 {
		t.Errorf("expected 0 findings when circuit file exists, got %d: %+v", len(findings), findings)
	}
}

// --- S37: dead-node-detection ---

func TestS37_DeadNodeDetection_UntestedNode(t *testing.T) {
	// Node gather-code in edges, never in expected_path → finding (warning)
	def := &circuit.CircuitDef{
		Circuit: "test",
		Nodes: []circuit.NodeDef{
			{Name: "recall"},
			{Name: "gather-code"},
			{Name: "triage"},
		},
		Edges: []circuit.EdgeDef{
			{ID: "e1", From: "recall", To: "gather-code"},
			{ID: "e2", From: "gather-code", To: "triage"},
			{ID: "e3", From: "triage", To: "_done"},
		},
		Start: "recall",
		Done:  "_done",
	}

	ctx := NewLintContextFromDef(def, "circuit.yaml")
	ctx.ProjectFiles = map[string][]ProjectFile{
		"scenario": {{
			File: "scenario.yaml",
			Kind: "scenario",
			Data: map[string]any{
				"cases": []any{
					map[string]any{
						"expected_path": []any{"triage"},
					},
				},
			},
		}},
	}

	rule := &DeadNodeDetection{}
	findings := rule.Check(ctx)

	// gather-code is in edges but not in expected_path → dead/untested
	if len(findings) != 1 {
		t.Fatalf("expected 1 finding for gather-code, got %d: %+v", len(findings), findings)
	}
	if findings[0].RuleID != "S37/dead-node-detection" {
		t.Errorf("unexpected rule ID: %s", findings[0].RuleID)
	}
	if findings[0].Severity != SeverityWarning {
		t.Errorf("expected warning severity, got %v", findings[0].Severity)
	}
	if !strings.Contains(findings[0].Message, "gather-code") {
		t.Errorf("expected message to mention 'gather-code', got %q", findings[0].Message)
	}
}

func TestS37_DeadNodeDetection_AllTested(t *testing.T) {
	def := &circuit.CircuitDef{
		Circuit: "test",
		Nodes: []circuit.NodeDef{
			{Name: "recall"},
			{Name: "triage"},
		},
		Edges: []circuit.EdgeDef{
			{ID: "e1", From: "recall", To: "triage"},
			{ID: "e2", From: "triage", To: "_done"},
		},
		Start: "recall",
		Done:  "_done",
	}

	ctx := NewLintContextFromDef(def, "circuit.yaml")
	ctx.ProjectFiles = map[string][]ProjectFile{
		"scenario": {{
			File: "scenario.yaml",
			Kind: "scenario",
			Data: map[string]any{
				"cases": []any{
					map[string]any{
						"expected_path": []any{"recall", "triage"},
					},
				},
			},
		}},
	}

	rule := &DeadNodeDetection{}
	findings := rule.Check(ctx)
	if len(findings) != 0 {
		t.Errorf("expected 0 findings when all nodes tested, got %d: %+v", len(findings), findings)
	}
}

// --- S38: mediator-backend-coverage ---

func TestS38_MediatorBackendCoverage_NoCircuitNoResolver(t *testing.T) {
	// handler_type=circuit handler=gnd, no circuit file, no registries → finding (warning)
	def := &circuit.CircuitDef{
		Circuit: "rca",
		Nodes: []circuit.NodeDef{
			{Name: "gather", HandlerType: "circuit", Handler: "gnd"},
		},
		Edges: []circuit.EdgeDef{
			{ID: "e1", From: "gather", To: "_done"},
		},
		Start: "gather",
		Done:  "_done",
	}

	ctx := NewLintContextFromDef(def, "rca.yaml")
	// No project files, no registries

	rule := &MediatorBackendCoverage{}
	findings := rule.Check(ctx)
	if len(findings) != 1 {
		t.Fatalf("expected 1 finding, got %d: %+v", len(findings), findings)
	}
	if findings[0].RuleID != "S38/mediator-backend-coverage" {
		t.Errorf("unexpected rule ID: %s", findings[0].RuleID)
	}
	if findings[0].Severity != SeverityWarning {
		t.Errorf("expected warning severity, got %v", findings[0].Severity)
	}
	if !strings.Contains(findings[0].Message, "gnd") {
		t.Errorf("expected message to mention 'gnd', got %q", findings[0].Message)
	}
	if !strings.Contains(findings[0].Message, "no mediator endpoint") {
		t.Errorf("expected message to mention no mediator endpoint, got %q", findings[0].Message)
	}
}

func TestS38_MediatorBackendCoverage_WithLocalCircuit(t *testing.T) {
	def := &circuit.CircuitDef{
		Circuit: "rca",
		Nodes: []circuit.NodeDef{
			{Name: "gather", HandlerType: "circuit", Handler: "gnd"},
		},
		Edges: []circuit.EdgeDef{
			{ID: "e1", From: "gather", To: "_done"},
		},
		Start: "gather",
		Done:  "_done",
	}

	ctx := NewLintContextFromDef(def, "rca.yaml")
	ctx.ProjectFiles = map[string][]ProjectFile{
		"circuit": {
			{File: "gnd.yaml", Kind: "circuit", Data: map[string]any{"circuit": "gnd"}},
		},
	}

	rule := &MediatorBackendCoverage{}
	findings := rule.Check(ctx)
	if len(findings) != 0 {
		t.Errorf("expected 0 findings when local circuit exists, got %d: %+v", len(findings), findings)
	}
}

func TestS38_MediatorBackendCoverage_WithMediatorEndpoint(t *testing.T) {
	def := &circuit.CircuitDef{
		Circuit: "rca",
		Nodes: []circuit.NodeDef{
			{Name: "gather", HandlerType: "circuit", Handler: "gnd"},
		},
		Edges: []circuit.EdgeDef{
			{ID: "e1", From: "gather", To: "_done"},
		},
		Start: "gather",
		Done:  "_done",
	}

	ctx := NewLintContextFromDef(def, "rca.yaml")
	ctx.Registries = &engine.GraphRegistries{
		MediatorEndpoint: "localhost:9000",
	}

	rule := &MediatorBackendCoverage{}
	findings := rule.Check(ctx)
	if len(findings) != 1 {
		t.Fatalf("expected 1 finding (still warns even with mediator), got %d: %+v", len(findings), findings)
	}
	if !strings.Contains(findings[0].Message, "mediator endpoint localhost:9000") {
		t.Errorf("expected message to mention mediator endpoint, got %q", findings[0].Message)
	}
}

// --- S39: port-type-consistency ---

func TestS39_PortTypeConsistency_Mismatch(t *testing.T) {
	// Wiring with mismatched port types → finding (warning)
	def := &circuit.CircuitDef{
		Circuit: "orchestrator",
		Ports: []circuit.PortDef{
			{Name: "post-triage", Direction: "out", Type: "string"},
		},
		Wiring: []circuit.WiringDef{
			{From: "orchestrator.out:post-triage", To: "gnd.in:keywords"},
		},
		Nodes: []circuit.NodeDef{
			{Name: "init"},
		},
		Edges: []circuit.EdgeDef{
			{ID: "e1", From: "init", To: "_done"},
		},
		Start: "init",
		Done:  "_done",
	}

	ctx := NewLintContextFromDef(def, "orchestrator.yaml")
	ctx.ProjectFiles = map[string][]ProjectFile{
		"circuit": {{
			File: "gnd.yaml",
			Kind: "circuit",
			Data: map[string]any{
				"circuit": "gnd",
				"ports": []any{
					map[string]any{"name": "keywords", "direction": "in", "type": "[]string"},
				},
			},
		}},
	}

	rule := &PortTypeConsistency{}
	findings := rule.Check(ctx)
	if len(findings) != 1 {
		t.Fatalf("expected 1 finding for type mismatch, got %d: %+v", len(findings), findings)
	}
	if findings[0].RuleID != "S39/port-type-consistency" {
		t.Errorf("unexpected rule ID: %s", findings[0].RuleID)
	}
	if findings[0].Severity != SeverityWarning {
		t.Errorf("expected warning severity, got %v", findings[0].Severity)
	}
	if !strings.Contains(findings[0].Message, "string") || !strings.Contains(findings[0].Message, "[]string") {
		t.Errorf("expected message to mention both types, got %q", findings[0].Message)
	}
}

func TestS39_PortTypeConsistency_Match(t *testing.T) {
	def := &circuit.CircuitDef{
		Circuit: "orchestrator",
		Ports: []circuit.PortDef{
			{Name: "post-triage", Direction: "out", Type: "string"},
		},
		Wiring: []circuit.WiringDef{
			{From: "orchestrator.out:post-triage", To: "gnd.in:keywords"},
		},
		Nodes: []circuit.NodeDef{
			{Name: "init"},
		},
		Edges: []circuit.EdgeDef{
			{ID: "e1", From: "init", To: "_done"},
		},
		Start: "init",
		Done:  "_done",
	}

	ctx := NewLintContextFromDef(def, "orchestrator.yaml")
	ctx.ProjectFiles = map[string][]ProjectFile{
		"circuit": {{
			File: "gnd.yaml",
			Kind: "circuit",
			Data: map[string]any{
				"circuit": "gnd",
				"ports": []any{
					map[string]any{"name": "keywords", "direction": "in", "type": "string"},
				},
			},
		}},
	}

	rule := &PortTypeConsistency{}
	findings := rule.Check(ctx)
	if len(findings) != 0 {
		t.Errorf("expected 0 findings when types match, got %d: %+v", len(findings), findings)
	}
}

// --- parseWiringRef ---

func TestParseWiringRef(t *testing.T) {
	tests := []struct {
		ref       string
		circuit   string
		direction string
		port      string
	}{
		{"rca.out:post-triage", "rca", "out", "post-triage"},
		{"gnd.in:keywords", "gnd", "in", "keywords"},
		{"bad", "", "", ""},
		{"circuit.out", "circuit", "out", ""},
	}
	for _, tt := range tests {
		c, d, p := parseWiringRef(tt.ref)
		if c != tt.circuit || d != tt.direction || p != tt.port {
			t.Errorf("parseWiringRef(%q) = (%q, %q, %q), want (%q, %q, %q)",
				tt.ref, c, d, p, tt.circuit, tt.direction, tt.port)
		}
	}
}
